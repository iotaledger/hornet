package tpkg

import (
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"math/rand"

	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
)

// RandByte returns a random byte.
func RandByte() byte {
	return byte(rand.Intn(256))
}

// RandBytes returns length amount random bytes.
func RandBytes(length int) []byte {
	var b []byte
	for i := 0; i < length; i++ {
		b = append(b, byte(rand.Intn(256)))
	}
	return b
}

func RandString(length int) string {
	return string(RandBytes(length))
}

// RandUint8 returns a random uint8.
func RandUint8(max uint8) uint8 {
	return uint8(rand.Int31n(int32(max)))
}

// RandUint16 returns a random uint16.
func RandUint16(max uint16) uint16 {
	return uint16(rand.Int31n(int32(max)))
}

// RandUint32 returns a random uint32.
func RandUint32(max uint32) uint32 {
	return uint32(rand.Int63n(int64(max)))
}

// RandUint64 returns a random uint64.
func RandUint64(max uint64) uint64 {
	return uint64(rand.Int63n(int64(uint32(max))))
}

// RandFloat64 returns a random float64.
func RandFloat64(max float64) float64 {
	return rand.Float64() * max
}

func Rand32ByteHash() [32]byte {
	var h [32]byte
	copy(h[:], RandBytes(32))
	return h
}

func RandOutputID(index ...uint16) iotago.OutputID {
	idx := RandUint16(126)
	if len(index) > 0 {
		idx = index[0]
	}

	var outputID iotago.OutputID
	_, err := rand.Read(outputID[:iotago.TransactionIDLength])
	if err != nil {
		panic(err)
	}
	binary.LittleEndian.PutUint16(outputID[iotago.TransactionIDLength:], idx)
	return outputID
}

func RandBlockID() iotago.BlockID {
	blockID := iotago.BlockID{}
	copy(blockID[:], RandBytes(iotago.BlockIDLength))
	return blockID
}

func RandTransactionID() iotago.TransactionID {
	transactionID := iotago.TransactionID{}
	copy(transactionID[:], RandBytes(iotago.TransactionIDLength))
	return transactionID
}

func RandNFTID() iotago.NFTID {
	nft := iotago.NFTID{}
	copy(nft[:], RandBytes(iotago.NFTIDLength))
	return nft
}

func RandAliasID() iotago.AliasID {
	alias := iotago.AliasID{}
	copy(alias[:], RandBytes(iotago.AliasIDLength))
	return alias
}

func RandMilestoneIndex() iotago.MilestoneIndex {
	return RandUint32(math.MaxUint32)
}

func RandMilestoneTimestamp() uint32 {
	return RandUint32(math.MaxUint32)
}

func RandAmount() uint64 {
	return RandUint64(math.MaxUint64)
}

func RandMilestoneID() iotago.MilestoneID {
	id := iotago.MilestoneID{}
	copy(id[:], RandBytes(iotago.MilestoneIDLength))
	return id
}

func RandAddress(addressType iotago.AddressType) iotago.Address {
	switch addressType {
	case iotago.AddressEd25519:
		address := &iotago.Ed25519Address{}
		addressBytes := RandBytes(32)
		copy(address[:], addressBytes)
		return address
	case iotago.AddressNFT:
		return RandNFTID().ToAddress()
	case iotago.AddressAlias:
		return RandAliasID().ToAddress()
	default:
		panic("unknown address type")
	}
}

func RandOutputType() iotago.OutputType {
	return iotago.OutputType(byte(rand.Intn(3) + 3))
}

func RandOutput(outputType iotago.OutputType) iotago.Output {
	var addr iotago.Address
	if outputType == iotago.OutputFoundry {
		addr = RandAddress(iotago.AddressAlias)
	} else {
		addr = RandAddress(iotago.AddressEd25519)
	}
	return RandOutputOnAddress(outputType, addr)
}

func RandOutputOnAddress(outputType iotago.OutputType, address iotago.Address) iotago.Output {
	return RandOutputOnAddressWithAmount(outputType, address, RandAmount())
}

func RandOutputOnAddressWithAmount(outputType iotago.OutputType, address iotago.Address, amount uint64) iotago.Output {

	var iotaOutput iotago.Output

	switch outputType {
	case iotago.OutputBasic:
		iotaOutput = &iotago.BasicOutput{
			Amount: amount,
			Conditions: iotago.UnlockConditions{
				&iotago.AddressUnlockCondition{
					Address: address,
				},
			},
		}
	case iotago.OutputAlias:
		iotaOutput = &iotago.AliasOutput{
			Amount:  amount,
			AliasID: RandAliasID(),
			Conditions: iotago.UnlockConditions{
				&iotago.StateControllerAddressUnlockCondition{
					Address: address,
				},
				&iotago.GovernorAddressUnlockCondition{
					Address: address,
				},
			},
		}
	case iotago.OutputFoundry:
		if address.Type() != iotago.AddressAlias {
			panic("not an alias address")
		}
		supply := new(big.Int).SetUint64(RandAmount())
		iotaOutput = &iotago.FoundryOutput{
			Amount:       amount,
			SerialNumber: 0,
			TokenScheme: &iotago.SimpleTokenScheme{
				MintedTokens:  supply,
				MeltedTokens:  new(big.Int).SetBytes([]byte{0}),
				MaximumSupply: supply,
			},
			Conditions: iotago.UnlockConditions{
				&iotago.ImmutableAliasUnlockCondition{
					Address: address.(*iotago.AliasAddress),
				},
			},
		}
	case iotago.OutputNFT:
		iotaOutput = &iotago.NFTOutput{
			Amount: amount,
			NFTID:  RandNFTID(),
			Conditions: iotago.UnlockConditions{
				&iotago.AddressUnlockCondition{
					Address: address,
				},
			},
		}
	default:
		panic("unhandled output type")
	}

	return iotaOutput
}

