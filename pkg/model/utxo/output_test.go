package utxo

import (
	"encoding/binary"
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

func EqualOutput(t *testing.T, expected *Output, actual *Output) {
	require.Equal(t, expected.OutputID()[:], actual.OutputID()[:])
	require.Equal(t, expected.MessageID()[:], actual.MessageID()[:])
	require.Equal(t, expected.OutputType(), actual.OutputType())
	require.Equal(t, expected.Address().String(), actual.Address().String())
	require.Equal(t, expected.Amount(), actual.Amount())
	require.Equal(t, expected.Output(), actual.Output())
}

func EqualSpent(t *testing.T, expected *Spent, actual *Spent) {
	require.Equal(t, expected.outputID[:], actual.outputID[:])
	require.Equal(t, expected.TargetTransactionID()[:], actual.TargetTransactionID()[:])
	require.Equal(t, expected.ConfirmationIndex(), actual.ConfirmationIndex())
	EqualOutput(t, expected.Output(), actual.Output())
}

func AssertOutputUnspentAndSpentTransitions(t *testing.T, output *Output, spent *Spent) {

	outputID := output.OutputID()
	manager := New(mapdb.NewMapDB())

	require.NoError(t, manager.AddUnspentOutput(output))

	// Read Output from DB and compare
	readOutput, err := manager.ReadOutputByOutputID(outputID)
	require.NoError(t, err)
	EqualOutput(t, output, readOutput)

	// Verify that it is unspent
	unspent, err := manager.IsOutputUnspent(outputID)
	require.NoError(t, err)
	require.True(t, unspent)

	// Verify that all unspent keys exist in the database
	for _, key := range output.unspentLookupKeys() {
		has, err := manager.utxoStorage.Has(key)
		require.NoError(t, err)
		require.True(t, has)
	}

	// Verify that no spent keys exist in the database
	for _, key := range output.spentLookupKeys() {
		has, err := manager.utxoStorage.Has(key)
		require.NoError(t, err)
		require.False(t, has)
	}

	// Spend it with a milestone
	require.NoError(t, manager.ApplyConfirmation(spent.confirmationIndex, Outputs{}, Spents{spent}, nil, nil))

	// Read Spent from DB and compare
	readSpent, err := manager.readSpentForOutputIDWithoutLocking(outputID)
	require.NoError(t, err)
	EqualSpent(t, spent, readSpent)

	// Verify that it is spent
	unspent, err = manager.IsOutputUnspent(outputID)
	require.NoError(t, err)
	require.False(t, unspent)

	// Verify that no unspent keys exist in the database
	for _, key := range output.unspentLookupKeys() {
		has, err := manager.utxoStorage.Has(key)
		require.NoError(t, err)
		require.False(t, has)
	}

	// Verify that all spent keys exist in the database
	for _, key := range output.spentLookupKeys() {
		has, err := manager.utxoStorage.Has(key)
		require.NoError(t, err)
		require.True(t, has)
	}

	// Rollback milestone
	require.NoError(t, manager.RollbackConfirmation(spent.confirmationIndex, Outputs{}, Spents{spent}, nil, nil))

	// Verify that it is unspent
	unspent, err = manager.IsOutputUnspent(outputID)
	require.NoError(t, err)
	require.True(t, unspent)

	// No Spent should be in the DB
	_, err = manager.readSpentForOutputIDWithoutLocking(outputID)
	require.ErrorIs(t, err, kvstore.ErrKeyNotFound)

	// Verify that all unspent keys exist in the database
	for _, key := range output.unspentLookupKeys() {
		has, err := manager.utxoStorage.Has(key)
		require.NoError(t, err)
		require.True(t, has)
	}

	// Verify that no spent keys exist in the database
	for _, key := range output.spentLookupKeys() {
		has, err := manager.utxoStorage.Has(key)
		require.NoError(t, err)
		require.False(t, has)
	}
}

func CreateOutputAndAssertSerialization(t *testing.T, messageID hornet.MessageID, msIndex milestone.Index, outputID *iotago.OutputID, iotaOutput iotago.Output) *Output {
	output := CreateOutput(outputID, messageID, msIndex, iotaOutput)
	outputBytes, err := output.Output().Serialize(serializer.DeSeriModeNoValidation, nil)
	require.NoError(t, err)

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, outputID[:]), output.kvStorableKey())

	value := output.kvStorableValue()
	require.Equal(t, messageID, hornet.MessageIDFromSlice(value[:32]))
	require.Equal(t, uint32(msIndex), binary.LittleEndian.Uint32(value[32:36]))
	require.Equal(t, outputBytes, value[36:])

	return output
}

