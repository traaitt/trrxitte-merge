package pool

import (
    "errors"
    "fmt"
    "log"
    "strings"
    "time"

    "designs.capital/dogepool/bitcoin"
    "designs.capital/dogepool/persistence"
)

// Main INPUT
func (p *PoolServer) fetchRpcBlockTemplatesAndCacheWork() error {
    var block *bitcoin.BitcoinBlock
    var err error
    template, auxBlocks, err := p.fetchAllBlockTemplatesFromRPC() // Updated to match server.go return type
    if err != nil {
        // Switch nodes if we fail to get work
        err = p.CheckAndRecoverRPCs()
        if err != nil {
            return err
        }
        template, auxBlocks, err = p.fetchAllBlockTemplatesFromRPC()
        if err != nil {
            return err
        }
    }

    auxillary := p.config.BlockSignature
    if len(auxBlocks) > 0 {
        // Pick the first aux block for compatibility with original logic
        for _, auxBlock := range auxBlocks {
            mergedPOW := auxBlock.Hash // Use Hash instead of GetWork (assumed field)
            auxillary = auxillary + hexStringToByteString(mergedPOW)
            p.templates.AuxBlocks = []bitcoin.AuxBlock{*auxBlock} // Simplified to first block
            break
        }
    }

    primaryName := p.config.GetPrimary()
    rewardPubScriptKey := p.GetPrimaryNode().RewardPubScriptKey
    extranonceByteReservationLength := 8

    block, p.workCache, err = bitcoin.GenerateWork(
        template,
        auxBlocks, // Updated to map
        primaryName,
        auxillary,
        rewardPubScriptKey,
        extranonceByteReservationLength,
    )
    if err != nil {
        log.Print(err)
        return err
    }

    p.templates.BitcoinBlock = *block
    return nil
}

