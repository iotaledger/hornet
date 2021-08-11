package keymanager_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gohornet/hornet/pkg/keymanager"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/iotaledger/iota.go/v2/ed25519"
)

func TestMilestoneKeyManager(t *testing.T) {

	pubKey1, privKey1, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)

	pubKey2, privKey2, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)

	pubKey3, privKey3, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)

	km := keymanager.New()
	km.AddKeyRange(pubKey1, 0, 0)
	km.AddKeyRange(pubKey2, 3, 10)
	km.AddKeyRange(pubKey3, 8, 15)

	keysIndex0 := km.GetPublicKeysForMilestoneIndex(0)
	assert.Len(t, keysIndex0, 1)

	keysIndex3 := km.GetPublicKeysForMilestoneIndex(3)
	assert.Len(t, keysIndex3, 2)

	keysIndex7 := km.GetPublicKeysForMilestoneIndex(7)
	assert.Len(t, keysIndex7, 2)

	keysIndex8 := km.GetPublicKeysForMilestoneIndex(8)
	assert.Len(t, keysIndex8, 3)

	keysIndex10 := km.GetPublicKeysForMilestoneIndex(10)
	assert.Len(t, keysIndex10, 3)

	keysIndex11 := km.GetPublicKeysForMilestoneIndex(11)
	assert.Len(t, keysIndex11, 2)

	keysIndex15 := km.GetPublicKeysForMilestoneIndex(15)
	assert.Len(t, keysIndex15, 2)

	keysIndex16 := km.GetPublicKeysForMilestoneIndex(16)
	assert.Len(t, keysIndex16, 1)

	keysIndex1000 := km.GetPublicKeysForMilestoneIndex(1000)
	assert.Len(t, keysIndex1000, 1)

	keysSet8 := km.GetPublicKeysSetForMilestoneIndex(8)
	assert.Len(t, keysSet8, 3)

	var msPubKey1 iotago.MilestonePublicKey
	copy(msPubKey1[:], pubKey1)

	var msPubKey2 iotago.MilestonePublicKey
	copy(msPubKey2[:], pubKey2)

	var msPubKey3 iotago.MilestonePublicKey
	copy(msPubKey3[:], pubKey3)

	assert.Contains(t, keysSet8, msPubKey1)
	assert.Contains(t, keysSet8, msPubKey2)
	assert.Contains(t, keysSet8, msPubKey3)

	keyMapping8 := km.GetMilestonePublicKeyMappingForMilestoneIndex(8, []ed25519.PrivateKey{privKey1, privKey2, privKey3}, 2)
	assert.Len(t, keyMapping8, 2)

	assert.Equal(t, keyMapping8[msPubKey1], privKey1)
	assert.Equal(t, keyMapping8[msPubKey2], privKey2)
}
