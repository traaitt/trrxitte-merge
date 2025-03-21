package bitcoin

import (
    "regexp"
)

const PepecoinMinConfirmations = 102

type Pepecoin struct{}

func (Pepecoin) ChainName() string {
    return "pepecoin"
}

func (Pepecoin) CoinbaseDigest(coinbase string) (string, error) {
    return DoubleSha256(coinbase)
}

func (Pepecoin) HeaderDigest(header string) (string, error) {
    return ScryptDigest(header)
}

func (Pepecoin) ShareMultiplier() float64 {
    return 65536
}

func (Pepecoin) MinimumConfirmations() uint {
    return PepecoinMinConfirmations
}

func (Pepecoin) ValidMainnetAddress(address string) bool {
    // Pepecoin addresses start with "P" (P2PKH prefix 60)
    return regexp.MustCompile("^P[a-km-zA-HJ-NP-Z1-9]{33,34}$").MatchString(address)
}

func (Pepecoin) ValidTestnetAddress(address string) bool {
    // Assume testnet prefix like Dogecoin’s
    return regexp.MustCompile("^(n|2)[a-km-zA-HJ-NP-Z1-9]{33}$").MatchString(address)
}