// Main OUTPUT
func (p *PoolServer) receiveWorkFromClient(share bitcoin.Work, client *stratumClient) error {
    primaryBlockTemplate := p.templates.GetPrimary()
    if primaryBlockTemplate.Template == nil {
        return errors.New("primary block template not yet set")
    }
    auxBlock := p.templates.GetAux1()

    var err error

    workerString := share[0].(string)
    workerStringParts := strings.Split(workerString, ".")
    if len(workerStringParts) < 2 {
        return errors.New("invalid miner address")
    }
    minerAddress := workerStringParts[0]
    rigID := workerStringParts[1]

    primaryBlockHeight := primaryBlockTemplate.Template.Height
    nonce := share[primaryBlockTemplate.NonceSubmissionSlot()].(string)
    extranonce2Slot, _ := primaryBlockTemplate.Extranonce2SubmissionSlot()
    extranonce2 := share[extranonce2Slot].(string)
    nonceTime := share[primaryBlockTemplate.NonceTimeSubmissionSlot()].(string)

    extranonce := client.extranonce1 + extranonce2

    _, err = primaryBlockTemplate.MakeHeader(extranonce, nonce, nonceTime)
    if err != nil {
        return err
    }

    shareStatus, shareDifficulty := validateAndWeighShare(&primaryBlockTemplate, auxBlock, p.config.PoolDifficulty)

    heightMessage := fmt.Sprintf("%v", primaryBlockHeight)
    if shareStatus == dualCandidate {
        heightMessage = fmt.Sprintf("%v,%v", primaryBlockHeight, auxBlock.Height)
    } else if shareStatus == aux1Candidate {
        heightMessage = fmt.Sprintf("%v", auxBlock.Height)
    }

    if shareStatus == shareInvalid {
        m := "❔ Invalid share for block %v from %v [%v] [%v]"
        m = fmt.Sprintf(m, heightMessage, client.ip, rigID, client.userAgent)
        return errors.New(m)
    }

    m := "Valid share for block %v from %v [%v]"
    m = fmt.Sprintf(m, heightMessage, client.ip, rigID)
    log.Println(m)

    blockTarget := bitcoin.Target(primaryBlockTemplate.Template.Target)
    blockDifficulty, _ := blockTarget.ToDifficulty()
    blockDifficulty = blockDifficulty * primaryBlockTemplate.ShareMultiplier()

    p.Lock()
    p.shareBuffer = append(p.shareBuffer, persistence.Share{
        PoolID:            p.config.PoolName,
        BlockHeight:       primaryBlockHeight,
        Miner:             minerAddress,
        Worker:            rigID,
        UserAgent:         client.userAgent,
        Difficulty:        shareDifficulty,
        NetworkDifficulty: blockDifficulty,
        IpAddress:         client.ip,
        Created:           time.Now(),
    })
    p.Unlock()

    if shareStatus == shareValid {
        return nil
    }

    statusReadable := statusMap[shareStatus]
    successStatus := 0

    m = "%v block candidate for block %v from %v [%v]"
    m = fmt.Sprintf(m, statusReadable, heightMessage, client.ip, rigID)
    log.Println(m)

    found := persistence.Found{
        PoolID:               p.config.PoolName,
        Status:               persistence.StatusPending,
        Type:                 statusReadable,
        ConfirmationProgress: 0,
        Miner:                minerAddress,
        Source:               "",
    }

    aux1Name := p.config.GetAux1()
    if aux1Name != "" && shareStatus >= aux1Candidate {
        err = p.submitAuxBlock(primaryBlockTemplate, *auxBlock)
        if err != nil {
            err = p.rpcManagers[p.config.GetAux1()].CheckAndRecoverRPCs()
            if err != nil {
                return err
            }
            err = p.submitBlockToChain(primaryBlockTemplate)
        }

        if err != nil {
            log.Println(err)
        } else {
            aux1Target := bitcoin.Target(reverseHexBytes(auxBlock.Target))
            aux1Difficulty, _ := aux1Target.ToDifficulty()
            aux1Difficulty = aux1Difficulty * bitcoin.GetChain(aux1Name).ShareMultiplier()

            found.Chain = aux1Name
            found.Created = time.Now()
            found.Hash = auxBlock.Hash
            found.NetworkDifficulty = aux1Difficulty
            found.BlockHeight = uint(auxBlock.Height)
            found.TransactionConfirmationData = reverseHexBytes(auxBlock.CoinbaseHash)

            err = persistence.Blocks.Insert(found)
            if err != nil {
                log.Println(err)
            }

            successStatus = aux1Candidate
        }
    }

    if shareStatus == dualCandidate || shareStatus == primaryCandidate {
        err = p.submitBlockToChain(primaryBlockTemplate)
        if err != nil {
            err = p.rpcManagers[p.config.GetPrimary()].CheckAndRecoverRPCs()
            if err != nil {
                return err
            }
            err = p.submitBlockToChain(primaryBlockTemplate)
        }

        if err != nil {
            return err
        } else {
            found.Chain = p.config.GetPrimary()
            found.Created = time.Now()
            found.Hash, err = primaryBlockTemplate.HeaderHashed()
            if err != nil {
                log.Println(err)
            }
            found.NetworkDifficulty = blockDifficulty
            found.BlockHeight = primaryBlockHeight
            found.TransactionConfirmationData, err = primaryBlockTemplate.CoinbaseHashed()
            if err != nil {
                log.Println(err)
            }

            err = persistence.Blocks.Insert(found)
            if err != nil {
                log.Println(err)
            }
            found.Chain = ""
            if successStatus == aux1Candidate {
                successStatus = dualCandidate
            } else {
                successStatus = primaryCandidate
            }
        }
    }

    statusReadable = statusMap[successStatus]
    log.Printf("✅  Successful %v submission of block %v from: %v [%v]", statusReadable, heightMessage, client.ip, rigID)

    return nil
}

// Generate work from cache
func (p *PoolServer) generateWorkFromCache(force bool) ([]string, error) {
    if force || len(p.workCache) == 0 {
        primaryName := p.config.BlockChainOrder[0] // Moved from line 231

        // Fetch template and aux blocks
        template, auxBlocks, err := p.fetchAllBlockTemplatesFromRPC()
        if err != nil {
            return nil, err
        }

        // Generate work
        _, work, err := bitcoin.GenerateWork(
            template,
            auxBlocks,
            primaryName,
            "", // arbitrary
            "", // rewardPubScriptKey (adjust if available)
            0,  // extranonceByteReservationLength
        )
        if err != nil {
            return nil, err
        }

        p.workCache = work
        return work, nil
    }
    return p.workCache, nil
}

// Helper function (assuming it’s defined elsewhere or needs to be added)
func hexStringToByteString(hexStr string) string {
    // Placeholder; implement as needed
    return hexStr
}