func RandTreasuryOutput() *utxo.TreasuryOutput {
	return &utxo.TreasuryOutput{MilestoneID: RandMilestoneID(), Amount: RandAmount()}
}

func RandUTXOOutput() *utxo.Output {
	return RandUTXOOutputWithType(RandOutputType())
}

func RandUTXOOutputWithType(outputType iotago.OutputType) *utxo.Output {
	return utxo.CreateOutput(RandOutputID(), RandBlockID(), RandMilestoneIndex(), RandMilestoneTimestamp(), RandOutput(outputType))
}

func RandUTXOOutputOnAddress(outputType iotago.OutputType, address iotago.Address) *utxo.Output {
	return utxo.CreateOutput(RandOutputID(), RandBlockID(), RandMilestoneIndex(), RandMilestoneTimestamp(), RandOutputOnAddress(outputType, address))
}

func RandUTXOOutputOnAddressWithAmount(outputType iotago.OutputType, address iotago.Address, amount uint64) *utxo.Output {
	return utxo.CreateOutput(RandOutputID(), RandBlockID(), RandMilestoneIndex(), RandMilestoneTimestamp(), RandOutputOnAddressWithAmount(outputType, address, amount))
}

func RandUTXOSpent(msIndexSpent iotago.MilestoneIndex, msTimestampSpent uint32) *utxo.Spent {
	return utxo.NewSpent(RandUTXOOutput(), RandTransactionID(), msIndexSpent, msTimestampSpent)
}

func RandUTXOSpentWithOutput(output *utxo.Output, msIndexSpent iotago.MilestoneIndex, msTimestampSpent uint32) *utxo.Spent {
	return utxo.NewSpent(output, RandTransactionID(), msIndexSpent, msTimestampSpent)
}

func RandReceipt(msIndex iotago.MilestoneIndex, protoParams *iotago.ProtocolParameters) (*iotago.ReceiptMilestoneOpt, error) {
	treasuryInput := &iotago.TreasuryInput{}
	copy(treasuryInput[:], RandBytes(32))
	ed25519Addr := RandAddress(iotago.AddressEd25519)
	migratedFundsEntry := &iotago.MigratedFundsEntry{Address: ed25519Addr, Deposit: RandAmount()}
	copy(migratedFundsEntry.TailTransactionHash[:], RandBytes(49))
	return iotago.NewReceiptBuilder(msIndex).
		AddTreasuryTransaction(&iotago.TreasuryTransaction{
			Input:  treasuryInput,
			Output: &iotago.TreasuryOutput{Amount: RandAmount()},
		}).
		AddEntry(migratedFundsEntry).
		Build(protoParams)
}

// RandRentStructure produces random rent structure.
func RandRentStructure() *iotago.RentStructure {
	return &iotago.RentStructure{
		VByteCost:    RandUint32(math.MaxUint32),
		VBFactorData: iotago.VByteCostFactor(RandUint8(math.MaxUint8)),
		VBFactorKey:  iotago.VByteCostFactor(RandUint8(math.MaxUint8)),
	}
}

// RandProtocolParameters produces random protocol parameters.
func RandProtocolParameters() *iotago.ProtocolParameters {
	return &iotago.ProtocolParameters{
		Version:       RandByte(),
		NetworkName:   RandString(255),
		Bech32HRP:     iotago.NetworkPrefix(RandString(255)),
		MinPoWScore:   RandUint32(50000),
		BelowMaxDepth: RandUint8(math.MaxUint8),
		RentStructure: *RandRentStructure(),
		TokenSupply:   RandAmount(),
	}
}

// RandProtocolParamsMilestoneOpt produces random protocol parameters milestone option.
func RandProtocolParamsMilestoneOpt() *iotago.ProtocolParamsMilestoneOpt {
	protoParams := RandProtocolParameters()

	protoParamsBytes, err := protoParams.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		panic(fmt.Errorf("failed to serialize protocol parameters: %w", err))
	}

	return &iotago.ProtocolParamsMilestoneOpt{
		TargetMilestoneIndex: RandMilestoneIndex(),
		ProtocolVersion:      2,
		Params:               protoParamsBytes,
	}
}

const seedLength = ed25519.SeedSize

func RandSeed() []byte {
	var b [seedLength]byte
	_, err := rand.Read(b[:])
	if err != nil {
		panic(err)
	}
	return b[:]
}
