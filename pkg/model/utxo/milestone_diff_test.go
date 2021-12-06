package utxo

import (
	"encoding/binary"
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/testutil"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestSimpleMilestoneDiffSerialization(t *testing.T) {
	confirmationIndex := milestone.Index(255975)

	outputID := randOutputID()
	messageID := randMessageID()
	address := randAddress(iotago.AddressEd25519)
	amount := uint64(832493)
	iotaOutput := &iotago.ExtendedOutput{
		Address: address,
		Amount:  amount,
	}
	output := CreateOutput(outputID, messageID, confirmationIndex, iotaOutput)

	transactionID := &iotago.TransactionID{}
	copy(transactionID[:], randBytes(iotago.TransactionIDLength))

	spent := NewSpent(output, transactionID, confirmationIndex)

	diff := &MilestoneDiff{
		Index:   confirmationIndex,
		Outputs: Outputs{output},
		Spents:  Spents{spent},
	}

	confirmationIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(confirmationIndexBytes, uint32(confirmationIndex))

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixMilestoneDiffs}, confirmationIndexBytes), diff.kvStorableKey())

	value := diff.kvStorableValue()
	require.Equal(t, len(value), 77)
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[:4]))
	require.Equal(t, outputID[:], value[4:38])
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[38:42]))
	require.Equal(t, outputID[:], value[42:76])
	require.Equal(t, value[76], byte(0))
}

func TestTreasuryMilestoneDiffSerialization(t *testing.T) {
	outputID := randOutputID()
	messageID := randMessageID()
	address := randAddress(iotago.AddressEd25519)
	amount := uint64(235234)
	msIndex := randMilestoneIndex()
	iotaOutput := &iotago.ExtendedOutput{
		Address: address,
		Amount:  amount,
	}
	output := CreateOutput(outputID, messageID, msIndex, iotaOutput)

	transactionID := &iotago.TransactionID{}
	copy(transactionID[:], randBytes(iotago.TransactionIDLength))

	confirmationIndex := milestone.Index(255975)

	spent := NewSpent(output, transactionID, confirmationIndex)

	spentMilestoneID := iotago.MilestoneID{}
	copy(spentMilestoneID[:], randBytes(iotago.MilestoneIDLength))

	spentTreasuryOutput := &TreasuryOutput{
		MilestoneID: spentMilestoneID,
		Amount:      1337,
		Spent:       true,
	}

	milestoneID := iotago.MilestoneID{}
	copy(milestoneID[:], randBytes(iotago.MilestoneIDLength))

	treasuryOutput := &TreasuryOutput{
		MilestoneID: milestoneID,
		Amount:      0,
		Spent:       false,
	}

	diff := &MilestoneDiff{
		Index:               confirmationIndex,
		Outputs:             Outputs{output},
		Spents:              Spents{spent},
		SpentTreasuryOutput: spentTreasuryOutput,
		TreasuryOutput:      treasuryOutput,
	}

	confirmationIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(confirmationIndexBytes, uint32(confirmationIndex))

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixMilestoneDiffs}, confirmationIndexBytes), diff.kvStorableKey())

	value := diff.kvStorableValue()
	require.Equal(t, len(value), 141)
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[:4]))
	require.Equal(t, outputID[:], value[4:38])
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[38:42]))
	require.Equal(t, outputID[:], value[42:76])
	require.Equal(t, value[76], byte(1))
	require.Equal(t, value[77:109], milestoneID[:])
	require.Equal(t, value[109:141], spentMilestoneID[:])
}

func randMilestoneIndex() milestone.Index {
	return milestone.Index(rand.Uint32())
}

func randAddress(addressType iotago.AddressType) iotago.Address {
	switch addressType {
	case iotago.AddressEd25519:
		address := &iotago.Ed25519Address{}
		addressBytes := randBytes(32)
		copy(address[:], addressBytes)
		return address
	case iotago.AddressNFT:
		return randNFTID().ToAddress()
	case iotago.AddressAlias:
		return randAliasID().ToAddress()
	default:
		panic("unknown address type")
	}
}

func randOutputID() *iotago.OutputID {
	outputID := &iotago.OutputID{}
	copy(outputID[:], testutil.RandBytes(iotago.OutputIDLength))
	return outputID
}

