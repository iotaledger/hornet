package main

import (
	"os"

	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	"github.com/iotaledger/hornet/v2/pkg/tpkg"
	iotago "github.com/iotaledger/iota.go/v3"
)

var protoParams = &iotago.ProtocolParameters{
	Version:       2,
	NetworkName:   "alphanet1",
	Bech32HRP:     iotago.PrefixDevnet,
	MinPoWScore:   10,
	BelowMaxDepth: 15,
	RentStructure: iotago.RentStructure{
		VByteCost:    0,
		VBFactorKey:  0,
		VBFactorData: 0,
	},
	TokenSupply: 2_779_530_283_277_761,
}

func main() {
	fullSnapshotHeader := writeFullSnapshot()
	writeDeltaSnapshot(fullSnapshotHeader)
}

// for the testing purposes it doesn't actually matter
// whether the milestones are correct. therefore the milestone is just
// filled with enough data that it still passes deserialization with validation.
func blankMilestone(index iotago.MilestoneIndex) *iotago.Milestone {
	return &iotago.Milestone{
		Index:               index,
		Timestamp:           0,
		ProtocolVersion:     2,
		PreviousMilestoneID: iotago.MilestoneID{},
		Parents: iotago.BlockIDs{
			static32ByteID(0),
			static32ByteID(1),
		},
		InclusionMerkleRoot: static32ByteID(2),
		AppliedMerkleRoot:   static32ByteID(2),
		Metadata:            nil,
		Opts:                nil,
		Signatures: []iotago.Signature{
			&iotago.Ed25519Signature{
				PublicKey: static32ByteID(0),
				Signature: static64ByteID(0),
			},
			&iotago.Ed25519Signature{
				PublicKey: static32ByteID(1),
				Signature: static64ByteID(1),
			},
		},
	}
}

var originTreasurySupply = protoParams.TokenSupply - fullSnapshotOutputs[0].Deposit() - fullSnapshotOutputs[1].Deposit()

var fullSnapshotOutputs = utxo.Outputs{
	utxoOutput(6, 10_000_000, 3),
	utxoOutput(5, 20_000_000, 3),
}

var fullSnapshotMsDiffs = []*snapshot.MilestoneDiff{
	{
		Milestone: blankMilestone(3),
		Created: utxo.Outputs{
			utxoOutput(6, fullSnapshotOutputs[0].Deposit(), 3),
			utxoOutput(5, fullSnapshotOutputs[1].Deposit(), 3),
		},
		Consumed: utxo.Spents{
			utxoSpent(4, fullSnapshotOutputs[0].Deposit(), 2, 3),
			utxoSpent(3, fullSnapshotOutputs[1].Deposit(), 2, 3),
		},
		SpentTreasuryOutput: nil,
	},
	{
		Milestone: blankMilestone(2),
		Created: utxo.Outputs{
			utxoOutput(4, fullSnapshotOutputs[0].Deposit(), 2),
			utxoOutput(3, fullSnapshotOutputs[1].Deposit(), 2),
		},
		Consumed: utxo.Spents{
			utxoSpent(2, fullSnapshotOutputs[0].Deposit(), 1, 2),
			utxoSpent(1, fullSnapshotOutputs[1].Deposit(), 1, 2),
		},
		SpentTreasuryOutput: nil,
	},
}

