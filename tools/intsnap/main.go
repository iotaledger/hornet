package main

import (
	"crypto/rand"
	"os"
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/snapshot"
	iotago "github.com/iotaledger/iota.go/v2"
)

func main() {
	writeFullSnapshot()
	writeDeltaSnapshot()
}

// for the testing purposes it doesn't actually matter
// whether the milestones are correct. therefore the milestone is just
// filled with enough data that it still passes deserialization with validation.
func blankMilestone(index uint32) *iotago.Milestone {
	return &iotago.Milestone{
		Index:     index,
		Timestamp: 0,
		Receipt:   nil,
		Parents: iotago.MilestoneParentMessageIDs{
			static32ByteID(0),
			static32ByteID(1),
		},
		InclusionMerkleProof: static32ByteID(2),
		PublicKeys: []iotago.MilestonePublicKey{
			static32ByteID(0),
			static32ByteID(1),
		},
		Signatures: []iotago.MilestoneSignature{
			static64ByteID(0),
			static64ByteID(1),
		},
	}
}

var fullSnapshotHeader = &snapshot.FileHeader{
	Version:              snapshot.SupportedFormatVersion,
	Type:                 snapshot.Full,
	NetworkID:            iotago.NetworkIDFromString("alphanet1"),
	SEPMilestoneIndex:    1,
	LedgerMilestoneIndex: 3,
	TreasuryOutput: &utxo.TreasuryOutput{
		MilestoneID: iotago.MilestoneID{},
		Amount:      originTreasurySupply,
	},
}

var originTreasurySupply = iotago.TokenSupply - fullSnapshotOutputs[0].Amount - fullSnapshotOutputs[1].Amount

var fullSnapshotOutputs = []*snapshot.Output{
	{
		MessageID:  static32ByteID(6),
		OutputID:   static34ByteID(6),
		OutputType: iotago.OutputSigLockedSingleOutput,
		Address:    staticEd25519Address(6),
		Amount:     10_000_000,
	},
	{
		MessageID:  static32ByteID(5),
		OutputID:   static34ByteID(5),
		OutputType: iotago.OutputSigLockedSingleOutput,
		Address:    staticEd25519Address(5),
		Amount:     20_000_000,
	},
}

var fullSnapshotMsDiffs = []*snapshot.MilestoneDiff{
	{
		Milestone: blankMilestone(3),
		Created: []*snapshot.Output{
			{
				MessageID:  static32ByteID(6),
				OutputID:   static34ByteID(6),
				OutputType: iotago.OutputSigLockedSingleOutput,
				Address:    staticEd25519Address(6),
				Amount:     fullSnapshotOutputs[0].Amount,
			},
			{
				MessageID:  static32ByteID(5),
				OutputID:   static34ByteID(5),
				OutputType: iotago.OutputSigLockedSingleOutput,
				Address:    staticEd25519Address(5),
				Amount:     fullSnapshotOutputs[1].Amount,
			},
		},
		Consumed: []*snapshot.Spent{
			{
				Output: snapshot.Output{
					MessageID:  static32ByteID(4),
					OutputID:   static34ByteID(4),
					OutputType: iotago.OutputSigLockedSingleOutput,
					Address:    staticEd25519Address(4),
					Amount:     fullSnapshotOutputs[0].Amount,
				},
				TargetTransactionID: static32ByteID(4),
			},
			{
				Output: snapshot.Output{
					MessageID:  static32ByteID(3),
					OutputID:   static34ByteID(3),
					OutputType: iotago.OutputSigLockedSingleOutput,
					Address:    staticEd25519Address(3),
					Amount:     fullSnapshotOutputs[1].Amount,
				},
				TargetTransactionID: static32ByteID(3),
			},
		},
	},
	{
		Milestone: blankMilestone(2),
		Created: []*snapshot.Output{
			{
				MessageID:  static32ByteID(4),
				OutputID:   static34ByteID(4),
				OutputType: iotago.OutputSigLockedSingleOutput,
				Address:    staticEd25519Address(4),
				Amount:     fullSnapshotOutputs[0].Amount,
			},
			{
				MessageID:  static32ByteID(3),
				OutputID:   static34ByteID(3),
				OutputType: iotago.OutputSigLockedSingleOutput,
				Address:    staticEd25519Address(3),
				Amount:     fullSnapshotOutputs[1].Amount,
			},
		},
		Consumed: []*snapshot.Spent{
			{
				Output: snapshot.Output{
					MessageID:  static32ByteID(2),
					OutputID:   static34ByteID(2),
					OutputType: iotago.OutputSigLockedSingleOutput,
					Address:    staticEd25519Address(2),
					Amount:     fullSnapshotOutputs[0].Amount,
				},
				TargetTransactionID: static32ByteID(2),
			},
			{
				Output: snapshot.Output{
					MessageID:  static32ByteID(1),
					OutputID:   static34ByteID(1),
					OutputType: iotago.OutputSigLockedSingleOutput,
					Address:    staticEd25519Address(1),
					Amount:     fullSnapshotOutputs[1].Amount,
				},
				TargetTransactionID: static32ByteID(1),
			},
		},
	},
}

