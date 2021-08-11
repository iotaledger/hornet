package testsuite

import (
	"fmt"
	"math/rand"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	iotago "github.com/iotaledger/iota.go/v2"
)

type MessageBuilder struct {
	te         *TestEnvironment
	indexation string

	parents hornet.MessageIDs

	fromWallet *utils.HDWallet
	toWallet   *utils.HDWallet

	amount uint64

	fakeInputs  bool
	dustUnlock  bool
	outputToUse *utxo.Output
}

type Message struct {
	builder *MessageBuilder
	message *storage.Message

	consumedOutputs []*utxo.Output
	sentOutput      *utxo.Output
	remainderOutput *utxo.Output

	booked          bool
	storedMessageID hornet.MessageID
}

func (te *TestEnvironment) NewMessageBuilder(indexation string) *MessageBuilder {
	return &MessageBuilder{
		te:         te,
		indexation: indexation,
	}
}

func (b *MessageBuilder) Parents(parents hornet.MessageIDs) *MessageBuilder {
	b.parents = parents
	return b
}

func (b *MessageBuilder) FromWallet(wallet *utils.HDWallet) *MessageBuilder {
	b.fromWallet = wallet
	return b
}

func (b *MessageBuilder) ToWallet(wallet *utils.HDWallet) *MessageBuilder {
	b.toWallet = wallet
	return b
}

func (b *MessageBuilder) Amount(amount uint64) *MessageBuilder {
	b.amount = amount
	return b
}

func (b *MessageBuilder) DustAllowance() *MessageBuilder {
	b.dustUnlock = true
	return b
}

func (b *MessageBuilder) FakeInputs() *MessageBuilder {
	b.fakeInputs = true
	return b
}

func (b *MessageBuilder) UsingOutput(output *utxo.Output) *MessageBuilder {
	b.outputToUse = output
	return b
}

func (b *MessageBuilder) BuildIndexation() *Message {

	require.NotEmpty(b.te.TestInterface, b.indexation)

	parents := [][]byte{}
	require.NotNil(b.te.TestInterface, b.parents)
	for _, parent := range b.parents {
		require.NotNil(b.te.TestInterface, parent)
		parents = append(parents, parent[:])
	}

	msg, err := iotago.NewMessageBuilder().Parents(parents).Payload(&iotago.Indexation{Index: []byte(b.indexation), Data: nil}).Build()
	require.NoError(b.te.TestInterface, err)

	err = b.te.PoWHandler.DoPoW(msg, nil, 1)
	require.NoError(b.te.TestInterface, err)

	message, err := storage.NewMessage(msg, iotago.DeSeriModePerformValidation)
	require.NoError(b.te.TestInterface, err)

	return &Message{
		builder: b,
		message: message,
	}
}

