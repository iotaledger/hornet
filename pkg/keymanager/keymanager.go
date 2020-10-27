package keymanager

import (
	"crypto/ed25519"
	"sort"

	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/utils"
)

type KeyRange struct {
	PublicKey  *iotago.MilestonePublicKey
	StartIndex milestone.Index
	EndIndex   milestone.Index
}

type KeyManager struct {
	keyRanges []*KeyRange
}

func New() *KeyManager {
	return &KeyManager{}
}

func (k *KeyManager) AddKeyRange(publicKey string, startIndex milestone.Index, endIndex milestone.Index) error {

	pubKey, err := utils.ParseEd25519PublicKeyFromString(publicKey)
	if err != nil {
		return err
	}

	var msPubKey iotago.MilestonePublicKey
	copy(msPubKey[:], pubKey)

	k.keyRanges = append(k.keyRanges, &KeyRange{PublicKey: &msPubKey, StartIndex: startIndex, EndIndex: endIndex})

	// sort by start index
	sort.Slice(k.keyRanges, func(i int, j int) bool {
		return k.keyRanges[i].StartIndex < k.keyRanges[j].StartIndex
	})

	return nil
}

func (k *KeyManager) GetPublicKeysForMilestoneIndex(msIndex milestone.Index) []*iotago.MilestonePublicKey {
	var pubKeys []*iotago.MilestonePublicKey

	for _, pubKeyRange := range k.keyRanges {
		if pubKeyRange.StartIndex <= msIndex {
			if pubKeyRange.EndIndex >= msIndex || pubKeyRange.StartIndex == pubKeyRange.EndIndex {
				// startIndex == endIndex means the key is valid forever
				pubKeys = append(pubKeys, pubKeyRange.PublicKey)
			}
			continue
		}
		break
	}

	return pubKeys
}

func (k *KeyManager) GetPublicKeysSetForMilestoneIndex(msIndex milestone.Index) iotago.MilestonePublicKeySet {
	pubKeys := k.GetPublicKeysForMilestoneIndex(msIndex)

	result := iotago.MilestonePublicKeySet{}

	for _, pubKey := range pubKeys {
		result[*pubKey] = struct{}{}
	}

	return result
}

func (k *KeyManager) GetKeyPairsForMilestoneIndex(msIndex milestone.Index, privateKeys []ed25519.PrivateKey, milestonePublicKeysCount int) iotago.MilestonePublicKeyMapping {
	pubKeySet := k.GetPublicKeysSetForMilestoneIndex(msIndex)

	result := iotago.MilestonePublicKeyMapping{}

	for _, privKey := range privateKeys {
		pubKey := privKey.Public().(ed25519.PublicKey)

		var msPubKey iotago.MilestonePublicKey
		copy(msPubKey[:], pubKey)

		if _, exists := pubKeySet[msPubKey]; exists {
			result[msPubKey] = privKey

			if len(result) == len(pubKeySet) {
				break
			}

			if len(result) == milestonePublicKeysCount {
				break
			}
		}
	}

	return result
}
