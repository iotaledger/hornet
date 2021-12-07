package testsuite

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

type MessageBuilder struct {
	te             *TestEnvironment
	indexation     string
	indexationData []byte

	parents hornet.MessageIDs

	fromWallet *utils.HDWallet
	toWallet   *utils.HDWallet

	amount uint64

	fakeInputs  bool
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

func (te *TestEnvironment) NewMessageBuilder(optionalIndexation ...string) *MessageBuilder {
	indexation := ""
	if len(optionalIndexation) > 0 {
		indexation = optionalIndexation[0]
	}
	return &MessageBuilder{
		te:         te,
		indexation: indexation,
	}
}

func (b *MessageBuilder) LatestMilestonesAsParents() *MessageBuilder {
	return b.Parents(hornet.MessageIDs{b.te.Milestones[len(b.te.Milestones)-1].Milestone().MessageID, b.te.Milestones[len(b.te.Milestones)-2].Milestone().MessageID})
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

func (b *MessageBuilder) FakeInputs() *MessageBuilder {
	b.fakeInputs = true
	return b
}

func (b *MessageBuilder) UsingOutput(output *utxo.Output) *MessageBuilder {
	b.outputToUse = output
	return b
}

func (b *MessageBuilder) IndexationData(data []byte) *MessageBuilder {
	b.indexationData = data
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

	msg, err := iotago.NewMessageBuilder().
		Parents(parents).
		Payload(&iotago.Indexation{Index: []byte(b.indexation), Data: b.indexationData}).
		Build()
	require.NoError(b.te.TestInterface, err)

	err = b.te.PoWHandler.DoPoW(context.Background(), msg, 1)
	require.NoError(b.te.TestInterface, err)

	message, err := storage.NewMessage(msg, serializer.DeSeriModePerformValidation, DeSerializationParameters)
	require.NoError(b.te.TestInterface, err)

	return &Message{
		builder: b,
		message: message,
	}
}

func (b *MessageBuilder) Build() *Message {

	require.Greaterf(b.te.TestInterface, b.amount, uint64(0), "trying to send a transaction with no value")

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
			fakeInputID := iotago.OutputID{}
			copy(fakeInputID[:], randBytes(iotago.TransactionIDLength))
			fakeInput := &iotago.ExtendedOutput{
				Address: fromAddr,
				Amount:  b.amount,
			}
			outputsThatCanBeConsumed = append(outputsThatCanBeConsumed, utxo.CreateOutput(&fakeInputID, hornet.NullMessageID(), 0, fakeInput))
		} else {
			outputsThatCanBeConsumed = b.fromWallet.Outputs()
		}
	}

	require.NotEmptyf(b.te.TestInterface, outputsThatCanBeConsumed, "no outputs available on the wallet")

	outputsBalance := uint64(0)
	for _, utxo := range outputsThatCanBeConsumed {
		outputsBalance += utxo.Amount()
	}

	require.GreaterOrEqualf(b.te.TestInterface, outputsBalance, b.amount, "not enough balance in the selected outputs to send the requested amount")

	for _, utxo := range outputsThatCanBeConsumed {

		builder.AddInput(&iotago.ToBeSignedUTXOInput{Address: fromAddr, Input: utxo.OutputID().UTXOInput()})
		consumedInputs = append(consumedInputs, utxo)
		consumedAmount += utxo.Amount()

		if consumedAmount >= b.amount {
			break
		}
	}

	builder.AddOutput(&iotago.ExtendedOutput{Address: toAddr, Amount: b.amount})

	var remainderAmount uint64
	if b.amount < consumedAmount {
		// Send remainder back to fromWallet
		remainderAmount = consumedAmount - b.amount
		builder.AddOutput(&iotago.ExtendedOutput{Address: fromAddr, Amount: remainderAmount})
	}

	if len(b.indexation) > 0 {
		builder.AddIndexationPayload(&iotago.Indexation{Index: []byte(b.indexation), Data: b.indexationData})
	}

	// Sign transaction
	inputPrivateKey, _ := b.fromWallet.KeyPair()
	inputAddrSigner := iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: fromAddr, Keys: inputPrivateKey})

	transaction, err := builder.Build(DeSerializationParameters, inputAddrSigner)
	require.NoError(b.te.TestInterface, err)

	require.NotNil(b.te.TestInterface, b.parents)

	msg, err := iotago.NewMessageBuilder().
		Parents(b.parents.ToSliceOfSlices()).
		Payload(transaction).Build()
	require.NoError(b.te.TestInterface, err)

	err = b.te.PoWHandler.DoPoW(context.Background(), msg, 1)
	require.NoError(b.te.TestInterface, err)

	message, err := storage.NewMessage(msg, serializer.DeSeriModePerformValidation, DeSerializationParameters)
	require.NoError(b.te.TestInterface, err)

	log := fmt.Sprintf("Send %d iota from %s to %s and remaining %d iota to original wallet", b.amount, fromAddr.Bech32(iotago.PrefixTestnet), toAddr.Bech32(iotago.PrefixTestnet), remainderAmount)
	if b.outputToUse != nil {
		usedType := iotago.OutputTypeToString(b.outputToUse.OutputType())
		log += fmt.Sprintf(" using UTXO: %s [%s]", b.outputToUse.OutputID().ToHex(), usedType)
	}
	fmt.Println(log)

	var sentOutput *utxo.Output
	var remainderOutput *utxo.Output

	// Book the outputs in the wallets
	messageTx := message.Transaction()
	txEssence := messageTx.Essence
	for i := range txEssence.Outputs {
		output, err := utxo.NewOutput(message.MessageID(), b.te.LastMilestoneIndex()+1, messageTx, uint16(i))
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

func (m *Message) RemainderUTXO() *utxo.Output {
	require.NotNil(m.builder.te.TestInterface, m.remainderOutput)
	return m.remainderOutput
}

func (m *Message) IotaMessage() *iotago.Message {
	return m.message.Message()
}

func (m *Message) StoredMessage() *storage.Message {
	return m.message
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
