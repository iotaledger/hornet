package utils

import (
	"encoding/hex"

	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/v2/ed25519"
)

var (
	ErrInvalidKeyLength = errors.New("invalid key length")
)

func ParseEd25519PublicKeyFromString(key string) (ed25519.PublicKey, error) {

	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return nil, err
	}

	if len(keyBytes) != ed25519.PublicKeySize {
		return nil, ErrInvalidKeyLength
	}

	return ed25519.PublicKey(keyBytes), nil
}

func ParseEd25519PrivateKeyFromString(key string) (ed25519.PrivateKey, error) {

	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return nil, err
	}

	if len(keyBytes) != ed25519.PrivateKeySize {
		return nil, ErrInvalidKeyLength
	}

	return ed25519.PrivateKey(keyBytes), nil
}
