package protocfg

import (
	"github.com/iotaledger/hive.go/core/crypto"
	"github.com/iotaledger/iota.go/v3/keymanager"
)

func KeyManagerWithConfigPublicKeyRanges(coordinatorPublicKeyRanges ConfigPublicKeyRanges) (*keymanager.KeyManager, error) {
	keyManager := keymanager.New()
	for _, keyRange := range coordinatorPublicKeyRanges {
		pubKey, err := crypto.ParseEd25519PublicKeyFromString(keyRange.Key)
		if err != nil {
			return nil, err
		}

		keyManager.AddKeyRange(pubKey, keyRange.StartIndex, keyRange.EndIndex)
	}

	return keyManager, nil
}
