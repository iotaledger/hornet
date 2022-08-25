//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package testsuite

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/testsuite/utils"
	"github.com/iotaledger/hornet/v2/pkg/tpkg"
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

	consumedOutputs utxo.Outputs
	createdOutputs  utxo.Outputs

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

func (b *BlockBuilder) fromWalletSigner() iotago.AddressSigner {
	require.NotNil(b.te.TestInterface, b.fromWallet)

	inputPrivateKey, _ := b.fromWallet.KeyPair()

	return iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: b.fromWallet.Address(), Keys: inputPrivateKey})
}

func (b *BlockBuilder) fromWalletOutputs() ([]*utxo.Output, uint64) {
	require.NotNil(b.te.TestInterface, b.fromWallet)
	require.Greaterf(b.te.TestInterface, b.amount, uint64(0), "trying to send a transaction with no value")

	var outputsThatCanBeConsumed []*utxo.Output

	if b.outputToUse != nil {
		// Only use the given output
		outputsThatCanBeConsumed = append(outputsThatCanBeConsumed, b.outputToUse)
	} else {
		if b.fakeInputs {
			// Add a fake output with enough balance to create a valid transaction
			fakeInputID := tpkg.RandOutputID()
			fakeInput := &iotago.BasicOutput{
				Amount: b.amount,
				Conditions: iotago.UnlockConditions{
					&iotago.AddressUnlockCondition{
						Address: b.fromWallet.Address(),
					},
				},
			}
			outputsThatCanBeConsumed = append(outputsThatCanBeConsumed, utxo.CreateOutput(fakeInputID, iotago.EmptyBlockID(), 0, 0, fakeInput))
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

	return outputsThatCanBeConsumed, outputsBalance
}

func (b *BlockBuilder) txBuilderFromWalletSendingOutputs(outputs ...iotago.Output) (txBuilder *builder.TransactionBuilder, consumedInputs []*utxo.Output) {
	require.Greater(b.te.TestInterface, len(outputs), 0)

	txBuilder = builder.NewTransactionBuilder(b.te.protoParams.NetworkID())

	fromAddr := b.fromWallet.Address()

	outputsThatCanBeConsumed, _ := b.fromWalletOutputs()

	var consumedAmount uint64
	for _, output := range outputsThatCanBeConsumed {
		txBuilder.AddInput(&builder.TxInput{UnlockTarget: fromAddr, InputID: output.OutputID(), Input: output.Output()})
		consumedInputs = append(consumedInputs, output)
		consumedAmount += output.Deposit()

		if consumedAmount >= b.amount {
			break
		}
	}

	for _, output := range outputs {
		txBuilder.AddOutput(output)
	}

	if b.amount < consumedAmount {
		// Send remainder back to fromWallet
		remainderAmount := consumedAmount - b.amount
		remainderOutput := &iotago.BasicOutput{Conditions: iotago.UnlockConditions{&iotago.AddressUnlockCondition{Address: fromAddr}}, Amount: remainderAmount}
		txBuilder.AddOutput(remainderOutput)
	}

	return
}

func (b *BlockBuilder) BuildTaggedData() *Block {

	require.NotEmpty(b.te.TestInterface, b.tag)

	iotaBlock, err := builder.NewBlockBuilder().
		ProtocolVersion(b.te.protoParams.Version).
		Parents(b.parents).
		Payload(&iotago.TaggedData{Tag: []byte(b.tag), Data: b.tagData}).
		Build()
	require.NoError(b.te.TestInterface, err)

	_, err = b.te.PoWHandler.DoPoW(context.Background(), iotaBlock, b.te.protoParams.MinPoWScore, 1, nil)
	require.NoError(b.te.TestInterface, err)

	block, err := storage.NewBlock(iotaBlock, serializer.DeSeriModePerformValidation, b.te.protoParams)
	require.NoError(b.te.TestInterface, err)

	return &Block{
		builder: b,
		block:   block,
	}
}

func (b *BlockBuilder) BuildTransactionSendingOutputsAndCalculateRemainder(outputs ...iotago.Output) *Block {
	txBuilder, consumedInputs := b.txBuilderFromWalletSendingOutputs(outputs...)

	return b.buildTransactionWithBuilderAndSigned(txBuilder, consumedInputs, b.fromWalletSigner())
}

func (b *BlockBuilder) BuildTransactionWithInputsAndOutputs(consumedInputs utxo.Outputs, outputs iotago.Outputs, signingWallets []*utils.HDWallet) *Block {

	walletKeys := make([]iotago.AddressKeys, len(signingWallets))
	for i, wallet := range signingWallets {
		inputPrivateKey, _ := wallet.KeyPair()
		walletKeys[i] = iotago.AddressKeys{Address: wallet.Address(), Keys: inputPrivateKey}
	}

	txBuilder := builder.NewTransactionBuilder(b.te.protoParams.NetworkID())
	for _, input := range consumedInputs {
		switch input.OutputType() {
		case iotago.OutputFoundry:
			// For foundries we need to unlock the alias
			txBuilder.AddInput(&builder.TxInput{UnlockTarget: input.Output().UnlockConditionSet().ImmutableAlias().Address, InputID: input.OutputID(), Input: input.Output()})
		case iotago.OutputAlias:
			// For alias we need to unlock the state controller
			txBuilder.AddInput(&builder.TxInput{UnlockTarget: input.Output().UnlockConditionSet().StateControllerAddress().Address, InputID: input.OutputID(), Input: input.Output()})
		default:
			txBuilder.AddInput(&builder.TxInput{UnlockTarget: input.Output().UnlockConditionSet().Address().Address, InputID: input.OutputID(), Input: input.Output()})
		}
	}

	for _, output := range outputs {
		txBuilder.AddOutput(output)
	}

	return b.buildTransactionWithBuilderAndSigned(txBuilder, consumedInputs, iotago.NewInMemoryAddressSigner(walletKeys...))
}

func (b *BlockBuilder) buildTransactionWithBuilderAndSigned(txBuilder *builder.TransactionBuilder, consumedInputs utxo.Outputs, signer iotago.AddressSigner) *Block {
	if len(b.tag) > 0 {
		txBuilder.AddTaggedDataPayload(&iotago.TaggedData{Tag: []byte(b.tag), Data: b.tagData})
	}

	require.NotNil(b.te.TestInterface, b.parents)

	iotaBlock, err := txBuilder.BuildAndSwapToBlockBuilder(b.te.protoParams, signer, nil).
		Parents(b.parents).
		ProofOfWork(context.Background(), b.te.protoParams, float64(b.te.protoParams.MinPoWScore)).
		Build()
	require.NoError(b.te.TestInterface, err)

	block, err := storage.NewBlock(iotaBlock, serializer.DeSeriModePerformValidation, b.te.protoParams)
	require.NoError(b.te.TestInterface, err)

	jsonBlockBytes, err := json.MarshalIndent(block.Block(), "", "   ")
	require.NoError(b.te.TestInterface, err)
	println(string(jsonBlockBytes))

	var sentUTXO utxo.Outputs

	// Book the outputs in the wallets
	blockTx := block.Transaction()
	txEssence := blockTx.Essence
	for i := range txEssence.Outputs {
		utxoOutput, err := utxo.NewOutput(block.BlockID(), b.te.LastMilestoneIndex()+1, 0, blockTx, uint16(i))
		require.NoError(b.te.TestInterface, err)
		sentUTXO = append(sentUTXO, utxoOutput)
	}

	return &Block{
		builder:         b,
		block:           block,
		consumedOutputs: consumedInputs,
		createdOutputs:  sentUTXO,
	}
}

func (b *BlockBuilder) BuildAlias() *Block {
	require.NotNil(b.te.TestInterface, b.fromWallet)
	fromAddress := b.fromWallet.Address()

	aliasOutput := &iotago.AliasOutput{
		Amount:         0,
		NativeTokens:   nil,
		AliasID:        iotago.AliasID{},
		StateIndex:     0,
		StateMetadata:  nil,
		FoundryCounter: 0,
		Conditions: iotago.UnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: fromAddress},
			&iotago.GovernorAddressUnlockCondition{Address: fromAddress},
		},
		Features: nil,
		ImmutableFeatures: iotago.Features{
			&iotago.IssuerFeature{Address: fromAddress},
		},
	}

	if b.amount == 0 {
		b.amount = b.te.protoParams.RentStructure.MinRent(aliasOutput)
	}
	aliasOutput.Amount = b.amount

	return b.BuildTransactionSendingOutputsAndCalculateRemainder(aliasOutput)
}