func CreateSpentAndAssertSerialization(t *testing.T, output *Output) *Spent {
	transactionID := &iotago.TransactionID{}
	copy(transactionID[:], testsuite.RandBytes(iotago.TransactionIDLength))

	confirmationIndex := milestone.Index(6788362)

	spent := NewSpent(output, transactionID, confirmationIndex)

	require.Equal(t, output, spent.Output())

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutputSpent}, output.OutputID()[:]), spent.kvStorableKey())

	value := spent.kvStorableValue()
	require.Equal(t, transactionID[:], value[:32])
	require.Equal(t, confirmationIndex, milestone.Index(binary.LittleEndian.Uint32(value[32:36])))

	return spent
}

func TestExtendedOutputOnEd25519WithoutSpendConstraintsSerialization(t *testing.T) {

	outputID := testsuite.RandOutputID()
	messageID := testsuite.RandMessageID()
	address := testsuite.RandAddress(iotago.AddressEd25519).(*iotago.Ed25519Address)
	senderAddress := testsuite.RandAddress(iotago.AddressEd25519).(*iotago.Ed25519Address)
	tag := testsuite.RandBytes(23)
	amount := rand.Uint64()
	msIndex := testsuite.RandMilestoneIndex()

	iotaOutput := &iotago.ExtendedOutput{
		Address: address,
		Amount:  amount,
		Blocks: iotago.FeatureBlocks{
			&iotago.SenderFeatureBlock{
				Address: senderAddress,
			},
			&iotago.IndexationFeatureBlock{
				Tag: tag,
			},
		},
	}

	output := CreateOutputAndAssertSerialization(t, messageID, msIndex, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	featureBlockKeys := []lookupKey{
		byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSenderLookup}, []byte{iotago.AddressEd25519}, senderAddress[:], []byte{byte(iotago.OutputExtended)}, outputID[:]),
		byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSenderAndIndexLookup}, []byte{iotago.AddressEd25519}, senderAddress[:], tag, make([]byte, 41), []byte{byte(iotago.OutputExtended)}, outputID[:]),
	}
	require.ElementsMatch(t, featureBlockKeys, output.featureLookupKeys())

	addressLookupKeys := []lookupKey{
		byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixAddressLookup}, []byte{iotago.AddressEd25519}, address[:], []byte{0}, []byte{byte(iotago.OutputExtended)}, outputID[:]),
	}
	require.ElementsMatch(t, addressLookupKeys, output.addressLookupKeys())

	var expectedSpentKeys []lookupKey
	expectedSpentKeys = append(expectedSpentKeys, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixExtendedOutputSpent}, []byte{iotago.AddressEd25519}, address[:], outputID[:]))
	require.ElementsMatch(t, expectedSpentKeys, output.spentLookupKeys())

	var expectedUnspentKeys []lookupKey
	expectedUnspentKeys = append(expectedUnspentKeys, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixExtendedOutputUnspent}, []byte{iotago.AddressEd25519}, address[:], outputID[:]))
	require.ElementsMatch(t, append(append(expectedUnspentKeys, addressLookupKeys...), featureBlockKeys...), output.unspentLookupKeys())
	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}

func TestExtendedOutputOnEd25519WithSpendConstraintsSerialization(t *testing.T) {

	outputID := testsuite.RandOutputID()
	messageID := testsuite.RandMessageID()
	address := testsuite.RandAddress(iotago.AddressEd25519).(*iotago.Ed25519Address)
	senderAddress := testsuite.RandAddress(iotago.AddressEd25519).(*iotago.Ed25519Address)
	amount := rand.Uint64()
	msIndex := testsuite.RandMilestoneIndex()

	iotaOutput := &iotago.ExtendedOutput{
		Address: address,
		Amount:  amount,
		Blocks: iotago.FeatureBlocks{
			&iotago.TimelockMilestoneIndexFeatureBlock{
				MilestoneIndex: 234,
			},
			&iotago.SenderFeatureBlock{
				Address: senderAddress,
			},
		},
	}

	output := CreateOutputAndAssertSerialization(t, messageID, msIndex, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	featureBlockKeys := []lookupKey{
		byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSenderLookup}, []byte{iotago.AddressEd25519}, senderAddress[:], []byte{byte(iotago.OutputExtended)}, outputID[:]),
	}
	require.ElementsMatch(t, featureBlockKeys, output.featureLookupKeys())

	addressLookupKeys := []lookupKey{
		byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixAddressLookup}, []byte{iotago.AddressEd25519}, address[:], []byte{1}, []byte{byte(iotago.OutputExtended)}, outputID[:]),
	}
	require.ElementsMatch(t, addressLookupKeys, output.addressLookupKeys())

	var expectedSpentKeys []lookupKey
	expectedSpentKeys = append(expectedSpentKeys, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixExtendedOutputSpent}, []byte{iotago.AddressEd25519}, address[:], outputID[:]))
	require.ElementsMatch(t, expectedSpentKeys, output.spentLookupKeys())

	var expectedUnspentKeys []lookupKey
	expectedUnspentKeys = append(expectedUnspentKeys, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixExtendedOutputUnspent}, []byte{iotago.AddressEd25519}, address[:], outputID[:]))
	require.ElementsMatch(t, append(append(expectedUnspentKeys, addressLookupKeys...), featureBlockKeys...), output.unspentLookupKeys())

	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}

