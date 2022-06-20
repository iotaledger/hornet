package utxo

import (
	"encoding/binary"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/utxo/utils"
	iotago "github.com/iotaledger/iota.go/v3"
)

func RandUTXOOutput(outputType iotago.OutputType) *Output {
	return CreateOutput(utils.RandOutputID(), utils.RandBlockID(), utils.RandMilestoneIndex(), rand.Uint32(), utils.RandOutput(outputType))
}

func RandUTXOOutputOnAddress(outputType iotago.OutputType, address iotago.Address) *Output {
	return CreateOutput(utils.RandOutputID(), utils.RandBlockID(), utils.RandMilestoneIndex(), rand.Uint32(), utils.RandOutputOnAddress(outputType, address))
}

func RandUTXOOutputOnAddressWithAmount(outputType iotago.OutputType, address iotago.Address, amount uint64) *Output {
	return CreateOutput(utils.RandOutputID(), utils.RandBlockID(), utils.RandMilestoneIndex(), rand.Uint32(), utils.RandOutputOnAddressWithAmount(outputType, address, amount))
}

func RandUTXOSpent(output *Output, index milestone.Index, timestamp uint32) *Spent {
	return NewSpent(output, utils.RandTransactionID(), index, timestamp)
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
	unspent, err := manager.IsOutputIDUnspentWithoutLocking(outputID)
	require.NoError(t, err)
	require.True(t, unspent)

	// Verify that all lookup keys exist in the database
	has, err := manager.utxoStorage.Has(output.unspentLookupKey())
	require.NoError(t, err)
	require.True(t, has)

	// Spend it with a milestone
	require.NoError(t, manager.ApplyConfirmation(spent.milestoneIndex, Outputs{}, Spents{spent}, nil, nil))

	// Read Spent from DB and compare
	readSpent, err := manager.ReadSpentForOutputIDWithoutLocking(outputID)
	require.NoError(t, err)
	EqualSpent(t, spent, readSpent)

	// Verify that it is spent
	unspent, err = manager.IsOutputIDUnspentWithoutLocking(outputID)
	require.NoError(t, err)
	require.False(t, unspent)

	// Verify that no lookup keys exist in the database
	has, err = manager.utxoStorage.Has(output.unspentLookupKey())
	require.NoError(t, err)
	require.False(t, has)

	// Rollback milestone
	require.NoError(t, manager.RollbackConfirmation(spent.milestoneIndex, Outputs{}, Spents{spent}, nil, nil))

	// Verify that it is unspent
	unspent, err = manager.IsOutputIDUnspentWithoutLocking(outputID)
	require.NoError(t, err)
	require.True(t, unspent)

	// No Spent should be in the DB
	_, err = manager.ReadSpentForOutputIDWithoutLocking(outputID)
	require.ErrorIs(t, err, kvstore.ErrKeyNotFound)

	// Verify that all unspent keys exist in the database
	has, err = manager.utxoStorage.Has(output.unspentLookupKey())
	require.NoError(t, err)
	require.True(t, has)
}

func CreateOutputAndAssertSerialization(t *testing.T, blockID iotago.BlockID, msIndex milestone.Index, msTimestamp uint32, outputID iotago.OutputID, iotaOutput iotago.Output) *Output {
	output := CreateOutput(outputID, blockID, msIndex, msTimestamp, iotaOutput)
	outputBytes, err := output.Output().Serialize(serializer.DeSeriModeNoValidation, nil)
	require.NoError(t, err)

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, outputID[:]), output.kvStorableKey())

	value := output.kvStorableValue()
	require.Equal(t, blockID[:], value[:32])
	require.Equal(t, uint32(msIndex), binary.LittleEndian.Uint32(value[32:36]))
	require.Equal(t, msTimestamp, binary.LittleEndian.Uint32(value[36:40]))
	require.Equal(t, outputBytes, value[40:])

	return output
}

func CreateSpentAndAssertSerialization(t *testing.T, output *Output) *Spent {
	transactionID := iotago.TransactionID{}
	copy(transactionID[:], utils.RandBytes(iotago.TransactionIDLength))

	confirmationIndex := milestone.Index(6788362)
	confirmationTimestamp := rand.Uint32()

	spent := NewSpent(output, transactionID, confirmationIndex, confirmationTimestamp)

	require.Equal(t, output, spent.Output())

	outputID := output.OutputID()
	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutputSpent}, outputID[:]), spent.kvStorableKey())

	value := spent.kvStorableValue()
	require.Equal(t, transactionID[:], value[:32])
	require.Equal(t, confirmationIndex, milestone.Index(binary.LittleEndian.Uint32(value[32:36])))
	require.Equal(t, confirmationTimestamp, binary.LittleEndian.Uint32(value[36:40]))

	return spent
}

func TestExtendedOutputOnEd25519WithoutSpendConstraintsSerialization(t *testing.T) {

	outputID := utils.RandOutputID()
	blockID := utils.RandBlockID()
	address := utils.RandAddress(iotago.AddressEd25519).(*iotago.Ed25519Address)
	senderAddress := utils.RandAddress(iotago.AddressEd25519).(*iotago.Ed25519Address)
	tag := utils.RandBytes(23)
	amount := rand.Uint64()
	msIndex := utils.RandMilestoneIndex()
	msTimestamp := rand.Uint32()

	iotaOutput := &iotago.BasicOutput{
		Amount: amount,
		Features: iotago.Features{
			&iotago.SenderFeature{
				Address: senderAddress,
			},
			&iotago.TagFeature{
				Tag: tag,
			},
		},
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{
				Address: address,
			},
		},
	}

	output := CreateOutputAndAssertSerialization(t, blockID, msIndex, msTimestamp, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	require.ElementsMatch(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutputUnspent}, outputID[:]), output.unspentLookupKey())
	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}

