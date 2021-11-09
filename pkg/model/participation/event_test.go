package participation_test

import (
	"math/rand"
)

// TODO: Add tests for serialization

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandString(strLen int) string {
	b := make([]byte, strLen)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
