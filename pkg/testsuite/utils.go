package testsuite

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/builder"
)

type BlockBuilder struct {
	te      *TestEnvironment
	tag     string
	tagData []byte

	parents iotago.BlockIDs

	fromWallet *utils.HDWallet
	toWallet   *utils.HDWallet

	amount uint64

	fakeInputs  bool
	outputToUse *utxo.Output
}

type Block struct {
	builder *BlockBuilder
	block   *storage.Block

	consumedOutputs []*utxo.Output
	sentOutput      *utxo.Output
	remainderOutput *utxo.Output

	booked        bool
	storedBlockID iotago.BlockID
}

func (te *TestEnvironment) NewBlockBuilder(optionalTag ...string) *BlockBuilder {
	tag := ""
	if len(optionalTag) > 0 {
		tag = optionalTag[0]
	}
	return &BlockBuilder{
		te:  te,
		tag: tag,
	}
}

func (b *BlockBuilder) LatestMilestoneAsParents() *BlockBuilder {
	return b.Parents(iotago.BlockIDs{b.te.coo.lastMilestoneBlockID})
}

func (b *BlockBuilder) Parents(parents iotago.BlockIDs) *BlockBuilder {
	b.parents = parents
	return b
}

func (b *BlockBuilder) FromWallet(wallet *utils.HDWallet) *BlockBuilder {
	b.fromWallet = wallet
	return b
}

func (b *BlockBuilder) ToWallet(wallet *utils.HDWallet) *BlockBuilder {
	b.toWallet = wallet
	return b
}

func (b *BlockBuilder) Amount(amount uint64) *BlockBuilder {
	b.amount = amount
	return b
}

func (b *BlockBuilder) FakeInputs() *BlockBuilder {
	b.fakeInputs = true
	return b
}

func (b *BlockBuilder) UsingOutput(output *utxo.Output) *BlockBuilder {
	b.outputToUse = output
	return b
}

func (b *BlockBuilder) TagData(data []byte) *BlockBuilder {
	b.tagData = data
	return b
}

func (b *BlockBuilder) BuildTaggedData() *Block {

	require.NotEmpty(b.te.TestInterface, b.tag)

	iotaBlock, err := builder.NewBlockBuilder(b.te.protoParas.Version).
		ParentsBlockIDs(b.parents).
		Payload(&iotago.TaggedData{Tag: []byte(b.tag), Data: b.tagData}).
		Build()
	require.NoError(b.te.TestInterface, err)

	_, err = b.te.PoWHandler.DoPoW(context.Background(), iotaBlock, 1)
	require.NoError(b.te.TestInterface, err)

	block, err := storage.NewBlock(iotaBlock, serializer.DeSeriModePerformValidation, b.te.protoParas)
	require.NoError(b.te.TestInterface, err)

	return &Block{
		builder: b,
		block:   block,
	}
}

