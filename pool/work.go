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
    template, auxBlocks, err := p.fetchAllBlockTemplatesFromRPC()
    if err != nil {
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
        for _, auxBlock := range auxBlocks {
            mergedPOW := auxBlock.Hash
            auxillary = auxillary + hexStringToByteString(mergedPOW) // Use from encoding.go
            p.templates.AuxBlocks = []bitcoin.AuxBlock{*auxBlock}
            break
        }
    }

    primaryName := p.config.GetPrimary()
    rewardPubScriptKey := p.GetPrimaryNode().RewardPubScriptKey
    extranonceByteReservationLength := 8

    block, work, err := bitcoin.GenerateWork(
        template,
        auxBlocks,
        primaryName,
        auxillary,
        rewardPubScriptKey,
        extranonceByteReservationLength,
    )
    if err != nil {
        log.Print(err)
        return err
    }

    // Convert bitcoin.Work to []string if needed
    p.workCache = convertWorkToStringSlice(work) // Define this helper
    p.templates.BitcoinBlock = *block
    return nil
}

// Generate work from cache
func (p *PoolServer) generateWorkFromCache(force bool) ([]string, error) {
    if force || len(p.workCache) == 0 {
        primaryName := p.config.BlockChainOrder[0]

        template, auxBlocks, err := p.fetchAllBlockTemplatesFromRPC()
        if err != nil {
            return nil, err
        }

        _, work, err := bitcoin.GenerateWork(
            template,
            auxBlocks,
            primaryName,
            "", // arbitrary
            "", // rewardPubScriptKey
            0,  // extranonceByteReservationLength
        )
        if err != nil {
            return nil, err
        }

        p.workCache = convertWorkToStringSlice(work) // Convert to []string
        return p.workCache, nil
    }
    return p.workCache, nil
}

// Helper to convert bitcoin.Work to []string (adjust based on actual type)
func convertWorkToStringSlice(work bitcoin.Work) []string {
    // Assuming bitcoin.Work is []interface{} or similar
    result := make([]string, len(work))
    for i, v := range work {
        result[i] = fmt.Sprintf("%v", v) // Convert each element to string
    }
    return result
}