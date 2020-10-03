package toolset

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/mr-tron/base58"
)

func seedGen(args []string) error {

	if len(args) > 0 {
		return fmt.Errorf("too many arguments for '%s'", ToolSeedGen)
	}

	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	lenLetterRunes := int64(len(letterRunes))

	b := make([]rune, 32)
	for i := range b {
		nBig, err := rand.Int(rand.Reader, big.NewInt(lenLetterRunes))
		if err != nil {
			panic(err)
		}
		b[i] = letterRunes[nBig.Int64()]
	}

	fmt.Println("Your autopeering seed: ", base58.Encode([]byte(string(b))))

	return nil
}