func writeFullSnapshot() {
	full, err := os.Create("full_snapshot.bin")
	must(err)
	defer func() { _ = full.Close() }()

	var seps, sepsMax = 0, 10
	fullSnapSEPProd := func() (hornet.MessageID, error) {
		seps++
		if seps > sepsMax {
			return nil, nil
		}
		return randMsgID(), nil
	}

	var currentOutput int
	fullSnapOutputProd := func() (*snapshot.Output, error) {
		if currentOutput == len(fullSnapshotOutputs) {
			return nil, nil
		}
		out := fullSnapshotOutputs[currentOutput]
		currentOutput++
		return out, nil
	}

	var currentMsDiff int
	fullSnapMsDiffProd := func() (*snapshot.MilestoneDiff, error) {
		if currentMsDiff == len(fullSnapshotMsDiffs) {
			return nil, nil
		}
		msDiff := fullSnapshotMsDiffs[currentMsDiff]
		currentMsDiff++
		return msDiff, nil
	}

	_, err = snapshot.StreamSnapshotDataTo(full, uint64(time.Now().Unix()), fullSnapshotHeader, fullSnapSEPProd, fullSnapOutputProd, fullSnapMsDiffProd)
	must(err)
}

var deltaSnapshotHeader = &snapshot.FileHeader{
	Version:              snapshot.SupportedFormatVersion,
	Type:                 snapshot.Delta,
	NetworkID:            iotago.NetworkIDFromString("alphanet1"),
	SEPMilestoneIndex:    5,
	LedgerMilestoneIndex: 1,
}

var deltaSnapshotMsDiffs = []*snapshot.MilestoneDiff{
	fullSnapshotMsDiffs[1],
	fullSnapshotMsDiffs[0],
	{
		Milestone: blankMilestone(4),
		Created: []*snapshot.Output{
			{
				MessageID:  static32ByteID(8),
				OutputID:   static34ByteID(8),
				OutputType: iotago.OutputSigLockedSingleOutput,
				Address:    staticEd25519Address(8),
				Amount:     fullSnapshotOutputs[0].Amount,
			},
			{
				MessageID:  static32ByteID(7),
				OutputID:   static34ByteID(7),
				OutputType: iotago.OutputSigLockedSingleOutput,
				Address:    staticEd25519Address(7),
				Amount:     fullSnapshotOutputs[1].Amount,
			},
		},
		Consumed: []*snapshot.Spent{
			{
				Output: snapshot.Output{
					MessageID:  static32ByteID(6),
					OutputID:   static34ByteID(6),
					OutputType: iotago.OutputSigLockedSingleOutput,
					Address:    staticEd25519Address(6),
					Amount:     fullSnapshotOutputs[0].Amount,
				},
				TargetTransactionID: static32ByteID(6),
			},
			{
				Output: snapshot.Output{
					MessageID:  static32ByteID(5),
					OutputID:   static34ByteID(5),
					OutputType: iotago.OutputSigLockedSingleOutput,
					Address:    staticEd25519Address(5),
					Amount:     fullSnapshotOutputs[1].Amount,
				},
				TargetTransactionID: static32ByteID(5),
			},
		},
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
				Build()
			if err != nil {
				panic(err)
			}
			ms.Receipt = receipt
			return ms
		}(),
		SpentTreasuryOutput: &utxo.TreasuryOutput{
			MilestoneID: iotago.MilestoneID{},
			Amount:      originTreasurySupply,
			Spent:       true,
		},
		Created: []*snapshot.Output{
			{
				MessageID:  static32ByteID(9),
				OutputID:   static34ByteID(9),
				OutputType: iotago.OutputSigLockedSingleOutput,
				Address:    staticEd25519Address(9),
				Amount:     fullSnapshotOutputs[0].Amount + fullSnapshotOutputs[1].Amount + 10_000_000,
			},
		},
		Consumed: []*snapshot.Spent{
			{
				Output: snapshot.Output{
					MessageID:  static32ByteID(8),
					OutputID:   static34ByteID(8),
					OutputType: iotago.OutputSigLockedSingleOutput,
					Address:    staticEd25519Address(8),
					Amount:     fullSnapshotOutputs[0].Amount,
				},
				TargetTransactionID: static32ByteID(7),
			},
			{
				Output: snapshot.Output{
					MessageID:  static32ByteID(7),
					OutputID:   static34ByteID(7),
					OutputType: iotago.OutputSigLockedSingleOutput,
					Address:    staticEd25519Address(7),
					Amount:     fullSnapshotOutputs[1].Amount,
				},
				TargetTransactionID: static32ByteID(7),
			},
		},
	},
}

func writeDeltaSnapshot() {
	delta, err := os.Create("delta_snapshot.bin")
	must(err)
	defer func() { _ = delta.Close() }()

	var seps, sepsMax = 0, 10
	deltaSnapSEPProd := func() (hornet.MessageID, error) {
		seps++
		if seps > sepsMax {
			return nil, nil
		}
		return randMsgID(), nil
	}

	var currentMsDiff int
	deltaSnapMsDiffProd := func() (*snapshot.MilestoneDiff, error) {
		if currentMsDiff == len(deltaSnapshotMsDiffs) {
			return nil, nil
		}
		msDiff := deltaSnapshotMsDiffs[currentMsDiff]
		currentMsDiff++
		return msDiff, nil
	}

	_, err = snapshot.StreamSnapshotDataTo(delta, uint64(time.Now().Unix()), deltaSnapshotHeader, deltaSnapSEPProd, nil, deltaSnapMsDiffProd)
	must(err)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func randMsgID() hornet.MessageID {
	b := make(hornet.MessageID, 32)
	_, err := rand.Read(b[:])
	must(err)
	return b
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

func static34ByteID(fill byte) [34]byte {
	var b [34]byte
	for i := 0; i < len(b); i++ {
		b[i] = fill
	}
	return b
}

func staticEd25519Address(fill byte) *iotago.Ed25519Address {
	b := static32ByteID(fill)
	var addr iotago.Ed25519Address
	copy(addr[:], b[:])
	return &addr
}
