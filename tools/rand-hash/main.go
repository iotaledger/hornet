package main

import (
	"crypto/rand"
	"encoding/hex"
)

func main() {
	for i := 0; i < 10; i++ {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			panic(err)
		}

		println(hex.EncodeToString(b[:]))

	}
}
