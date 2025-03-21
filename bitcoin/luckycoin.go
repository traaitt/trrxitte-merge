package bitcoin

import (
    "regexp"
)

const LuckycoinMinConfirmations = 102

type Luckycoin struct{}

func (Luckycoin) ChainName() string {
    return "luckycoin"
}

func (Luckycoin) CoinbaseDigest(coinbase string) (string, error) {
    return DoubleSha256(coinbase)
}

func (Luckycoin) HeaderDigest(header string) (string, error) {
    return ScryptDigest(header)
}

func (Luckycoin) ShareMultiplier() float64 {
    return 65536
}

func (Luckycoin) MinimumConfirmations() uint {
    return LuckycoinMinConfirmations
}

func (Luckycoin) ValidMainnetAddress(address string) bool {
    // Luckycoin addresses start with "L" (P2PKH prefix 47)
    return regexp.MustCompile("^L[a-km-zA-HJ-NP-Z1-9]{33,34}$").MatchString(address)
}

func (Luckycoin) ValidTestnetAddress(address string) bool {
    // Assume testnet prefix like Dogecoin’s
    return regexp.MustCompile("^(n|2)[a-km-zA-HJ-NP-Z1-9]{33}$").MatchString(address)
}