func (b *BlockBuilder) BuildFoundryOnAlias(aliasOutput *utxo.Output) *Block {
	require.NotNil(b.te.TestInterface, b.fromWallet)

	newAlias := aliasOutput.Output().Clone().(*iotago.AliasOutput)
	if newAlias.AliasID.Empty() {
		newAlias.AliasID = iotago.AliasIDFromOutputID(aliasOutput.OutputID())
	}

	newAlias.StateIndex++
	newAlias.FoundryCounter++

	foundry := &iotago.FoundryOutput{
		Amount:       0,
		NativeTokens: nil,
		SerialNumber: newAlias.FoundryCounter,
		TokenScheme: &iotago.SimpleTokenScheme{
			MintedTokens:  big.NewInt(0),
			MeltedTokens:  big.NewInt(0),
			MaximumSupply: big.NewInt(1000),
		},
		Conditions: iotago.UnlockConditions{
			&iotago.ImmutableAliasUnlockCondition{Address: newAlias.AliasID.ToAddress().(*iotago.AliasAddress)},
		},
		Features:          nil,
		ImmutableFeatures: nil,
	}

	if b.amount == 0 {
		b.amount = b.te.protoParams.RentStructure.MinRent(foundry)
	}

	foundry.Amount = b.amount
	newAlias.Amount -= b.amount

	return b.BuildTransactionWithInputsAndOutputs(utxo.Outputs{aliasOutput}, iotago.Outputs{foundry, newAlias}, []*utils.HDWallet{b.fromWallet})
}

