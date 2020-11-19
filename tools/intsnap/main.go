package main

import (
	"crypto/rand"
	"os"
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/snapshot"
	iotago "github.com/iotaledger/iota.go"
)

func main() {
	writeFullSnapshot()
	writeDeltaSnapshot()
}

var fullSnapshotHeader = &snapshot.FileHeader{
	Version:              snapshot.SupportedFormatVersion,
	Type:                 snapshot.Full,
	NetworkID:            iotago.NetworkIDFromString("alphanet1"),
	SEPMilestoneIndex:    1,
	LedgerMilestoneIndex: 3,
}

var fullSnapshotOutputs = []*snapshot.Output{
	{
		MessageID: static32ByteID(6),
		OutputID:  static34ByteID(6),
		Address:   staticEd25519Address(6),
		Amount:    1000,
	},
	{
		MessageID: static32ByteID(5),
		OutputID:  static34ByteID(5),
		Address:   staticEd25519Address(5),
		Amount:    iotago.TokenSupply - 1000,
	},
}

var fullSnapshotMsDiffs = []*snapshot.MilestoneDiff{
	{
		MilestoneIndex: 3,
		Created: []*snapshot.Output{
			{
				MessageID: static32ByteID(6),
				OutputID:  static34ByteID(6),
				Address:   staticEd25519Address(6),
				Amount:    fullSnapshotOutputs[0].Amount,
			},
			{
				MessageID: static32ByteID(5),
				OutputID:  static34ByteID(5),
				Address:   staticEd25519Address(5),
				Amount:    fullSnapshotOutputs[1].Amount,
			},
		},
		Consumed: []*snapshot.Spent{
			{
				Output: snapshot.Output{
					MessageID: static32ByteID(4),
					OutputID:  static34ByteID(4),
					Address:   staticEd25519Address(4),
					Amount:    fullSnapshotOutputs[0].Amount,
				},
				TargetTransactionID: static32ByteID(4),
			},
			{
				Output: snapshot.Output{
					MessageID: static32ByteID(3),
					OutputID:  static34ByteID(3),
					Address:   staticEd25519Address(3),
					Amount:    fullSnapshotOutputs[1].Amount,
				},
				TargetTransactionID: static32ByteID(3),
			},
		},
	},
	{
		MilestoneIndex: 2,
		Created: []*snapshot.Output{
			{
				MessageID: static32ByteID(4),
				OutputID:  static34ByteID(4),
				Address:   staticEd25519Address(4),
				Amount:    fullSnapshotOutputs[0].Amount,
			},
			{
				MessageID: static32ByteID(3),
				OutputID:  static34ByteID(3),
				Address:   staticEd25519Address(3),
				Amount:    fullSnapshotOutputs[1].Amount,
			},
		},
		Consumed: []*snapshot.Spent{
			{
				Output: snapshot.Output{
					MessageID: static32ByteID(2),
					OutputID:  static34ByteID(2),
					Address:   staticEd25519Address(2),
					Amount:    fullSnapshotOutputs[0].Amount,
				},
				TargetTransactionID: static32ByteID(2),
			},
			{
				Output: snapshot.Output{
					MessageID: static32ByteID(1),
					OutputID:  static34ByteID(1),
					Address:   staticEd25519Address(1),
					Amount:    fullSnapshotOutputs[1].Amount,
				},
				TargetTransactionID: static32ByteID(1),
			},
		},
	},
}

func writeFullSnapshot() {
	full, err := os.Create("full_snapshot.bin")
	must(err)
	defer full.Close()

	var seps, sepsMax = 0, 10
	fullSnapSEPProd := func() (*hornet.MessageID, error) {
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

	must(snapshot.StreamSnapshotDataTo(full, uint64(time.Now().Unix()), fullSnapshotHeader, fullSnapSEPProd, fullSnapOutputProd, fullSnapMsDiffProd))
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
		MilestoneIndex: 4,
		Created: []*snapshot.Output{
			{
				MessageID: static32ByteID(8),
				OutputID:  static34ByteID(8),
				Address:   staticEd25519Address(8),
				Amount:    fullSnapshotOutputs[0].Amount,
			},
			{
				MessageID: static32ByteID(7),
				OutputID:  static34ByteID(7),
				Address:   staticEd25519Address(7),
				Amount:    fullSnapshotOutputs[1].Amount,
			},
		},
		Consumed: []*snapshot.Spent{
			{
				Output: snapshot.Output{
					MessageID: static32ByteID(6),
					OutputID:  static34ByteID(6),
					Address:   staticEd25519Address(6),
					Amount:    fullSnapshotOutputs[0].Amount,
				},
				TargetTransactionID: static32ByteID(6),
			},
			{
				Output: snapshot.Output{
					MessageID: static32ByteID(5),
					OutputID:  static34ByteID(5),
					Address:   staticEd25519Address(5),
					Amount:    fullSnapshotOutputs[1].Amount,
				},
				TargetTransactionID: static32ByteID(5),
			},
		},
	},
	{
		MilestoneIndex: 5,
		Created: []*snapshot.Output{
			{
				MessageID: static32ByteID(9),
				OutputID:  static34ByteID(9),
				Address:   staticEd25519Address(9),
				Amount:    iotago.TokenSupply,
			},
		},
		Consumed: []*snapshot.Spent{
			{
				Output: snapshot.Output{
					MessageID: static32ByteID(8),
					OutputID:  static34ByteID(8),
					Address:   staticEd25519Address(8),
					Amount:    fullSnapshotOutputs[0].Amount,
				},
				TargetTransactionID: static32ByteID(7),
			},
			{
				Output: snapshot.Output{
					MessageID: static32ByteID(7),
					OutputID:  static34ByteID(7),
					Address:   staticEd25519Address(7),
					Amount:    fullSnapshotOutputs[1].Amount,
				},
				TargetTransactionID: static32ByteID(7),
			},
		},
	},
}

func writeDeltaSnapshot() {
	delta, err := os.Create("delta_snapshot.bin")
	must(err)
	defer delta.Close()

	var seps, sepsMax = 0, 10
	deltaSnapSEPProd := func() (*hornet.MessageID, error) {
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

	must(snapshot.StreamSnapshotDataTo(delta, uint64(time.Now().Unix()), deltaSnapshotHeader, deltaSnapSEPProd, nil, deltaSnapMsDiffProd))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func randMsgID() *hornet.MessageID {
	var b iotago.MessageID
	_, err := rand.Read(b[:])
	must(err)
	bb := hornet.MessageID(b)
	return &bb
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
