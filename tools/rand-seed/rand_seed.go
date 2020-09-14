package main

import (
	"crypto/rand"
	"fmt"

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

	fmt.Println(seed)
}
