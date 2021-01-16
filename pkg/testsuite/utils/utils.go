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

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/pow"
)

const (
	pathString = "44'/4218'/0'/%d'"
)

type HDWallet struct {
	name  string
	seed  []byte
	index uint64
	utxo  []*utxo.Output
}

func NewHDWallet(name string, seed []byte, index uint64) *HDWallet {
	return &HDWallet{
		name:  name,
		seed:  seed,
		index: index,
		utxo:  make([]*utxo.Output, 0),
	}
}

func (hd *HDWallet) BookSpents(spentOutputs []*utxo.Output) {
	for _, spent := range spentOutputs {
		hd.BookSpent(spent)
	}
}

func (hd *HDWallet) BookSpent(spentOutput *utxo.Output) {
	newUtxo := make([]*utxo.Output, 0)
	for _, u := range hd.utxo {
		if bytes.Equal(u.UTXOKey(), spentOutput.UTXOKey()) {
			fmt.Printf("%s spent %s\n", hd.name, u.OutputID().ToHex())
			continue
		}
		newUtxo = append(newUtxo, u)
	}
	hd.utxo = newUtxo
}

func (hd *HDWallet) Name() string {
	return hd.name
}

func (hd *HDWallet) Balance() uint64 {
	var balance uint64
	for _, u := range hd.utxo {
		balance += u.Amount()
	}
	return balance
}

func (hd *HDWallet) BookOutput(output *utxo.Output) {
	if output != nil {
		fmt.Printf("%s book %s\n", hd.name, output.OutputID().ToHex())
		hd.utxo = append(hd.utxo, output)
	}
}

// KeyPair calculates an ed25519 key pair by using slip10.
func (hd *HDWallet) KeyPair() (ed25519.PrivateKey, ed25519.PublicKey) {

	path, err := bip32path.ParsePath(fmt.Sprintf(pathString, hd.index))
	if err != nil {
		panic(err)
	}

	curve := slip10.Ed25519()
	key, err := slip10.DeriveKeyFromPath(hd.seed, curve, path)
	if err != nil {
		panic(err)
	}

	pubKey, privKey := slip10.Ed25519Key(key)
	return privKey, pubKey
}

func (hd *HDWallet) Outputs() []*utxo.Output {
	return hd.utxo
}

// Address calculates an ed25519 address by using slip10.
func (hd *HDWallet) Address() *iotago.Ed25519Address {
	_, pubKey := hd.KeyPair()
	addr := iotago.AddressFromEd25519PubKey(pubKey)
	return &addr
}

func (hd *HDWallet) PrintStatus() {
	var status string
	status += fmt.Sprintf("Name: %s\n", hd.name)
	status += fmt.Sprintf("Address: %s\n", hd.Address().Bech32(iotago.PrefixTestnet))
	status += fmt.Sprintf("Balance: %d\n", hd.Balance())
	status += "Outputs: \n"
	for _, utxo := range hd.utxo {
		status += fmt.Sprintf("\t%s = %d\n", utxo.OutputID().ToHex(), utxo.Amount())
	}
	fmt.Printf("%s\n", status)
}

// MsgWithIndexation creates a zero value transaction to a random address with the given tag.
func MsgWithIndexation(t *testing.T, parent1 *hornet.MessageID, parent2 *hornet.MessageID, indexation string, powHandler *pow.Handler) *storage.Message {

	msg, err := iotago.NewMessageBuilder().Parent1(parent1.Slice()).Parent2(parent2.Slice()).Payload(&iotago.Indexation{Index: indexation, Data: nil}).Build()
	require.NoError(t, err)

	err = powHandler.DoPoW(msg, nil, 1)
	require.NoError(t, err)

	message, err := storage.NewMessage(msg, iotago.DeSeriModePerformValidation)
	require.NoError(t, err)

	return message
}

func MsgWithValueTx(t *testing.T, parent1 *hornet.MessageID, parent2 *hornet.MessageID, indexation string, fromWallet *HDWallet, toWallet *HDWallet, amount uint64, powHandler *pow.Handler) (message *storage.Message, consumedOutputs []*utxo.Output, sentOutput *utxo.Output, remainderOutput *utxo.Output) {
	return msgWithValueTx(t, parent1, parent2, indexation, fromWallet, toWallet, amount, powHandler, false, false)
}

