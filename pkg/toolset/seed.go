package toolset

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"time"
)

func seedGen(args []string) {

	if len(args) > 0 {
		fmt.Println("Too many arguments for 'seedGen'")
		os.Exit(0)
	}

	rand.Seed(time.Now().UnixNano())

	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, 32)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	fmt.Println("Your autopeering seed: ", base64.StdEncoding.EncodeToString([]byte(string(b))))
	os.Exit(0)
}
