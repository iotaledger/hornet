package main

import (
	"crypto/rand"
	"os"
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/snapshot"
	iotago "github.com/iotaledger/iota.go/v3"
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

var deSeriParas = &iotago.DeSerializationParameters{
	RentStructure: &iotago.RentStructure{
		VByteCost:    0,
		VBFactorData: 0,
		VBFactorKey:  0,
	},
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

var originTreasurySupply = iotago.TokenSupply - fullSnapshotOutputs[0].Deposit() - fullSnapshotOutputs[1].Deposit()

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
	},
}

func writeFullSnapshot() {
	full, err := os.Create("test_full_snapshot.bin")
	must(err)
	defer func() { _ = full.Close() }()

	var seps, sepsMax = 0, 10
	fullSnapSEPProd := func() (hornet.MessageID, error) {
		seps++
		if seps == 1 {
			return hornet.NullMessageID(), nil
		}
		if seps > sepsMax {
			return nil, nil
		}
		return randMsgID(), nil
	}

	var currentOutput int
	fullSnapOutputProd := func() (*utxo.Output, error) {
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
		Created: utxo.Outputs{
			utxoOutput(8, fullSnapshotOutputs[0].Deposit(), 4),
			utxoOutput(7, fullSnapshotOutputs[1].Deposit(), 4),
		},
		Consumed: utxo.Spents{
			utxoSpent(6, fullSnapshotOutputs[0].Deposit(), 3, 4),
			utxoSpent(5, fullSnapshotOutputs[1].Deposit(), 3, 4),
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
				Build(deSeriParas)
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
		Created: utxo.Outputs{
			utxoOutput(9, fullSnapshotOutputs[0].Deposit()+fullSnapshotOutputs[1].Deposit()+10_000_000, 5),
		},
		Consumed: utxo.Spents{
			utxoSpent(8, fullSnapshotOutputs[0].Deposit(), 4, 5),
			utxoSpent(7, fullSnapshotOutputs[1].Deposit(), 4, 5),
		},
	},
}

func writeDeltaSnapshot() {
	delta, err := os.Create("test_delta_snapshot.bin")
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

func staticMessageID(fill byte) hornet.MessageID {
	return hornet.MessageIDFromArray(static32ByteID(fill))
}

func staticOutputID(fill byte) *iotago.OutputID {
	b := &iotago.OutputID{}
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

func utxoOutput(fill byte, amount uint64, msIndex milestone.Index) *utxo.Output {
	return utxo.CreateOutput(
		staticOutputID(fill),
		staticMessageID(fill),
		msIndex,
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

func utxoSpent(fill byte, amount uint64, msIndexCreated milestone.Index, msIndexSpent milestone.Index) *utxo.Spent {
	txID := static32ByteID(fill)
	return utxo.NewSpent(utxoOutput(fill, amount, msIndexCreated), &txID, msIndexSpent, 0)
}
