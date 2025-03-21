package bitcoin

import (
    "regexp"
)

const BellscoinMinConfirmations = 102

type Bellscoin struct{}

func (Bellscoin) ChainName() string {
    return "bellscoin"
}

func (Bellscoin) CoinbaseDigest(coinbase string) (string, error) {
    return DoubleSha256(coinbase)
}

func (Bellscoin) HeaderDigest(header string) (string, error) {
    return ScryptDigest(header)
}

func (Bellscoin) ShareMultiplier() float64 {
    return 65536
}

func (Bellscoin) MinimumConfirmations() uint {
    return BellscoinMinConfirmations
}

func (Bellscoin) ValidMainnetAddress(address string) bool {
    // Bellscoin addresses start with "B" (P2PKH prefix 25)
    return regexp.MustCompile("^B[a-km-zA-HJ-NP-Z1-9]{33,34}$").MatchString(address)
}

func (Bellscoin) ValidTestnetAddress(address string) bool {
    // Assume testnet prefix similar to Dogecoin (e.g., "n" or "2"), adjust if known
    return regexp.MustCompile("^(n|2)[a-km-zA-HJ-NP-Z1-9]{33}$").MatchString(address)
}