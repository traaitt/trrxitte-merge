package pool

import (
    "encoding/json"

    "designs.capital/dogepool/bitcoin"
)

type stratumRequest struct {
    Id     json.RawMessage `json:"id"`
    Method string          `json:"method"`
    Params json.RawMessage `json:"params"`
}

func miningNotify(work bitcoin.Work) stratumRequest {
    var request stratumRequest

    // Marshal the work directly, which includes jobID, prevHash, coinb1, coinb2, merkleBranch, version, nBits, nTime, and aux data
    params, err := json.Marshal(work)
    logOnError(err)

    request.Method = "mining.notify"
    request.Params = params

    return request
}

func miningSetDifficulty(difficulty float64) stratumRequest {
    var request stratumRequest

    request.Method = "mining.set_difficulty"

    // Stratum expects difficulty as an array with one float value
    diff := []float64{difficulty}

    var err error
    request.Params, err = json.Marshal(diff)
    logOnError(err)

    return request
}

func miningSetExtranonce(extranonce string) stratumRequest {
    var request stratumRequest

    // Stratum method for setting extranonce (not standard but used by some pools)
    // Typically sent as ["extranonce1", extranonce2_size], but here we just send the extranonce
    request.Method = "mining.set_extranonce"

    // Assuming extranonce is a string (hex-encoded)
    params := []string{extranonce}
    var err error
    request.Params, err = json.Marshal(params)
    logOnError(err)

    return request
}

// Helper function to log errors (assumed to exist elsewhere; included for clarity)
func logOnError(err error) {
    if err != nil {
        log.Println("JSON marshal error:", err)
    }
}