func (b *BlockBuilder) Build() *Block {

	require.Greaterf(b.te.TestInterface, b.amount, uint64(0), "trying to send a transaction with no value")

	txBuilder := builder.NewTransactionBuilder(b.te.protoParas.NetworkID())

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
			fakeInput := &iotago.BasicOutput{
				Amount: b.amount,
				Conditions: iotago.UnlockConditions{
					&iotago.AddressUnlockCondition{
						Address: fromAddr,
					},
				},
			}
			outputsThatCanBeConsumed = append(outputsThatCanBeConsumed, utxo.CreateOutput(&fakeInputID, iotago.EmptyBlockID(), 0, 0, fakeInput))
		} else {
			outputsThatCanBeConsumed = b.fromWallet.Outputs()
		}
	}

	require.NotEmptyf(b.te.TestInterface, outputsThatCanBeConsumed, "no outputs available on the wallet")

	outputsBalance := uint64(0)
	for _, output := range outputsThatCanBeConsumed {
		outputsBalance += output.Deposit()
	}

	require.GreaterOrEqualf(b.te.TestInterface, outputsBalance, b.amount, "not enough balance in the selected outputs to send the requested amount")

	for _, output := range outputsThatCanBeConsumed {
		txBuilder.AddInput(&builder.ToBeSignedUTXOInput{Address: fromAddr, OutputID: *output.OutputID(), Output: output.Output()})
		consumedInputs = append(consumedInputs, output)
		consumedAmount += output.Deposit()

		if consumedAmount >= b.amount {
			break
		}
	}

	txBuilder.AddOutput(&iotago.BasicOutput{Conditions: iotago.UnlockConditions{&iotago.AddressUnlockCondition{Address: toAddr}}, Amount: b.amount})

	var remainderAmount uint64
	if b.amount < consumedAmount {
		// Send remainder back to fromWallet
		remainderAmount = consumedAmount - b.amount
		txBuilder.AddOutput(&iotago.BasicOutput{Conditions: iotago.UnlockConditions{&iotago.AddressUnlockCondition{Address: fromAddr}}, Amount: remainderAmount})
	}

	if len(b.tag) > 0 {
		txBuilder.AddTaggedDataPayload(&iotago.TaggedData{Tag: []byte(b.tag), Data: b.tagData})
	}

	// Sign transaction
	inputPrivateKey, _ := b.fromWallet.KeyPair()
	inputAddrSigner := iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: fromAddr, Keys: inputPrivateKey})

	transaction, err := txBuilder.Build(b.te.protoParas, inputAddrSigner)
	require.NoError(b.te.TestInterface, err)

	require.NotNil(b.te.TestInterface, b.parents)

	iotaBlock, err := builder.NewBlockBuilder(b.te.protoParas.Version).
		ParentsBlockIDs(b.parents).
		Payload(transaction).Build()
	require.NoError(b.te.TestInterface, err)

	_, err = b.te.PoWHandler.DoPoW(context.Background(), iotaBlock, 1)
	require.NoError(b.te.TestInterface, err)

	block, err := storage.NewBlock(iotaBlock, serializer.DeSeriModePerformValidation, b.te.protoParas)
	require.NoError(b.te.TestInterface, err)

	log := fmt.Sprintf("Send %d iota from %s to %s and remaining %d iota to original wallet", b.amount, fromAddr.Bech32(iotago.PrefixTestnet), toAddr.Bech32(iotago.PrefixTestnet), remainderAmount)
	if b.outputToUse != nil {
		log += fmt.Sprintf(" using UTXO: %s [%s]", b.outputToUse.OutputID().ToHex(), b.outputToUse.OutputType().String())
	}
	fmt.Println(log)

	var sentOutput *utxo.Output
	var remainderOutput *utxo.Output

	// Book the outputs in the wallets
	blockTx := block.Transaction()
	txEssence := blockTx.Essence
	for i := range txEssence.Outputs {
		output, err := utxo.NewOutput(block.BlockID(), b.te.LastMilestoneIndex()+1, 0, blockTx, uint16(i))
		require.NoError(b.te.TestInterface, err)

		switch iotaOutput := output.Output().(type) {
		case *iotago.BasicOutput:
			conditions := iotaOutput.UnlockConditionsSet()
			if conditions.Address().Address.Equal(toAddr) && output.Deposit() == b.amount {
				sentOutput = output
				continue
			}

			if remainderAmount > 0 && conditions.Address().Address.Equal(fromAddr) && output.Deposit() == remainderAmount {
				remainderOutput = output
			}
		default:
			continue
		}
	}

	return &Block{
		builder:         b,
		block:           block,
		consumedOutputs: consumedInputs,
		sentOutput:      sentOutput,
		remainderOutput: remainderOutput,
	}
}

func (m *Block) Store() *Block {
	require.True(m.builder.te.TestInterface, m.storedBlockID.Empty())
	m.storedBlockID = m.builder.te.StoreBlock(m.block).Block().BlockID()
	return m
}

func (m *Block) BookOnWallets() *Block {

	require.False(m.builder.te.TestInterface, m.booked)
	m.builder.fromWallet.BookSpents(m.consumedOutputs)
	m.builder.toWallet.BookOutput(m.sentOutput)
	m.builder.fromWallet.BookOutput(m.remainderOutput)
	m.booked = true

	return m
}

func (m *Block) GeneratedUTXO() *utxo.Output {
	require.NotNil(m.builder.te.TestInterface, m.sentOutput)
	return m.sentOutput
}

func (m *Block) RemainderUTXO() *utxo.Output {
	require.NotNil(m.builder.te.TestInterface, m.remainderOutput)
	return m.remainderOutput
}

func (m *Block) IotaBlock() *iotago.Block {
	return m.block.Block()
}

func (m *Block) StoredBlock() *storage.Block {
	return m.block
}

func (m *Block) StoredBlockID() iotago.BlockID {
	require.NotNil(m.builder.te.TestInterface, m.storedBlockID)
	return m.storedBlockID
}

// returns length amount random bytes
func randBytes(length int) []byte {
	var b []byte
	for i := 0; i < length; i++ {
		b = append(b, byte(rand.Intn(256)))
	}
	return b
}
