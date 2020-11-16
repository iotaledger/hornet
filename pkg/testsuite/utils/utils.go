package utils

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ed25519"

	iotago "github.com/iotaledger/iota.go"
	"github.com/wollac/iota-crypto-demo/pkg/bip32path"
	"github.com/wollac/iota-crypto-demo/pkg/slip10"

	"github.com/gohornet/hornet/core/pow"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
)

var (
	pathString = "44'/4218'/0'/%d'"
)

// GenerateHDWalletKeyPair calculates an ed25519 key pair by using slip10.
func GenerateHDWalletKeyPair(t *testing.T, seed []byte, index uint64) (ed25519.PrivateKey, ed25519.PublicKey) {

	path, err := bip32path.ParsePath(fmt.Sprintf(pathString, index))
	require.NoError(t, err)

	curve := slip10.Ed25519()
	key, err := slip10.DeriveKeyFromPath(seed, curve, path)
	require.NoError(t, err)

	pubKey, privKey := slip10.Ed25519Key(key)
	return privKey, pubKey
}

// GenerateHDWalletAddress calculates an ed25519 address by using slip10.
func GenerateHDWalletAddress(t *testing.T, seed []byte, index uint64) iotago.Ed25519Address {
	_, pubKey := GenerateHDWalletKeyPair(t, seed, index)
	return iotago.AddressFromEd25519PubKey(pubKey)
}

// MsgWithIndexation creates a zero value transaction to a random address with the given tag.
func MsgWithIndexation(t *testing.T, parent1 *hornet.MessageID, parent2 *hornet.MessageID, indexation string) *storage.Message {

	msg := &iotago.Message{
		Version: iotago.MessageVersion,
		Parent1: *parent1,
		Parent2: *parent2,
		Payload: &iotago.Indexation{Index: indexation, Data: nil},
	}

	err := pow.Handler().DoPoW(msg, nil, 1)
	require.NoError(t, err)

	message, err := storage.NewMessage(msg, iotago.DeSeriModePerformValidation)
	require.NoError(t, err)

	return message
}

// MsgWithValueTx creates a value transaction with the given tag from an input seed index to an address created by a given output seed and index.
func MsgWithValueTx(t *testing.T, parent1 *hornet.MessageID, parent2 *hornet.MessageID, indexation string, inputUTXOs []*iotago.UTXOInput, fromSeed []byte, fromIndices []uint64, balances []uint64,
	toRemainderIndex uint64, toSeed []byte, toIndex uint64, value uint64) (message *storage.Message, remainderUTXOInput *iotago.UTXOInput, sentToUTXOInput *iotago.UTXOInput) {

	require.Equal(t, len(inputUTXOs), len(fromIndices))
	require.Equal(t, len(inputUTXOs), len(balances))

	msg := &iotago.Message{
		Version: iotago.MessageVersion,
		Parent1: *parent1,
		Parent2: *parent2,
	}

	var totalInputBalance uint64
	for _, balance := range balances {
		totalInputBalance += balance
	}

	builder := iotago.NewTransactionBuilder()

	addressKeys := []iotago.AddressKeys{}
	for i, inputUTXO := range inputUTXOs {
		inputPrivateKey, inputPublicKey := GenerateHDWalletKeyPair(t, fromSeed, fromIndices[i])
		inputAddr := iotago.AddressFromEd25519PubKey(inputPublicKey)
		builder.AddInput(&iotago.ToBeSignedUTXOInput{Address: &inputAddr, Input: inputUTXO})
		addressKeys = append(addressKeys, iotago.AddressKeys{Address: &inputAddr, Keys: inputPrivateKey})
	}

	inputAddrSigner := iotago.NewInMemoryAddressSigner(addressKeys...)

	_, remainderPublicKey := GenerateHDWalletKeyPair(t, fromSeed, toRemainderIndex)
	remainderAddr := iotago.AddressFromEd25519PubKey(remainderPublicKey)

	if totalInputBalance != value {
		// there is a remander
		builder.AddOutput(&iotago.SigLockedSingleOutput{Address: &remainderAddr, Amount: totalInputBalance - value})
	}

	_, outputPublicKey := GenerateHDWalletKeyPair(t, toSeed, toIndex)
	outputAddr := iotago.AddressFromEd25519PubKey(outputPublicKey)
	builder.AddOutput(&iotago.SigLockedSingleOutput{Address: &outputAddr, Amount: value})
	builder.AddIndexationPayload(&iotago.Indexation{Index: indexation, Data: nil})

	tx, err := builder.Build(inputAddrSigner)
	require.NoError(t, err)

	transactionID, err := tx.ID()
	require.NoError(t, err)

	// search the output indexes
	txEssence := tx.Essence.(*iotago.TransactionEssence)
	for i, output := range txEssence.Outputs {
		sigLockedSingleOutput := output.(*iotago.SigLockedSingleOutput)
		ed25519Address := sigLockedSingleOutput.Address.(*iotago.Ed25519Address)

		if bytes.Equal(ed25519Address[:], remainderAddr[:]) {
			remainderUTXOInput = &iotago.UTXOInput{TransactionID: *transactionID, TransactionOutputIndex: uint16(i)}
			continue
		}

		if bytes.Equal(ed25519Address[:], outputAddr[:]) {
			sentToUTXOInput = &iotago.UTXOInput{TransactionID: *transactionID, TransactionOutputIndex: uint16(i)}
		}
	}

	msg.Payload = tx

	err = pow.Handler().DoPoW(msg, nil, 1)
	require.NoError(t, err)

	message, err = storage.NewMessage(msg, iotago.DeSeriModePerformValidation)
	require.NoError(t, err)

	fmt.Println(fmt.Sprintf("Send %d iota to %s and remaining %d iota to %s", value, outputAddr.Bech32(iotago.PrefixTestnet), totalInputBalance-value, remainderAddr.Bech32(iotago.PrefixTestnet)))

	return message, remainderUTXOInput, sentToUTXOInput
}
