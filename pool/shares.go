package pool

import (
    "log"
    "designs.capital/dogepool/bitcoin"
)

const (
    shareInvalid = iota
    shareValid
    primaryCandidate
    aux1Candidate
    dualCandidate
)

var statusMap = map[int]string{
    2: "Primary",
    3: "Aux1",
    4: "Dual",
}

func validateAndWeighShare(primary *bitcoin.BitcoinBlock, aux1 *bitcoin.AuxBlock, poolDifficulty float64) (int, float64) {
    if primary == nil {
        log.Printf("Nil primary block")
        return shareInvalid, 0
    }

    primarySum, err := primary.Sum()
    logOnError(err)
    if primarySum == nil {
        log.Printf("Nil primarySum")
        return shareInvalid, 0
    }

    primaryTarget := bitcoin.Target(primary.Template.Target)
    primaryTargetBig, ok := primaryTarget.ToBig()
    if !ok || primaryTargetBig == nil {
        log.Printf("Invalid primary target: %s", primary.Template.Target)
        return shareInvalid, 0
    }

    poolTarget, _ := bitcoin.TargetFromDifficulty(poolDifficulty / primary.ShareMultiplier())
    shareDifficulty, _ := poolTarget.ToDifficulty()

    status := shareInvalid

    if primarySum.Cmp(primaryTargetBig) <= 0 {
        log.Printf("Primary share is a block candidate")
        status = primaryCandidate
    }

    if aux1 != nil && aux1.Hash != "" {
        log.Printf("Processing aux chain: Hash=%s, Target=%s", aux1.Hash, aux1.Target)
        auxTarget := bitcoin.Target(reverseHexBytes(aux1.Target))
        auxTargetBig, ok := auxTarget.ToBig()
        if !ok || auxTargetBig == nil {
            log.Printf("Invalid aux target: %s (reversed: %s)", aux1.Target, reverseHexBytes(aux1.Target))
            return status, shareDifficulty
        }

        if primarySum.Cmp(auxTargetBig) <= 0 {
            log.Printf("Aux share is a block candidate")
            if status == primaryCandidate {
                status = dualCandidate
            } else {
                status = aux1Candidate
            }
        }
    } else {
        log.Printf("No aux chain data: aux1=%v", aux1)
    }

    if status > shareInvalid {
        log.Printf("Valid share or candidate: status=%d", status)
        return status, shareDifficulty
    }

    poolTargetBig, ok := poolTarget.ToBig()
    if !ok || poolTargetBig == nil {
        log.Printf("Invalid pool target")
        return shareInvalid, shareDifficulty
    }
    if primarySum.Cmp(poolTargetBig) <= 0 {
        log.Printf("Share meets pool difficulty")
        return shareValid, shareDifficulty
    }

    log.Printf("Share invalid: primarySum=%s, poolTarget=%s", primarySum.Text(16), poolTargetBig.Text(16))
    return shareInvalid, shareDifficulty
}