func TestExtendedOutputOnEd25519WithSpendConstraintsSerialization(t *testing.T) {

	outputID := utils.RandOutputID()
	blockID := utils.RandBlockID()
	address := utils.RandAddress(iotago.AddressEd25519).(*iotago.Ed25519Address)
	senderAddress := utils.RandAddress(iotago.AddressEd25519).(*iotago.Ed25519Address)
	amount := rand.Uint64()
	msIndex := utils.RandMilestoneIndex()
	msTimestamp := rand.Uint32()

	iotaOutput := &iotago.BasicOutput{
		Amount: amount,
		Features: iotago.Features{
			&iotago.SenderFeature{
				Address: senderAddress,
			},
		},
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{
				Address: address,
			},
			&iotago.TimelockUnlockCondition{
				UnixTime: uint32(time.Now().Unix()),
			},
		},
	}

	output := CreateOutputAndAssertSerialization(t, blockID, msIndex, msTimestamp, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	require.ElementsMatch(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutputUnspent}, outputID[:]), output.unspentLookupKey())
	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}

func TestNFTOutputSerialization(t *testing.T) {

	outputID := utils.RandOutputID()
	blockID := utils.RandBlockID()
	address := utils.RandAddress(iotago.AddressEd25519).(*iotago.Ed25519Address)
	nftID := utils.RandNFTID()
	amount := rand.Uint64()
	msIndex := utils.RandMilestoneIndex()
	msTimestamp := rand.Uint32()

	iotaOutput := &iotago.NFTOutput{
		Amount: amount,
		NFTID:  nftID,
		ImmutableFeatures: iotago.Features{
			&iotago.MetadataFeature{
				Data: utils.RandBytes(12),
			},
		},
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{
				Address: address,
			},
		},
	}

	output := CreateOutputAndAssertSerialization(t, blockID, msIndex, msTimestamp, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	require.ElementsMatch(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutputUnspent}, outputID[:]), output.unspentLookupKey())
	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}

func TestNFTOutputWithSpendConstraintsSerialization(t *testing.T) {

	outputID := utils.RandOutputID()
	blockID := utils.RandBlockID()
	address := utils.RandNFTID()
	issuerAddress := utils.RandAddress(iotago.AddressEd25519).(*iotago.Ed25519Address)
	nftID := utils.RandNFTID()
	amount := rand.Uint64()
	msIndex := utils.RandMilestoneIndex()
	msTimestamp := rand.Uint32()

	iotaOutput := &iotago.NFTOutput{
		Amount: amount,
		NFTID:  nftID,
		ImmutableFeatures: iotago.Features{
			&iotago.MetadataFeature{
				Data: utils.RandBytes(12),
			},
			&iotago.IssuerFeature{
				Address: issuerAddress,
			},
		},
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{
				Address: address.ToAddress(),
			},
			&iotago.ExpirationUnlockCondition{
				UnixTime:      uint32(time.Now().Unix()),
				ReturnAddress: issuerAddress,
			},
		},
	}

	output := CreateOutputAndAssertSerialization(t, blockID, msIndex, msTimestamp, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	require.ElementsMatch(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutputUnspent}, outputID[:]), output.unspentLookupKey())
	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}

func TestAliasOutputSerialization(t *testing.T) {

	outputID := utils.RandOutputID()
	blockID := utils.RandBlockID()
	aliasID := utils.RandAliasID()
	stateController := utils.RandAliasID()
	governor := utils.RandAddress(iotago.AddressEd25519).(*iotago.Ed25519Address)
	issuer := utils.RandNFTID()
	sender := utils.RandAliasID()
	amount := rand.Uint64()
	msIndex := utils.RandMilestoneIndex()
	msTimestamp := rand.Uint32()

	iotaOutput := &iotago.AliasOutput{
		Amount:  amount,
		AliasID: aliasID,
		Features: iotago.Features{
			&iotago.SenderFeature{
				Address: sender.ToAddress(),
			},
		},
		ImmutableFeatures: iotago.Features{
			&iotago.IssuerFeature{
				Address: issuer.ToAddress(),
			},
		},
		Conditions: iotago.UnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{
				Address: stateController.ToAddress(),
			},
			&iotago.GovernorAddressUnlockCondition{
				Address: governor,
			},
		},
	}

	output := CreateOutputAndAssertSerialization(t, blockID, msIndex, msTimestamp, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	require.ElementsMatch(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutputUnspent}, outputID[:]), output.unspentLookupKey())
	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}

func TestFoundryOutputSerialization(t *testing.T) {

	outputID := utils.RandOutputID()
	blockID := utils.RandBlockID()
	aliasID := utils.RandAliasID()
	amount := rand.Uint64()
	msIndex := utils.RandMilestoneIndex()
	msTimestamp := rand.Uint32()
	supply := new(big.Int).SetUint64(rand.Uint64())

	iotaOutput := &iotago.FoundryOutput{
		Amount:       amount,
		SerialNumber: rand.Uint32(),
		TokenScheme: &iotago.SimpleTokenScheme{
			MintedTokens:  supply,
			MeltedTokens:  new(big.Int).SetBytes([]byte{0}),
			MaximumSupply: supply,
		},
		Conditions: iotago.UnlockConditions{
			&iotago.ImmutableAliasUnlockCondition{
				Address: aliasID.ToAddress().(*iotago.AliasAddress),
			},
		},
	}

	output := CreateOutputAndAssertSerialization(t, blockID, msIndex, msTimestamp, outputID, iotaOutput)
	spent := CreateSpentAndAssertSerialization(t, output)

	require.ElementsMatch(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutputUnspent}, outputID[:]), output.unspentLookupKey())
	AssertOutputUnspentAndSpentTransitions(t, output, spent)
}