func MsgWithInvalidValueTx(t *testing.T, parent1 *hornet.MessageID, parent2 *hornet.MessageID, indexation string, fromWallet *HDWallet, toWallet *HDWallet, amount uint64, powHandler *pow.Handler) (message *storage.Message, consumedOutputs []*utxo.Output, sentOutput *utxo.Output, remainderOutput *utxo.Output) {
	return msgWithValueTx(t, parent1, parent2, indexation, fromWallet, toWallet, amount, powHandler, true, false)
}

func MsgWithDustAllowance(t *testing.T, parent1 *hornet.MessageID, parent2 *hornet.MessageID, indexation string, fromWallet *HDWallet, toWallet *HDWallet, amount uint64, powHandler *pow.Handler) (message *storage.Message, consumedOutputs []*utxo.Output, sentOutput *utxo.Output, remainderOutput *utxo.Output) {
	return msgWithValueTx(t, parent1, parent2, indexation, fromWallet, toWallet, amount, powHandler, false, true)
}

func msgWithValueTx(t *testing.T, parent1 *hornet.MessageID, parent2 *hornet.MessageID, indexation string, fromWallet *HDWallet, toWallet *HDWallet, amount uint64, powHandler *pow.Handler, allowInvalidInputs bool, dustUnlock bool) (message *storage.Message, consumedOutputs []*utxo.Output, sentOutput *utxo.Output, remainderOutput *utxo.Output) {

	builder := iotago.NewTransactionBuilder()

	fromAddr := fromWallet.Address()
	toAddr := toWallet.Address()

	var consumedInputs []*utxo.Output
	var consumedAmount uint64

	walletOutputs := fromWallet.Outputs()

	if !allowInvalidInputs {
		require.NotEmpty(t, fromWallet.Outputs())
	}

	if allowInvalidInputs && len(walletOutputs) == 0 {
		// Add a fake output with enough balance to create a valid transaction
		walletOutputs = append(walletOutputs, utxo.GetOutput(&iotago.UTXOInputID{}, hornet.GetNullMessageID(), iotago.OutputSigLockedSingleOutput, fromAddr, amount))
	}

	for _, utxo := range walletOutputs {

		builder.AddInput(&iotago.ToBeSignedUTXOInput{Address: fromAddr, Input: utxo.UTXOInput()})
		consumedInputs = append(consumedInputs, utxo)
		consumedAmount += utxo.Amount()

		if consumedAmount >= amount {
			break
		}
	}

	if dustUnlock {
		builder.AddOutput(&iotago.SigLockedDustAllowanceOutput{Address: toAddr, Amount: amount})
	} else {
		builder.AddOutput(&iotago.SigLockedSingleOutput{Address: toAddr, Amount: amount})
	}

	var remainderAmount uint64
	if amount < consumedAmount {
		// Send remainder back to fromWallet
		remainderAmount = consumedAmount - amount
		builder.AddOutput(&iotago.SigLockedSingleOutput{Address: fromAddr, Amount: remainderAmount})
	}

	// Add indexation
	builder.AddIndexationPayload(&iotago.Indexation{Index: indexation, Data: nil})

	// Sign transaction
	inputPrivateKey, _ := fromWallet.KeyPair()
	inputAddrSigner := iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: fromAddr, Keys: inputPrivateKey})

	transaction, err := builder.Build(inputAddrSigner)
	require.NoError(t, err)

	msg, err := iotago.NewMessageBuilder().Parent1(parent1.Slice()).Parent2(parent2.Slice()).Payload(transaction).Build()
	require.NoError(t, err)

	err = powHandler.DoPoW(msg, nil, 1)
	require.NoError(t, err)

	message, err = storage.NewMessage(msg, iotago.DeSeriModePerformValidation)
	require.NoError(t, err)

	fmt.Println(fmt.Sprintf("Send %d iota from %s to %s and remaining %d iota to original wallet", amount, fromAddr.Bech32(iotago.PrefixTestnet), toAddr.Bech32(iotago.PrefixTestnet), remainderAmount))

	// Book the outputs in the wallets
	messageTx := message.GetTransaction()
	txEssence := messageTx.Essence.(*iotago.TransactionEssence)
	for i, _ := range txEssence.Outputs {
		output, err := utxo.NewOutput(message.GetMessageID(), messageTx, uint16(i))
		require.NoError(t, err)

		if bytes.Equal(output.Address()[:], toAddr[:]) {
			sentOutput = output
			continue
		}

		if remainderAmount > 0 && bytes.Equal(output.Address()[:], fromAddr[:]) {
			remainderOutput = output
		}
	}

	return message, consumedInputs, sentOutput, remainderOutput
}