func (b *MessageBuilder) Build() *Message {

	require.True(b.te.TestInterface, b.amount > 0)

	builder := iotago.NewTransactionBuilder()

	fromAddr := b.fromWallet.Address()
	toAddr := b.toWallet.Address()

	var consumedInputs []*utxo.Output
	var consumedAmount uint64

	var outputsThatCanBeConsumed []*utxo.Output

	if b.outputToUse != nil {
		// Only use the given output
		outputsThatCanBeConsumed = append(outputsThatCanBeConsumed, b.outputToUse)
	} else {
		if b.fakeInputs {
			// Add a fake output with enough balance to create a valid transaction
			fakeInput := iotago.UTXOInputID{}
			copy(fakeInput[:], randBytes(iotago.TransactionIDLength))
			outputsThatCanBeConsumed = append(outputsThatCanBeConsumed, utxo.CreateOutput(&fakeInput, hornet.NullMessageID(), iotago.OutputSigLockedSingleOutput, fromAddr, b.amount))
		} else {
			outputsThatCanBeConsumed = b.fromWallet.Outputs()
		}
	}

	require.NotEmpty(b.te.TestInterface, outputsThatCanBeConsumed)

	for _, utxo := range outputsThatCanBeConsumed {

		builder.AddInput(&iotago.ToBeSignedUTXOInput{Address: fromAddr, Input: utxo.UTXOInput()})
		consumedInputs = append(consumedInputs, utxo)
		consumedAmount += utxo.Amount()

		if consumedAmount >= b.amount {
			break
		}
	}

	if b.dustUnlock {
		builder.AddOutput(&iotago.SigLockedDustAllowanceOutput{Address: toAddr, Amount: b.amount})
	} else {
		builder.AddOutput(&iotago.SigLockedSingleOutput{Address: toAddr, Amount: b.amount})
	}

	var remainderAmount uint64
	if b.amount < consumedAmount {
		// Send remainder back to fromWallet
		remainderAmount = consumedAmount - b.amount
		builder.AddOutput(&iotago.SigLockedSingleOutput{Address: fromAddr, Amount: remainderAmount})
	}

	require.NotEmpty(b.te.TestInterface, b.indexation)
	builder.AddIndexationPayload(&iotago.Indexation{Index: []byte(b.indexation), Data: nil})

	// Sign transaction
	inputPrivateKey, _ := b.fromWallet.KeyPair()
	inputAddrSigner := iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: fromAddr, Keys: inputPrivateKey})

	transaction, err := builder.Build(inputAddrSigner)
	require.NoError(b.te.TestInterface, err)

	require.NotNil(b.te.TestInterface, b.parents)

	msg, err := iotago.NewMessageBuilder().Parents(b.parents.ToSliceOfSlices()).Payload(transaction).Build()
	require.NoError(b.te.TestInterface, err)

	err = b.te.PoWHandler.DoPoW(msg, nil, 1)
	require.NoError(b.te.TestInterface, err)

	message, err := storage.NewMessage(msg, iotago.DeSeriModePerformValidation)
	require.NoError(b.te.TestInterface, err)

	var outputType string
	if b.dustUnlock {
		outputType = "DustAllowance"
	} else {
		outputType = "SingleOutput"
	}

	log := fmt.Sprintf("Send %d iota %s from %s to %s and remaining %d iota to original wallet", b.amount, outputType, fromAddr.Bech32(iotago.PrefixTestnet), toAddr.Bech32(iotago.PrefixTestnet), remainderAmount)
	if b.outputToUse != nil {
		var usedType string
		switch b.outputToUse.OutputType() {
		case iotago.OutputSigLockedDustAllowanceOutput:
			usedType = "DustAllowance"
		case iotago.OutputSigLockedSingleOutput:
			usedType = "SingleOutput"
		default:
			usedType = fmt.Sprintf("%d", b.outputToUse.OutputType())
		}
		log += fmt.Sprintf(" using UTXO: %s [%s]", b.outputToUse.OutputID().ToHex(), usedType)
	}
	fmt.Println(log)

	var sentOutput *utxo.Output
	var remainderOutput *utxo.Output

	// Book the outputs in the wallets
	messageTx := message.Transaction()
	txEssence := messageTx.Essence.(*iotago.TransactionEssence)
	for i := range txEssence.Outputs {
		output, err := utxo.NewOutput(message.MessageID(), messageTx, uint16(i))
		require.NoError(b.te.TestInterface, err)

		if output.Address().String() == toAddr.String() && output.Amount() == b.amount {
			sentOutput = output
			continue
		}

		if remainderAmount > 0 && output.Address().String() == fromAddr.String() && output.Amount() == remainderAmount {
			remainderOutput = output
		}
	}

	return &Message{
		builder:         b,
		message:         message,
		consumedOutputs: consumedInputs,
		sentOutput:      sentOutput,
		remainderOutput: remainderOutput,
	}
}

func (m *Message) Store() *Message {
	require.Nil(m.builder.te.TestInterface, m.storedMessageID)
	m.storedMessageID = m.builder.te.StoreMessage(m.message).Message().MessageID()
	return m
}

func (m *Message) BookOnWallets() *Message {

	require.False(m.builder.te.TestInterface, m.booked)
	m.builder.fromWallet.BookSpents(m.consumedOutputs)
	m.builder.toWallet.BookOutput(m.sentOutput)
	m.builder.fromWallet.BookOutput(m.remainderOutput)
	m.booked = true

	return m
}

func (m *Message) GeneratedUTXO() *utxo.Output {
	require.NotNil(m.builder.te.TestInterface, m.sentOutput)
	return m.sentOutput
}

func (m *Message) StoredMessageID() hornet.MessageID {
	require.NotNil(m.builder.te.TestInterface, m.storedMessageID)
	return m.storedMessageID
}

// returns length amount random bytes
func randBytes(length int) []byte {
	var b []byte
	for i := 0; i < length; i++ {
		b = append(b, byte(rand.Intn(256)))
	}
	return b
}