func randOutput(outputType iotago.OutputType) *Output {
	var addr iotago.Address
	if outputType == iotago.OutputFoundry {
		addr = randAddress(iotago.AddressAlias)
	} else {
		addr = randAddress(iotago.AddressEd25519)
	}
	return randOutputOnAddress(outputType, addr)
}

func randOutputOnAddress(outputType iotago.OutputType, address iotago.Address) *Output {
	return randOutputOnAddressWithAmount(outputType, address, uint64(rand.Intn(2156465)))
}

func randOutputOnAddressWithAmount(outputType iotago.OutputType, address iotago.Address, amount uint64) *Output {
	outputID := randOutputID()
	messageID := randMessageID()
	msIndex := randMilestoneIndex()

	var iotaOutput iotago.Output

	switch outputType {
	case iotago.OutputExtended:
		iotaOutput = &iotago.ExtendedOutput{
			Address: address,
			Amount:  amount,
		}
	case iotago.OutputAlias:
		iotaOutput = &iotago.AliasOutput{
			Amount:               amount,
			AliasID:              randAliasID(),
			StateController:      address,
			GovernanceController: address,
			StateMetadata:        []byte{},
		}
	case iotago.OutputNFT:
		iotaOutput = &iotago.NFTOutput{
			Address:           address,
			Amount:            amount,
			NFTID:             randNFTID(),
			ImmutableMetadata: []byte{},
		}
	case iotago.OutputFoundry:
		if address.Type() != iotago.AddressAlias {
			panic("not an alias address")
		}
		supply := new(big.Int).SetUint64(rand.Uint64())
		iotaOutput = &iotago.FoundryOutput{
			Address:           address,
			Amount:            amount,
			SerialNumber:      0,
			TokenTag:          randTokenTag(),
			CirculatingSupply: supply,
			MaximumSupply:     supply,
			TokenScheme:       &iotago.SimpleTokenScheme{},
		}
	default:
		panic("unhandled output type")
	}

	return CreateOutput(outputID, messageID, msIndex, iotaOutput)
}

func EqualOutputs(t *testing.T, expected Outputs, actual Outputs) {
	require.Equal(t, len(expected), len(actual))

	for i := 0; i < len(expected); i++ {
		EqualOutput(t, expected[i], actual[i])
	}
}

func EqualSpents(t *testing.T, expected Spents, actual Spents) {
	require.Equal(t, len(expected), len(actual))

	for i := 0; i < len(expected); i++ {
		EqualSpent(t, expected[i], actual[i])
	}
}

func randomSpent(output *Output, index milestone.Index) *Spent {
	transactionID := &iotago.TransactionID{}
	copy(transactionID[:], randBytes(iotago.TransactionIDLength))
	return NewSpent(output, transactionID, index)
}

func TestMilestoneDiffSerialization(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	outputs := Outputs{
		randOutput(iotago.OutputExtended),
		randOutput(iotago.OutputExtended),
		randOutput(iotago.OutputExtended),
		randOutput(iotago.OutputExtended),
		randOutput(iotago.OutputExtended),
	}

	msIndex := milestone.Index(756)

	spents := Spents{
		randomSpent(outputs[3], msIndex),
		randomSpent(outputs[2], msIndex),
	}

	spentMilestoneID := iotago.MilestoneID{}
	copy(spentMilestoneID[:], randBytes(iotago.MilestoneIDLength))

	spentTreasuryOutput := &TreasuryOutput{
		MilestoneID: spentMilestoneID,
		Amount:      1337,
		Spent:       true,
	}

	milestoneID := iotago.MilestoneID{}
	copy(milestoneID[:], randBytes(iotago.MilestoneIDLength))

	treasuryOutput := &TreasuryOutput{
		MilestoneID: milestoneID,
		Amount:      0,
		Spent:       false,
	}

	treasuryTuple := &TreasuryMutationTuple{
		NewOutput:   treasuryOutput,
		SpentOutput: spentTreasuryOutput,
	}

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, treasuryTuple, nil))

	readDiff, err := utxo.MilestoneDiffWithoutLocking(msIndex)
	require.NoError(t, err)

	require.Equal(t, msIndex, readDiff.Index)
	EqualOutputs(t, outputs, readDiff.Outputs)
	EqualSpents(t, spents, readDiff.Spents)
	require.Equal(t, treasuryOutput, readDiff.TreasuryOutput)
	require.Equal(t, spentTreasuryOutput, readDiff.SpentTreasuryOutput)
}
