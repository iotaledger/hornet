package main

import (
	"crypto/rand"
	"fmt"

	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/consts"
)

func main() {
	b := make([]byte, consts.HashTrytesSize)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	tryteAlphabetLength := len(consts.TryteAlphabet)
	var seed string
	for _, randByte := range b {
		seed += string(consts.TryteAlphabet[randByte%byte(tryteAlphabetLength)])
	}

	fmt.Println("seed", seed)

	addr, err := address.GenerateAddress(seed, 0, consts.SecurityLevelMedium, false)
	if err != nil {
		panic(err)
	}
	fmt.Printf("addr at index 0, sec lvl2: %s", addr)
}
