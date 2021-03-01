package test

import (
	"bytes"
	"crypto"
	"encoding"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/whiteflag"

	// import implementation
	_ "golang.org/x/crypto/blake2b"
)

func mustMessageIDFromHexString(h string) encoding.BinaryMarshaler {
	msgID, err := hornet.MessageIDFromHex(h)
	if err != nil {
		panic(err)
	}
	return msgID
}

func TestWhiteFlagMerkleTreeHash(t *testing.T) {

	var includedMessages []encoding.BinaryMarshaler

	// https://github.com/Wollac/iota-crypto-demo/tree/master/examples/merkle

	includedMessages = append(includedMessages, mustMessageIDFromHexString("52fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c649"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("81855ad8681d0d86d1e91e00167939cb6694d2c422acd208a0072939487f6999"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("eb9d18a44784045d87f3c67cf22746e995af5a25367951baa2ff6cd471c483f1"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("5fb90badb37c5821b6d95526a41a9504680b4e7c8b763a1b1d49d4955c848621"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("6325253fec738dd7a9e28bf921119c160f0702448615bbda08313f6a8eb668d2"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("0bf5059875921e668a5bdf2c7fc4844592d2572bcd0668d2d6c52f5054e2d083"))
	includedMessages = append(includedMessages, mustMessageIDFromHexString("6bf84c7174cb7476364cc3dbd968b0f7172ed85794bb358b0c3b525da1786f9f"))

	hash, err := whiteflag.NewHasher(crypto.BLAKE2b_256).Hash(includedMessages)
	require.NoError(t, err)

	expectedHash, err := hex.DecodeString("bf67ce7ba23e8c0951b5abaec4f5524360d2c26d971ff226d3359fa70cdb0beb")
	require.NoError(t, err)
	require.True(t, bytes.Equal(hash, expectedHash))
}
