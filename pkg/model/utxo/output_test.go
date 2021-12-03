package utxo

import (
	"encoding/binary"
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

// returns length amount random bytes
func randBytes(length int) []byte {
	var b []byte
	for i := 0; i < length; i++ {
		b = append(b, byte(rand.Intn(256)))
	}
	return b
}

func randMessageID() hornet.MessageID {
	return randBytes(iotago.MessageIDLength)
}

func randNFTID() iotago.NFTID {
	nft := iotago.NFTID{}
	copy(nft[:], randBytes(iotago.NFTIDLength))
	return nft
}

func randAliasID() iotago.AliasID {
	alias := iotago.AliasID{}
	copy(alias[:], randBytes(iotago.AliasIDLength))
	return alias
}

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

	// Read Output from DB and compate
	readOutput, err := manager.ReadOutputByOutputID(outputID)
	require.NoError(t, err)
	EqualOutput(t, output, readOutput)

	// Verify that it is unspent
	unspent, err := manager.IsOutputUnspent(outputID)
	require.NoError(t, err)
	require.True(t, unspent)

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

	// Rollback milestone
	require.NoError(t, manager.RollbackConfirmation(spent.confirmationIndex, Outputs{}, Spents{spent}, nil, nil))

	// Verify that it is unspent
	unspent, err = manager.IsOutputUnspent(outputID)
	require.NoError(t, err)
	require.True(t, unspent)

	// No Spent should be in the DB
	_, err = manager.readSpentForOutputIDWithoutLocking(outputID)
	require.ErrorIs(t, err, kvstore.ErrKeyNotFound)
}

func CreateOutputAndAssertSerialization(t *testing.T, messageID hornet.MessageID, outputID *iotago.OutputID, iotaOutput iotago.Output) *Output {
	output := CreateOutput(outputID, messageID, iotaOutput)
	outputBytes, err := output.Output().Serialize(serializer.DeSeriModeNoValidation, nil)
	require.NoError(t, err)

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, outputID[:]), output.kvStorableKey())

	value := output.kvStorableValue()
	require.Equal(t, messageID, hornet.MessageIDFromSlice(value[:32]))
	require.Equal(t, outputBytes, value[32:])

	return output
}

func CreateSpentAndAssertSerialization(t *testing.T, output *Output) *Spent {
	transactionID := &iotago.TransactionID{}
	copy(transactionID[:], randBytes(iotago.TransactionIDLength))

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

	outputID := randOutputID()
	messageID := randMessageID()
	address := randomAddress()
	amount := uint64(832493)

	iotaOutput := &iotago.ExtendedOutput{
		Address: address,
		Amount:  amount,
	}

	output := CreateOutputAndAssertSerialization(t, messageID, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutputOnAddressSpent}, []byte{iotago.AddressEd25519}, address[:], []byte{0}, []byte{byte(output.OutputType())}, outputID[:]), []byte(output.spentDatabaseKeys()[0]))
	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutputOnAddressUnspent}, []byte{iotago.AddressEd25519}, address[:], []byte{0}, []byte{byte(output.OutputType())}, outputID[:]), []byte(output.unspentDatabaseKeys()[0]))

	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}

func TestExtendedOutputOnEd25519WithSpendConstraintsSerialization(t *testing.T) {

	outputID := randOutputID()
	messageID := randMessageID()
	address := randomAddress()
	amount := uint64(832493)

	iotaOutput := &iotago.ExtendedOutput{
		Address: address,
		Amount:  amount,
		Blocks: iotago.FeatureBlocks{
			&iotago.TimelockMilestoneIndexFeatureBlock{
				MilestoneIndex: 234,
			},
		},
	}

	output := CreateOutputAndAssertSerialization(t, messageID, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutputOnAddressSpent}, []byte{iotago.AddressEd25519}, address[:], []byte{1}, []byte{byte(output.OutputType())}, outputID[:]), []byte(output.spentDatabaseKeys()[0]))
	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutputOnAddressUnspent}, []byte{iotago.AddressEd25519}, address[:], []byte{1}, []byte{byte(output.OutputType())}, outputID[:]), []byte(output.unspentDatabaseKeys()[0]))

	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}

func TestNFTOutputSerialization(t *testing.T) {

	outputID := randOutputID()
	messageID := randMessageID()
	address := randomAddress()
	nftID := randNFTID()
	amount := uint64(832493)

	iotaOutput := &iotago.NFTOutput{
		Address:           address,
		Amount:            amount,
		NFTID:             nftID,
		ImmutableMetadata: randBytes(12),
	}

	output := CreateOutputAndAssertSerialization(t, messageID, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixNFTSpent}, nftID[:], outputID[:]), []byte(output.spentDatabaseKeys()[0]))
	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixNFTUnspent}, nftID[:], outputID[:]), []byte(output.unspentDatabaseKeys()[0]))

	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}

func TestAliasOutputSerialization(t *testing.T) {

	outputID := randOutputID()
	messageID := randMessageID()
	aliasID := randAliasID()
	amount := uint64(832493)

	iotaOutput := &iotago.AliasOutput{
		Amount:               amount,
		AliasID:              aliasID,
		StateController:      randomAddress(),
		GovernanceController: randomAddress(),
		StateMetadata:        []byte{},
	}

	output := CreateOutputAndAssertSerialization(t, messageID, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixAliasSpent}, aliasID[:], outputID[:]), []byte(output.spentDatabaseKeys()[0]))
	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixAliasUnspent}, aliasID[:], outputID[:]), []byte(output.unspentDatabaseKeys()[0]))

	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}

func TestFoundryOutputSerialization(t *testing.T) {

	outputID := randOutputID()
	messageID := randMessageID()
	aliasID := randAliasID()
	amount := uint64(832493)

	serialNumber := rand.Uint32()
	tokenTag := iotago.TokenTag{}
	copy(tokenTag[:], randBytes(iotago.TokenTagLength))

	supply := new(big.Int).SetUint64(rand.Uint64())

	iotaOutput := &iotago.FoundryOutput{
		Address:           aliasID.ToAddress(),
		Amount:            amount,
		SerialNumber:      serialNumber,
		TokenTag:          tokenTag,
		CirculatingSupply: supply,
		MaximumSupply:     supply,
		TokenScheme:       &iotago.SimpleTokenScheme{},
	}

	output := CreateOutputAndAssertSerialization(t, messageID, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	foundryID, err := iotaOutput.ID()
	require.NoError(t, err)

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixFoundrySpent}, foundryID[:], outputID[:]), []byte(output.spentDatabaseKeys()[0]))
	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixFoundryUnspent}, foundryID[:], outputID[:]), []byte(output.unspentDatabaseKeys()[0]))

	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}
