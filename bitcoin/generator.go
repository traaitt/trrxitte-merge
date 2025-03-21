package bitcoin

import (
    "encoding/hex"
    "errors"
    "fmt"
    "math/big"
)

type BlockGenerator interface {
    MakeHeader(extranonce, nonce string) (string, error)
    Header() string
    Sum() (*big.Int, error)
    Submit() (string, error)
}

var jobCounter int

// GenerateWork using []string as Work (temporary until actual Work type is provided)
func GenerateWork(template *Template, auxBlocks map[string]*AuxBlock, chainName, arbitrary, poolPayoutPubScriptKey string, reservedArbitraryByteLength int) (*BitcoinBlock, []string, error) {
    if template == nil {
        return nil, []string{}, errors.New("template cannot be null")
    }

    var err error
    block := BitcoinBlock{}
    block.init(GetChain(chainName))
    block.Template = template

    block.reversePrevBlockHash, err = reverseHex4Bytes(block.Template.PrevBlockHash)
    if err != nil {
        return nil, []string{}, fmt.Errorf("invalid previous block hash hex: %v", err)
    }

    arbitraryBytes := bytesWithLengthHeader([]byte(arbitrary))
    arbitraryByteLength := uint(len(arbitraryBytes) + reservedArbitraryByteLength)
    arbitraryHex := hex.EncodeToString(arbitraryBytes)

    block.coinbaseInitial = block.Template.CoinbaseInitial(arbitraryByteLength).Serialize()
    block.coinbaseFinal = arbitraryHex + block.Template.CoinbaseFinal(poolPayoutPubScriptKey).Serialize()
    block.merkleSteps, err = block.Template.MerkleSteps()
    if err != nil {
        return nil, []string{}, fmt.Errorf("failed to generate merkle steps: %v", err)
    }

    // Assuming Work is a []string (based on original make(Work, 8))
    work := make([]string, 8)
    work[0] = fmt.Sprintf("%08x", jobCounter) // Job ID
    work[1] = block.reversePrevBlockHash
    work[2] = block.coinbaseInitial
    work[3] = block.coinbaseFinal
    work[4] = block.merkleSteps
    work[5] = fmt.Sprintf("%08x", block.Template.Version)
    work[6] = block.Template.Bits
    work[7] = fmt.Sprintf("%x", block.Template.CurrentTime)

    // Append auxpow data as additional strings (adjust based on actual AuxPow struct)
    for chainName, auxBlock := range auxBlocks {
        if auxBlock != nil {
            auxData := fmt.Sprintf("%s:%s", chainName, auxBlock.Hash)
            work = append(work, auxData)
        }
    }

    jobCounter++
    return &block, work, nil
}

// MakeHeader (unchanged except for nonceTime inclusion)
func (b *BitcoinBlock) MakeHeader(extranonce, nonce, nonceTime string) (string, error) {
    if b.Template == nil {
        return "", errors.New("generate work first")
    }

    var err error
    coinbase := Coinbase{
        CoinbaseInital: b.coinbaseInitial,
        Arbitrary:      extranonce,
        CoinbaseFinal:  b.coinbaseFinal,
    }

    b.coinbase = coinbase.Serialize()
    coinbaseHashed, err := b.CoinbaseHashed()
    if err != nil {
        return "", err
    }

    merkleRoot, err := makeHeaderMerkleRoot(coinbaseHashed, b.merkleSteps)
    if err != nil {
        return "", err
    }

    t := b.Template
    b.header, err = blockHeader(uint(t.Version), t.PrevBlockHash, merkleRoot, nonceTime, t.Bits, nonce)
    if err != nil {
        return "", err
    }

    return b.header, nil
}

// Remaining functions (unchanged)
func (b *BitcoinBlock) HeaderHashed() (string, error) {
    header, err := b.chain.CoinbaseDigest(b.header)
    if err != nil {
        return "", err
    }
    header, err = reverseHexBytes(header)
    if err != nil {
        return "", err
    }
    return header, nil
}

func (b *BitcoinBlock) CoinbaseHashed() (string, error) {
    return b.chain.CoinbaseDigest(b.coinbase)
}

func (b *BitcoinBlock) Sum() (*big.Int, error) {
    if b.chain == nil {
        return nil, errors.New("calculateSum: Missing blockchain interface")
    }
    if b.header == "" {
        return nil, errors.New("generate header first")
    }

    digest, err := b.chain.HeaderDigest(b.header)
    if err != nil {
        return nil, err
    }

    digest, err = reverseHexBytes(digest)
    if err != nil {
        return nil, err
    }

    b.hash = digest
    digestBytes, err := hex.DecodeString(digest)
    if err != nil {
        return nil, err
    }

    return new(big.Int).SetBytes(digestBytes), nil
}

func (b *BitcoinBlock) Submit() (string, error) {
    if b.header == "" {
        return "", errors.New("generate header first")
    }

    transactionPool := make([]string, len(b.Template.Transactions))
    for i, transaction := range b.Template.Transactions {
        transactionPool[i] = transaction.Data
    }

    submission := b.createSubmissionHex()
    if b.Template.MimbleWimble != "" {
        submission = submission + "01" + b.Template.MimbleWimble
    }

    return submission, nil
}

func debugMerkleSteps(block BitcoinBlock) {
    fmt.Println()
    fmt.Println("Transaction IDs")
    for i, transaction := range block.Template.Transactions {
        fmt.Println(i+1, transaction.ID, " : ", transaction.Data)
    }

    fmt.Println()
    fmt.Println("Steps")
    for i, step := range block.merkleSteps {
        fmt.Println(i+1, step)
    }
    fmt.Println()
}