func (b *BlockBuilder) BuildTransactionToWallet(wallet *utils.HDWallet) *Block {
	require.Nil(b.te.TestInterface, b.toWallet)
	b.toWallet = wallet
	output := &iotago.BasicOutput{Conditions: iotago.UnlockConditions{&iotago.AddressUnlockCondition{Address: b.toWallet.Address()}}, Amount: b.amount}

	return b.BuildTransactionSendingOutputsAndCalculateRemainder(output)
}

func (m *Block) Store() *Block {
	require.True(m.builder.te.TestInterface, m.storedBlockID.Empty())
	m.storedBlockID = m.builder.te.StoreBlock(m.block).Block().BlockID()

	return m
}

func (m *Block) BookOnWallets() *Block {
	require.False(m.builder.te.TestInterface, m.booked)
	m.builder.fromWallet.BookSpents(m.consumedOutputs)

	if m.builder.toWallet != nil {
		// Also book it in the toWallet because both addresses can have part ownership of the output.
		// Note: if there is a third wallet involved this will not catch it, and the third wallet will still hold a reference to it
		m.builder.toWallet.BookSpents(m.consumedOutputs)
	}

	for _, sentOutput := range m.createdOutputs {
		// Check if we should book the output to the toWallet or to the fromWallet
		switch output := sentOutput.Output().(type) {
		case *iotago.BasicOutput, *iotago.NFTOutput:
			if m.builder.toWallet != nil {
				if output.UnlockConditionSet().Address().Address.Equal(m.builder.toWallet.Address()) {
					m.builder.toWallet.BookOutput(sentOutput)

					continue
				}
			}
			// Note: we don't care about SDRUC here right now
			m.builder.fromWallet.BookOutput(sentOutput)

		case *iotago.AliasOutput:
			if m.builder.toWallet != nil {
				if output.UnlockConditionSet().GovernorAddress().Address.Equal(m.builder.toWallet.Address()) ||
					output.UnlockConditionSet().StateControllerAddress().Address.Equal(m.builder.toWallet.Address()) {
					m.builder.toWallet.BookOutput(sentOutput)
				}
			}
			if output.UnlockConditionSet().GovernorAddress().Address.Equal(m.builder.fromWallet.Address()) ||
				output.UnlockConditionSet().StateControllerAddress().Address.Equal(m.builder.fromWallet.Address()) {
				m.builder.fromWallet.BookOutput(sentOutput)
			}

		case *iotago.FoundryOutput:
			// We always book the foundry to the controlling wallet here, since everything else is too complex for the testsuite
			m.builder.fromWallet.BookOutput(sentOutput)
		}
	}
	m.booked = true

	return m
}

func (m *Block) GeneratedUTXO() *utxo.Output {
	require.Greater(m.builder.te.TestInterface, len(m.createdOutputs), 0)

	return m.createdOutputs[0]
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