func TestNFTOutputSerialization(t *testing.T) {

	outputID := testsuite.RandOutputID()
	messageID := testsuite.RandMessageID()
	address := testsuite.RandAddress(iotago.AddressEd25519).(*iotago.Ed25519Address)
	nftID := testsuite.RandNFTID()
	amount := rand.Uint64()
	msIndex := testsuite.RandMilestoneIndex()

	iotaOutput := &iotago.NFTOutput{
		Address:           address,
		Amount:            amount,
		NFTID:             nftID,
		ImmutableMetadata: testsuite.RandBytes(12),
	}

	output := CreateOutputAndAssertSerialization(t, messageID, msIndex, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	featureBlockKeys := []lookupKey{}
	require.ElementsMatch(t, featureBlockKeys, output.featureLookupKeys())

	addressLookupKeys := []lookupKey{
		byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixAddressLookup}, []byte{iotago.AddressEd25519}, address[:], []byte{0}, []byte{byte(iotago.OutputNFT)}, outputID[:]),
	}
	require.ElementsMatch(t, addressLookupKeys, output.addressLookupKeys())

	var expectedSpentKeys []lookupKey
	expectedSpentKeys = append(expectedSpentKeys, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixNFTSpent}, nftID[:], outputID[:]))
	require.ElementsMatch(t, expectedSpentKeys, output.spentLookupKeys())

	var expectedUnspentKeys []lookupKey
	expectedUnspentKeys = append(expectedUnspentKeys, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixNFTUnspent}, nftID[:], outputID[:]))
	require.ElementsMatch(t, append(append(expectedUnspentKeys, addressLookupKeys...), featureBlockKeys...), output.unspentLookupKeys())

	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}

func TestNFTOutputWithSpendConstraintsSerialization(t *testing.T) {

	outputID := testsuite.RandOutputID()
	messageID := testsuite.RandMessageID()
	address := testsuite.RandNFTID()
	issuerAddress := testsuite.RandAddress(iotago.AddressEd25519).(*iotago.Ed25519Address)
	nftID := testsuite.RandNFTID()
	amount := rand.Uint64()
	msIndex := testsuite.RandMilestoneIndex()

	iotaOutput := &iotago.NFTOutput{
		Address:           address.ToAddress(),
		Amount:            amount,
		NFTID:             nftID,
		ImmutableMetadata: testsuite.RandBytes(12),
		Blocks: iotago.FeatureBlocks{
			&iotago.IssuerFeatureBlock{
				Address: issuerAddress,
			},
			&iotago.ExpirationMilestoneIndexFeatureBlock{
				MilestoneIndex: 324324,
			},
		},
	}

	output := CreateOutputAndAssertSerialization(t, messageID, msIndex, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	featureBlockKeys := []lookupKey{
		byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixIssuerLookup}, []byte{iotago.AddressEd25519}, issuerAddress[:], []byte{byte(iotago.OutputNFT)}, outputID[:]),
	}
	require.ElementsMatch(t, featureBlockKeys, output.featureLookupKeys())

	addressLookupKeys := []lookupKey{
		byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixAddressLookup}, []byte{iotago.AddressNFT}, address[:], []byte{1}, []byte{byte(iotago.OutputNFT)}, outputID[:]),
	}
	require.ElementsMatch(t, addressLookupKeys, output.addressLookupKeys())

	var expectedSpentKeys []lookupKey
	expectedSpentKeys = append(expectedSpentKeys, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixNFTSpent}, nftID[:], outputID[:]))
	require.ElementsMatch(t, expectedSpentKeys, output.spentLookupKeys())

	var expectedUnspentKeys []lookupKey
	expectedUnspentKeys = append(expectedUnspentKeys, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixNFTUnspent}, nftID[:], outputID[:]))
	require.ElementsMatch(t, append(append(expectedUnspentKeys, addressLookupKeys...), featureBlockKeys...), output.unspentLookupKeys())

	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}

