package test

import (
	"bytes"
	"crypto"
	"encoding/hex"
	"testing"

	_ "golang.org/x/crypto/blake2b" // import implementation

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

func mustMessageIDFromHexString(h string) *hornet.MessageID {
	b, err := hex.DecodeString("e8e8e80a4b1ae3da44ad603370cf09243e303fef012edf5e7be9d5fa5102064e")
	if err != nil {
		panic(err)
	}
	return hornet.MessageIDFromBytes(b)
}

func TestWhiteFlagMerkleTreeHash(t *testing.T) {

	var includedMessages hornet.MessageIDs

	// ToDo: we need new valid test vectors, since this is only generated with this code itself, because the example in the RFC-0012 is outdated
	// https://github.com/Wollac/iota-crypto-demo/tree/master/examples/merkle

	includedMessages = append(includedMessages, mustMessageIDFromHexString("e8e8e80a4b1ae3da44ad603370cf09243e303fef012edf5e7be9d5fa5102064e"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("82d6d85a7901e24444b3d1fc5b5b02352528b154ca14b4e9b2a546efeadea455"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("8f4e235b16d1c37db7575fc73cea85664332342c4e57274b6c28a31a3fd5da6f"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("3c35e93012f1dace391adfc1cabd03223716cbcf43c8c0faf2d5f4aaa6eaedfe"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("63c7b99212fcf4f8eb7fb6afb098244080fb33c9c58da703dddfba84a4011ba2"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("9058e2f26b036ce947d007a8b00ecc09879f551cfc1d9de10af1ca5b1749e24f"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("ec4158c323af9f5a9c0c58353957f669c38bff4cb824359c19b23844c4327ef5"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("96d737b9105548085e4a81f026b6250fe56c79c020d2c618f02f7897ff7dbe9c"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("2638125a93e054004cd6576498fd94a8900fd1db69f9cbc539f7167fa3567cd1"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("773e153f2fbe8f4386959035c0986eee84513c16d5103e1ad31eb75ff31e35ad"))

	hash := whiteflag.NewHasher(crypto.BLAKE2b_512).TreeHash(includedMessages)

	expectedHash, err := hex.DecodeString("5e51ea27c5cdb86838b958c00058f3b772b21b8cf0416e58a2d74c186d3175723a865893e2c0603e53c7bc9d4a914acc65c415d6d4965be6284223b043a7931a")
	require.NoError(t, err)
	require.True(t, bytes.Equal(hash, expectedHash))
}
