package pool

import (
    "encoding/json"
    "errors"
    "fmt"
    "log"
    "math/big"
    "strings"
    "time"

    "designs.capital/dogepool/bitcoin"
    "github.com/google/uuid"
)

type stratumResponse struct {
    Id      json.RawMessage       `json:"id"`
    Version string                `json:"jsonrpc,omitempty"`
    Result  interface{}           `json:"result"`
    Error   *stratumErrorResponse `json:"error,omitempty"`
}

type stratumErrorResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

func (pool *PoolServer) respondToStratumClient(client *stratumClient, requestPayload []byte) error {
    var request stratumRequest
    err := json.Unmarshal(requestPayload, &request)
    if err != nil {
        markMalformedRequest(client, requestPayload)
        log.Println("Malformed stratum request from: " + client.ip)
        return err
    }

    timeoutTime := time.Now().Add(pool.connectionTimeout)
    client.connection.SetDeadline(timeoutTime)

    response, err := handleStratumRequest(&request, client, pool)
    if err != nil {
        return err
    }

    return sendPacket(response, client)
}

func handleStratumRequest(request *stratumRequest, client *stratumClient, pool *PoolServer) (any, error) {
    switch request.Method {
    case "mining.subscribe":
        return miningSubscribe(request, client)
    case "mining.authorize":
        return miningAuthorize(request, client, pool)
    case "mining.extranonce.subscribe":
        return miningExtranonceSubscribe(request, client)
    case "mining.submit":
        return miningSubmit(request, client, pool)
    case "mining.multi_version":
        return nil, nil // ignored
    default:
        return stratumResponse{}, errors.New("unknown stratum request method: " + request.Method)
    }
}

func miningSubscribe(request *stratumRequest, client *stratumClient) (stratumResponse, error) {
    var response stratumResponse

    if isBanned(client.ip) {
        return response, errors.New("client blocked: " + client.ip)
    }

    requestParamsJson, err := request.Params.MarshalJSON()
    if err != nil {
        return response, err
    }

    var requestParams []string
    json.Unmarshal(requestParamsJson, &requestParams)
    if len(requestParams) > 0 {
        clientType := requestParams[0]
        log.Println("New subscription from client type: " + clientType)
        client.userAgent = clientType
    }

    client.sessionID = uuid.NewString()

    var subscriptions []interface{}
    difficulty := interface{}([]string{"mining.set_difficulty", client.sessionID})
    notify := interface{}([]string{"mining.notify", client.sessionID})
    extranonce1 := interface{}(client.extranonce1)
    extranonce2Length := interface{}(4)

    subscriptions = append(subscriptions, difficulty)
    subscriptions = append(subscriptions, notify)

    var responseResult []interface{}
    responseResult = append(responseResult, subscriptions)
    responseResult = append(responseResult, extranonce1)
    responseResult = append(responseResult, extranonce2Length)

    response.Id = request.Id
    response.Result = responseResult

    return response, nil
}

func miningAuthorize(request *stratumRequest, client *stratumClient, pool *PoolServer) (any, error) {
    var reply stratumRequest

    if isBanned(client.ip) {
        return reply, errors.New("banned client attempted to access: " + client.ip)
    }

    var params []string
    err := json.Unmarshal(request.Params, ¶ms)
    if err != nil {
        return reply, err
    }
    if len(params) < 1 {
        return reply, errors.New("invalid parameters")
    }

    authResponse := stratumResponse{
        Result: interface{}(false),
        Id:     request.Id,
    }

    loginString := params[0]
    loginParts := strings.Split(loginString, ".")
    if len(loginParts) != 2 {
        return authResponse, errors.New("invalid login format: missing rigID")
    }

    minerAddressesString := loginParts[0]
    minerAddresses := strings.Split(minerAddressesString, "-")
    if len(minerAddresses) != len(pool.config.BlockChainOrder) {
        return authResponse, errors.New("not enough miner addresses to login: expected " + fmt.Sprint(len(pool.config.BlockChainOrder)))
    }

    rigID := loginParts[1]

    blockchainIndex := 0
    for _, blockChainName := range pool.config.BlockChainOrder {
        blockChain := bitcoin.GetChain(blockChainName)
        inputBlockChainAddress := minerAddresses[blockchainIndex]

        network := pool.activeNodes[blockChainName].Network
        if (network == "test" && !blockChain.ValidTestnetAddress(inputBlockChainAddress)) ||
            (network == "main" && !blockChain.ValidMainnetAddress(inputBlockChainAddress)) {
            m := "invalid %v %vnet miner address from %v: %v"
            m = fmt.Sprintf(m, blockChainName, network, client.ip, inputBlockChainAddress)
            return authResponse, errors.New(m)
        }

        blockchainIndex++
    }

    log.Printf("Authorized rig: %v mining to addresses: %v", rigID, minerAddresses)

    client.login = loginString
    addSession(client)

    authResponse.Result = interface{}(true)

    err = sendPacket(authResponse, client)
    if err != nil {
        return reply, err
    }

    err = sendPacket(miningSetDifficulty(pool.config.PoolDifficulty), client)
    if err != nil {
        return reply, err
    }

    work, err := pool.generateWorkFromCache(false)
    if err != nil {
        return reply, err
    }

    reply = miningNotify(work)
    return reply, nil
}

