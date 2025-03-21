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

// Updated to handle multiple aux chains
func GenerateWork(template *Template, auxBlocks map[string]*AuxBlock, chainName, arbitrary, poolPayoutPubScriptKey string, reservedArbitraryByteLength int) (*BitcoinBlock, Work, error) {
    if template == nil {
        return nil, nil, errors.New("Template cannot be null")
    }

    var err error
    block := BitcoinBlock{}
    block.init(GetChain(chainName))
    block.Template = template

    block.reversePrevBlockHash, err = reverseHex4Bytes(block.Template.PrevBlockHash)
    if err != nil {
        m := "invalid previous block hash hex: " + err.Error()
        return nil, nil, errors.New(m)
    }

    arbitraryBytes := bytesWithLengthHeader([]byte(arbitrary))
    arbitraryByteLength := uint(len(arbitraryBytes) + reservedArbitraryByteLength)
    arbitraryHex := hex.EncodeToString(arbitraryBytes)

    block.coinbaseInitial = block.Template.CoinbaseInitial(arbitraryByteLength).Serialize()
    block.coinbaseFinal = arbitraryHex + block.Template.CoinbaseFinal(poolPayoutPubScriptKey).Serialize()
    block.merkleSteps, err = block.Template.MerkleSteps()
    if err != nil {
        return nil, nil, err
    }

    // Populate work with auxpow data
    work := Work{
        JobID:       fmt.Sprintf("%08x", jobCounter),
        PrevHash:    block.reversePrevBlockHash,
        Coinb1:      block.coinbaseInitial,
        Coinb2:      block.coinbaseFinal,
        MerkleBranch: block.merkleSteps,
        Version:     fmt.Sprintf("%08x", block.Template.Version),
        NBits:       block.Template.Bits,
        NTime:       fmt.Sprintf("%x", block.Template.CurrentTime),
        AuxData:     make(map[string]AuxPow),
    }

    // Add auxpow data for each auxiliary chain
    for chainName, auxBlock := range auxBlocks {
        if auxBlock != nil {
            work.AuxData[chainName] = AuxPow{
                Target: auxBlock.Target,
                Hash:   auxBlock.Hash,
            }
        }
    }

    jobCounter++
    return &block, work, nil
}

// Updated Work struct (assumed)
type Work struct {
    JobID       string
    PrevHash    string
    Coinb1      string
    Coinb2      string
    MerkleBranch []string
    Version     string
    NBits       string
    NTime       string
    AuxData     map[string]AuxPow
}

type AuxPow struct {
    Target string
    Hash   string
}

// Adjust MakeHeader to include nonceTime
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

// Other functions (unchanged for brevity)
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