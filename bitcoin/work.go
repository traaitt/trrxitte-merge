package pool

import (
    "designs.capital/dogepool/bitcoin"
)

func (pool *PoolServer) generateWorkFromCache(clean bool) (bitcoin.Work, error) {
    primaryName := pool.config.BlockChainOrder[0]
    auxNames := pool.config.BlockChainOrder[1:]

    // Fetch template and aux blocks
    primaryChain := pool.activeNodes[primaryName]
    template := primaryChain.GetTemplate() // Assume this exists
    auxBlocks := make(map[string]*bitcoin.AuxBlock)
    for _, auxName := range auxNames {
        auxChain := pool.activeNodes[auxName]
        auxBlock, err := auxChain.GetAuxBlock() // Assume this exists
        if err != nil {
            return nil, err
        }
        auxBlocks[auxName] = auxBlock
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

    return work, nil
}