func miningExtranonceSubscribe(request *stratumRequest, client *stratumClient) (stratumResponse, error) {
    var response stratumResponse
    response.Id = request.Id
    response.Result = interface{}(true)
    log.Println("Client subscribed to extranonce updates: " + client.ip)
    return response, nil
}

func miningSubmit(request *stratumRequest, client *stratumClient, pool *PoolServer) (stratumResponse, error) {
    response := stratumResponse{
        Result: interface{}(false),
        Id:     request.Id,
    }

    var work []string // bitcoin.Work is []string
    err := json.Unmarshal(request.Params, &work)
    if err != nil {
        return response, fmt.Errorf("failed to parse submit params: %v", err)
    }

    err = pool.recieveWorkFromClient(work, client)
    if err != nil {
        log.Printf("Work submission error from %v: %v", client.ip, err)
        if strings.Contains(err.Error(), "invalid share") {
            return response, nil // Invalid share but not a protocol error
        }
        return response, err
    }

    response.Result = interface{}(true)
    return response, nil
}

func (pool *PoolServer) recieveWorkFromClient(work []string, client *stratumClient) error {
    // Fetch current work template and aux blocks (replace cached fields)
    templateWork, err := pool.generateWorkFromCache(false)
    if err != nil {
        return fmt.Errorf("failed to fetch work template: %v", err)
    }

    // Extract submitted values (assuming Stratum submit format: [worker, jobID, extranonce2, ntime, nonce])
    if len(work) < 5 {
        return errors.New("invalid work submission: too few parameters")
    }
    // worker := work[0] // Not used here
    jobID := work[1]
    extranonce2 := work[2]
    ntime := work[3]
    nonce := work[4]

    // Regenerate block with submitted data
    block, _, err := bitcoin.GenerateWork(
        pool.activeNodes[pool.config.BlockChainOrder[0]].Template, // Primary chain template
        pool.activeNodes[pool.config.BlockChainOrder[0]].AuxBlocks, // Adjust if aux blocks are stored differently
        pool.config.BlockChainOrder[0],
        extranonce2, // Use extranonce2 as arbitrary
        "",          // Placeholder for PoolPayoutPubScriptKey (adjust as needed)
        0,
    )
    if err != nil {
        return fmt.Errorf("failed to regenerate work: %v", err)
    }

    header, err := block.MakeHeader(extranonce2, nonce, ntime)
    if err != nil {
        return fmt.Errorf("failed to make header: %v", err)
    }

    // Validate share against pool difficulty
    hashInt, err := block.Sum()
    if err != nil {
        return fmt.Errorf("failed to hash header: %v", err)
    }

    targetInt, ok := big.NewInt(0).SetString(pool.config.PoolDifficulty, 10)
    if !ok {
        targetInt = big.NewInt(1) // Default fallback
    }
    if hashInt.Cmp(targetInt) > 0 {
        return errors.New("invalid share: below pool difficulty")
    }

    // Submit to primary chain
    primaryChain := pool.activeNodes[pool.config.BlockChainOrder[0]]
    _, err = primaryChain.RPC.SubmitBlock([]string{header}) // Adjust based on RPC method signature
    if err != nil {
        log.Printf("Primary chain submission failed: %v", err)
    }

    // Submit auxpow to auxiliary chains
    for _, chainName := range pool.config.BlockChainOrder[1:] {
        if len(templateWork) > 8 { // Check for aux data
            for _, auxEntry := range templateWork[8:] {
                parts := strings.SplitN(auxEntry, ":", 2)
                if len(parts) == 2 && parts[0] == chainName {
                    auxChain := pool.activeNodes[chainName]
                    _, err = auxChain.RPC.SubmitAuxBlock(parts[1], header)
                    if err != nil {
                        log.Printf("Aux chain %v submission failed: %v", chainName, err)
                    }
                }
            }
        }
    }

    return nil
}