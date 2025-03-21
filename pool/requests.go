package pool

import (
    "encoding/json"
    "log" // Ensure log is imported

    "designs.capital/dogepool/bitcoin"
)

type stratumRequest struct {
    Id     json.RawMessage `json:"id"`
    Method string          `json:"method"`
    Params json.RawMessage `json:"params"`
}

func miningNotify(work bitcoin.Work) stratumRequest {
    var request stratumRequest

    params, err := json.Marshal(work)
    logOnError(err) // Uses server.go definition

    request.Method = "mining.notify"
    request.Params = params

    return request
}

func miningSetDifficulty(difficulty float64) stratumRequest {
    var request stratumRequest

    request.Method = "mining.set_difficulty"

    diff := []float64{difficulty}

    var err error
    request.Params, err = json.Marshal(diff)
    logOnError(err) // Uses server.go definition

    return request
}

func miningSetExtranonce(extranonce string) stratumRequest {
    var request stratumRequest

    request.Method = "mining.set_extranonce"

    params := []string{extranonce}
    var err error
    request.Params, err = json.Marshal(params)
    logOnError(err) // Uses server.go definition

    return request
}