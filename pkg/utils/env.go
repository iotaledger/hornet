package utils

import (
	"fmt"
	"os"
	"strings"

	"github.com/iotaledger/iota.go/v2/ed25519"
)

// LoadStringFromEnvironment loads a string from the given environment variable.
func LoadStringFromEnvironment(name string) (string, error) {

	str, exists := os.LookupEnv(name)
	if !exists {
		return "", fmt.Errorf("environment variable '%s' not set", name)
	}

	if len(str) == 0 {
		return "", fmt.Errorf("environment variable '%s' not set", name)
	}

	return str, nil
}

// LoadEd25519PrivateKeysFromEnvironment loads ed25519 private keys from the given environment variable.
func LoadEd25519PrivateKeysFromEnvironment(name string) ([]ed25519.PrivateKey, error) {

	keys, exists := os.LookupEnv(name)
	if !exists {
		return nil, fmt.Errorf("environment variable '%s' not set", name)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("environment variable '%s' not set", name)
	}

	var privateKeys []ed25519.PrivateKey
	for _, key := range strings.Split(keys, ",") {
		privateKey, err := ParseEd25519PrivateKeyFromString(key)
		if err != nil {
			return nil, fmt.Errorf("environment variable '%s' contains an invalid private key '%s'", name, key)

		}
		privateKeys = append(privateKeys, privateKey)
	}

	return privateKeys, nil
}
