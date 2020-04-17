package toolset

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

func seedGen(args []string) error {

	if len(args) > 0 {
		return errors.New("too many arguments for 'seedGen'")
	}

	rand.Seed(time.Now().UnixNano())

	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, 32)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	fmt.Println("Your autopeering seed: ", base64.StdEncoding.EncodeToString([]byte(string(b))))

	return nil
}
