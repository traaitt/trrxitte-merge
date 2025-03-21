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
    if err != nil {
        log.Printf("Primary Sum failed: %v", err)
        return shareInvalid, 0
    }
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
        status = primaryCandidate
    }

    if aux1 != nil && aux1.Hash != "" {
        auxTarget := bitcoin.Target(reverseHexBytes(aux1.Target))
        auxTargetBig, ok := auxTarget.ToBig()
        if !ok || auxTargetBig == nil {
            log.Printf("Invalid aux target: %s", aux1.Target)
            return status, shareDifficulty // Return current status, don’t crash
        }

        if primarySum.Cmp(auxTargetBig) <= 0 { // Line 41: Now safe
            if status == primaryCandidate {
                status = dualCandidate
            } else {
                status = aux1Candidate
            }
        }
    }

    if status > shareInvalid {
        return status, shareDifficulty
    }

    poolTargetBig, ok := poolTarget.ToBig()
    if !ok || poolTargetBig == nil {
        log.Printf("Invalid pool target")
        return shareInvalid, shareDifficulty
    }
    if primarySum.Cmp(poolTargetBig) <= 0 {
        return shareValid, shareDifficulty
    }

    return shareInvalid, shareDifficulty
}

// Assuming logOnError exists elsewhere; if not, define it
func logOnError(err error) {
    if err != nil {
        log.Println(err)
    }
}