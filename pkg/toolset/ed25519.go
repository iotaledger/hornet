package toolset

import (
	"encoding/hex"
	"errors"
	"fmt"

	"crypto/ed25519"
)

func generateKeyEd25519(args []string) error {

	if len(args) > 0 {
		return errors.New("too many arguments for 'ed25519'")
	}

	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}

	fmt.Println("Your ed25519 private key: ", hex.EncodeToString(privKey))
	fmt.Println("Your ed25519 public key: ", hex.EncodeToString(pubKey))
	return nil
}