func writeFullSnapshot() *snapshot.FullSnapshotHeader {

	protoParamsBytes, err := protoParams.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		panic(err)
	}

	protoParamsMsOption := &iotago.ProtocolParamsMilestoneOpt{
		TargetMilestoneIndex: 0,
		ProtocolVersion:      2,
		Params:               protoParamsBytes,
	}

	fullSnapshotHeader := &snapshot.FullSnapshotHeader{
		Version:                  snapshot.SupportedFormatVersion,
		Type:                     snapshot.Full,
		GenesisMilestoneIndex:    0,
		TargetMilestoneIndex:     1,
		TargetMilestoneTimestamp: 0,
		TargetMilestoneID:        iotago.MilestoneID{},
		LedgerMilestoneIndex:     3,
		TreasuryOutput: &utxo.TreasuryOutput{
			MilestoneID: iotago.MilestoneID{},
			Amount:      originTreasurySupply,
		},
		ProtocolParamsMilestoneOpt: protoParamsMsOption,
		OutputCount:                0,
		MilestoneDiffCount:         0,
		SEPCount:                   0,
	}

	fileHandle, err := os.Create("snapshot_full_snapshot.bin")
	must(err)
	defer func() { _ = fileHandle.Close() }()

	var currentOutput int
	fullSnapOutputProd := func() (*utxo.Output, error) {
		if currentOutput == len(fullSnapshotOutputs) {
			//nolint:nilnil // nil, nil is ok in this context, even if it is not go idiomatic
			return nil, nil
		}
		out := fullSnapshotOutputs[currentOutput]
		currentOutput++

		return out, nil
	}

	var currentMsDiff int
	fullSnapMsDiffProd := func() (*snapshot.MilestoneDiff, error) {
		if currentMsDiff == len(fullSnapshotMsDiffs) {
			//nolint:nilnil // nil, nil is ok in this context, even if it is not go idiomatic
			return nil, nil
		}
		msDiff := fullSnapshotMsDiffs[currentMsDiff]
		currentMsDiff++

		return msDiff, nil
	}

	var seps, sepsMax = 0, 10
	fullSnapSEPProd := func() (iotago.BlockID, error) {
		seps++
		if seps == 1 {
			return iotago.EmptyBlockID(), nil
		}
		if seps > sepsMax {
			return iotago.EmptyBlockID(), snapshot.ErrNoMoreSEPToProduce
		}

		return tpkg.RandBlockID(), nil
	}

	_, err = snapshot.StreamFullSnapshotDataTo(
		fileHandle,
		fullSnapshotHeader,
		fullSnapOutputProd,
		fullSnapMsDiffProd,
		fullSnapSEPProd,
	)
	must(err)

	return fullSnapshotHeader
}

var deltaSnapshotMsDiffs = []*snapshot.MilestoneDiff{
	fullSnapshotMsDiffs[1],
	fullSnapshotMsDiffs[0],
	{
		Milestone: blankMilestone(4),
		Created: utxo.Outputs{
			utxoOutput(8, fullSnapshotOutputs[0].Deposit(), 4),
			utxoOutput(7, fullSnapshotOutputs[1].Deposit(), 4),
		},
		Consumed: utxo.Spents{
			utxoSpent(6, fullSnapshotOutputs[0].Deposit(), 3, 4),
			utxoSpent(5, fullSnapshotOutputs[1].Deposit(), 3, 4),
		},
		SpentTreasuryOutput: nil,
	},
	{
		// milestone 5 has a receipt
		Milestone: func() *iotago.Milestone {
			ms := blankMilestone(5)
			ttx := &iotago.TreasuryTransaction{
				Input: &iotago.TreasuryInput{},
				Output: &iotago.TreasuryOutput{
					Amount: originTreasurySupply - 10_000_000,
				},
			}
			receipt, err := iotago.NewReceiptBuilder(9001).
				AddTreasuryTransaction(ttx).
				AddEntry(&iotago.MigratedFundsEntry{
					TailTransactionHash: iotago.LegacyTailTransactionHash{},
					Address:             &iotago.Ed25519Address{},
					Deposit:             10_000_000,
				}).
				Build(protoParams)
			if err != nil {
				panic(err)
			}
			ms.Opts = iotago.MilestoneOpts{receipt}

			return ms
		}(),
		Created: utxo.Outputs{
			utxoOutput(9, fullSnapshotOutputs[0].Deposit()+fullSnapshotOutputs[1].Deposit()+10_000_000, 5),
		},
		Consumed: utxo.Spents{
			utxoSpent(8, fullSnapshotOutputs[0].Deposit(), 4, 5),
			utxoSpent(7, fullSnapshotOutputs[1].Deposit(), 4, 5),
		},
		SpentTreasuryOutput: &utxo.TreasuryOutput{
			MilestoneID: iotago.MilestoneID{},
			Amount:      originTreasurySupply,
			Spent:       true,
		},
	},
}