func TestAliasOutputSerialization(t *testing.T) {

	outputID := testsuite.RandOutputID()
	messageID := testsuite.RandMessageID()
	aliasID := testsuite.RandAliasID()
	stateController := testsuite.RandAliasID()
	governanceController := testsuite.RandAddress(iotago.AddressEd25519).(*iotago.Ed25519Address)
	issuer := testsuite.RandNFTID()
	sender := testsuite.RandAliasID()
	amount := rand.Uint64()
	msIndex := testsuite.RandMilestoneIndex()

	iotaOutput := &iotago.AliasOutput{
		Amount:               amount,
		AliasID:              aliasID,
		StateController:      stateController.ToAddress(),
		GovernanceController: governanceController,
		StateMetadata:        []byte{},
		Blocks: iotago.FeatureBlocks{
			&iotago.IssuerFeatureBlock{
				Address: issuer.ToAddress(),
			},
			&iotago.SenderFeatureBlock{
				Address: sender.ToAddress(),
			},
		},
	}

	output := CreateOutputAndAssertSerialization(t, messageID, msIndex, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	featureBlockKeys := []lookupKey{
		byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixIssuerLookup}, []byte{iotago.AddressNFT}, issuer[:], []byte{byte(iotago.OutputAlias)}, outputID[:]),
		byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSenderLookup}, []byte{iotago.AddressAlias}, sender[:], []byte{byte(iotago.OutputAlias)}, outputID[:]),
	}
	require.ElementsMatch(t, featureBlockKeys, output.featureLookupKeys())

	addressLookupKeys := []lookupKey{
		byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixAddressLookup}, []byte{iotago.AddressAlias}, stateController[:], []byte{0}, []byte{byte(iotago.OutputAlias)}, outputID[:]),
		byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixAddressLookup}, []byte{iotago.AddressEd25519}, governanceController[:], []byte{0}, []byte{byte(iotago.OutputAlias)}, outputID[:]),
	}
	require.ElementsMatch(t, addressLookupKeys, output.addressLookupKeys())

	var expectedSpentKeys []lookupKey
	expectedSpentKeys = append(expectedSpentKeys, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixAliasSpent}, aliasID[:], outputID[:]))
	require.ElementsMatch(t, expectedSpentKeys, output.spentLookupKeys())

	var expectedUnspentKeys []lookupKey
	expectedUnspentKeys = append(expectedUnspentKeys, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixAliasUnspent}, aliasID[:], outputID[:]))
	require.ElementsMatch(t, append(append(expectedUnspentKeys, addressLookupKeys...), featureBlockKeys...), output.unspentLookupKeys())

	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}

func TestFoundryOutputSerialization(t *testing.T) {

	outputID := testsuite.RandOutputID()
	messageID := testsuite.RandMessageID()
	aliasID := testsuite.RandAliasID()
	amount := rand.Uint64()
	msIndex := testsuite.RandMilestoneIndex()
	supply := new(big.Int).SetUint64(rand.Uint64())

	iotaOutput := &iotago.FoundryOutput{
		Address:           aliasID.ToAddress(),
		Amount:            amount,
		SerialNumber:      rand.Uint32(),
		TokenTag:          testsuite.RandTokenTag(),
		CirculatingSupply: supply,
		MaximumSupply:     supply,
		TokenScheme:       &iotago.SimpleTokenScheme{},
	}

	output := CreateOutputAndAssertSerialization(t, messageID, msIndex, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	foundryID, err := iotaOutput.ID()
	require.NoError(t, err)

	featureBlockKeys := []lookupKey{}
	require.ElementsMatch(t, featureBlockKeys, output.featureLookupKeys())

	addressLookupKeys := []lookupKey{
		byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixAddressLookup}, []byte{iotago.AddressAlias}, aliasID[:], []byte{0}, []byte{byte(iotago.OutputFoundry)}, outputID[:]),
	}
	require.ElementsMatch(t, addressLookupKeys, output.addressLookupKeys())

	var expectedSpentKeys []lookupKey
	expectedSpentKeys = append(expectedSpentKeys, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixFoundrySpent}, foundryID[:], outputID[:]))
	require.ElementsMatch(t, expectedSpentKeys, output.spentLookupKeys())

	var expectedUnspentKeys []lookupKey
	expectedUnspentKeys = append(expectedUnspentKeys, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixFoundryUnspent}, foundryID[:], outputID[:]))
	require.ElementsMatch(t, append(append(expectedUnspentKeys, addressLookupKeys...), featureBlockKeys...), output.unspentLookupKeys())

	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}