func writeDeltaSnapshot(fullSnapshotHeader *snapshot.FullSnapshotHeader) {

	deltaSnapshotHeader := &snapshot.DeltaSnapshotHeader{
		Version:                       snapshot.SupportedFormatVersion,
		Type:                          snapshot.Delta,
		TargetMilestoneIndex:          5,
		TargetMilestoneTimestamp:      0,
		FullSnapshotTargetMilestoneID: fullSnapshotHeader.TargetMilestoneID,
		SEPFileOffset:                 0,
		MilestoneDiffCount:            0,
		SEPCount:                      0,
	}

	fileHandle, err := os.Create("snapshot_delta_snapshot.bin")
	must(err)
	defer func() { _ = fileHandle.Close() }()

	var currentMsDiff int
	deltaSnapMsDiffProd := func() (*snapshot.MilestoneDiff, error) {
		if currentMsDiff == len(deltaSnapshotMsDiffs) {
			//nolint:nilnil // nil, nil is ok in this context, even if it is not go idiomatic
			return nil, nil
		}
		msDiff := deltaSnapshotMsDiffs[currentMsDiff]
		currentMsDiff++

		return msDiff, nil
	}

	var seps, sepsMax = 0, 10
	deltaSnapSEPProd := func() (iotago.BlockID, error) {
		seps++
		if seps > sepsMax {
			return iotago.EmptyBlockID(), snapshot.ErrNoMoreSEPToProduce
		}

		return tpkg.RandBlockID(), nil
	}

	_, err = snapshot.StreamDeltaSnapshotDataTo(
		fileHandle,
		deltaSnapshotHeader,
		deltaSnapMsDiffProd,
		deltaSnapSEPProd,
	)
	must(err)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func static64ByteID(fill byte) [64]byte {
	var b [64]byte
	for i := 0; i < len(b); i++ {
		b[i] = fill
	}

	return b
}

func static32ByteID(fill byte) [32]byte {
	var b [32]byte
	for i := 0; i < len(b); i++ {
		b[i] = fill
	}

	return b
}

func staticBlockID(fill byte) iotago.BlockID {
	bytes := static32ByteID(fill)
	blockID := iotago.BlockID{}
	copy(blockID[:], bytes[:])

	return blockID
}

func staticOutputID(fill byte) iotago.OutputID {
	b := iotago.OutputID{}
	for i := 0; i < len(b); i++ {
		b[i] = fill
	}

	return b
}

func staticEd25519Address(fill byte) iotago.Address {
	b := static32ByteID(fill)
	var addr iotago.Ed25519Address
	copy(addr[:], b[:])

	return &addr
}

func utxoOutput(fill byte, amount uint64, msIndexBooked iotago.MilestoneIndex) *utxo.Output {
	return utxo.CreateOutput(
		staticOutputID(fill),
		staticBlockID(fill),
		msIndexBooked,
		0,
		&iotago.BasicOutput{
			Amount: amount,
			Conditions: iotago.UnlockConditions{
				&iotago.AddressUnlockCondition{
					Address: staticEd25519Address(fill),
				},
			},
		},
	)
}

func utxoSpent(fill byte, amount uint64, msIndexBooked iotago.MilestoneIndex, msIndexSpent iotago.MilestoneIndex) *utxo.Spent {
	r := static32ByteID(fill)
	txIDSpent := iotago.TransactionID{}
	copy(txIDSpent[:], r[:])

	return utxo.NewSpent(utxoOutput(fill, amount, msIndexBooked), txIDSpent, msIndexSpent, 0)
}
