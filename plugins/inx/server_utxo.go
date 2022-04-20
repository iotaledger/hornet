package inx

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/workerpool"
	inx "github.com/iotaledger/inx/go"
	iotago "github.com/iotaledger/iota.go/v3"
)

func NewLedgerOutput(o *utxo.Output) (*inx.LedgerOutput, error) {
	outputBytes, err := o.Output().Serialize(serializer.DeSeriModeNoValidation, iotago.ZeroRentParas)
	if err != nil {
		return nil, err
	}
	l := &inx.LedgerOutput{
		OutputId:                 inx.NewOutputId(o.OutputID()),
		MessageId:                inx.NewMessageId(o.MessageID().ToArray()),
		MilestoneIndexBooked:     uint32(o.MilestoneIndex()),
		MilestoneTimestampBooked: o.MilestoneTimestamp(),
		Output:                   make([]byte, len(outputBytes)),
	}
	copy(l.Output, outputBytes)
	return l, nil
}

func NewLedgerSpent(s *utxo.Spent) (*inx.LedgerSpent, error) {
	output, err := NewLedgerOutput(s.Output())
	if err != nil {
		return nil, err
	}
	transactionID := s.TargetTransactionID()
	l := &inx.LedgerSpent{
		Output:                  output,
		TransactionIdSpent:      make([]byte, len(transactionID)),
		MilestoneIndexSpent:     uint32(s.MilestoneIndex()),
		MilestoneTimestampSpent: s.MilestoneTimestamp(),
	}
	copy(l.TransactionIdSpent, transactionID[:])
	return l, nil
}

func NewLedgerUpdate(index milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents) (*inx.LedgerUpdate, error) {
	u := &inx.LedgerUpdate{
		MilestoneIndex: uint32(index),
		Created:        make([]*inx.LedgerOutput, len(newOutputs)),
		Consumed:       make([]*inx.LedgerSpent, len(newSpents)),
	}
	for i, o := range newOutputs {
		output, err := NewLedgerOutput(o)
		if err != nil {
			return nil, err
		}
		u.Created[i] = output
	}
	for i, s := range newSpents {
		spent, err := NewLedgerSpent(s)
		if err != nil {
			return nil, err
		}
		u.Consumed[i] = spent
	}
	return u, nil
}

func NewTreasuryUpdate(index milestone.Index, created *utxo.TreasuryOutput, consumed *utxo.TreasuryOutput) (*inx.TreasuryUpdate, error) {
	u := &inx.TreasuryUpdate{
		MilestoneIndex: uint32(index),
		Created: &inx.TreasuryOutput{
			MilestoneId: inx.NewMilestoneId(created.MilestoneID),
			Amount:      created.Amount,
		},
	}
	if consumed != nil {
		u.Consumed = &inx.TreasuryOutput{
			MilestoneId: inx.NewMilestoneId(consumed.MilestoneID),
			Amount:      consumed.Amount,
		}
	}
	return u, nil
}

func (s *INXServer) ReadOutput(_ context.Context, id *inx.OutputId) (*inx.OutputResponse, error) {
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	ledgerIndex, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, err
	}

	outputID := id.Unwrap()

	unspent, err := deps.UTXOManager.IsOutputIDUnspentWithoutLocking(outputID)
	if err != nil {
		return nil, err
	}

	if unspent {
		output, err := deps.UTXOManager.ReadOutputByOutputIDWithoutLocking(outputID)
		if err != nil {
			return nil, err
		}
		ledgerOutput, err := NewLedgerOutput(output)
		if err != nil {
			return nil, err
		}
		return &inx.OutputResponse{
			LedgerIndex: uint32(ledgerIndex),
			Payload: &inx.OutputResponse_Output{
				Output: ledgerOutput,
			},
		}, nil
	}

	spent, err := deps.UTXOManager.ReadSpentForOutputIDWithoutLocking(outputID)
	if err != nil {
		return nil, err
	}
	ledgerSpent, err := NewLedgerSpent(spent)
	if err != nil {
		return nil, err
	}
	return &inx.OutputResponse{
		LedgerIndex: uint32(ledgerIndex),
		Payload: &inx.OutputResponse_Spent{
			Spent: ledgerSpent,
		},
	}, nil
}

func (s *INXServer) ReadUnspentOutputs(_ *inx.NoParams, srv inx.INX_ReadUnspentOutputsServer) error {
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	ledgerIndex, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return err
	}

	var innerErr error
	err = deps.UTXOManager.ForEachUnspentOutput(func(output *utxo.Output) bool {
		ledgerOutput, err := NewLedgerOutput(output)
		if err != nil {
			innerErr = err
			return false
		}
		payload := &inx.UnspentOutput{
			LedgerIndex: uint32(ledgerIndex),
			Output:      ledgerOutput,
		}
		if err := srv.Send(payload); err != nil {
			innerErr = fmt.Errorf("send error: %w", err)
			return false
		}
		return true
	}, utxo.ReadLockLedger(false))
	if innerErr != nil {
		return innerErr
	}
	return err
}

func (s *INXServer) ListenToLedgerUpdates(req *inx.LedgerRequest, srv inx.INX_ListenToLedgerUpdatesServer) error {

	sendPreviousMilestoneDiffs := func(startIndex milestone.Index) error {
		if startIndex > 0 {
			deps.UTXOManager.ReadLockLedger()
			defer deps.UTXOManager.ReadUnlockLedger()

			// Stream all available milestone diffs first
			pruningIndex := deps.Storage.SnapshotInfo().PruningIndex
			if startIndex <= pruningIndex {
				return status.Errorf(codes.InvalidArgument, "given startMilestoneIndex %d is older than the current pruningIndex %d", startIndex, pruningIndex)
			}

			ledgerIndex, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
			if err != nil {
				return status.Error(codes.Unavailable, "error accessing the UTXO ledger")
			}
			for currentIndex := startIndex; currentIndex <= ledgerIndex; currentIndex++ {
				msDiff, err := deps.UTXOManager.MilestoneDiffWithoutLocking(currentIndex)
				if err != nil {
					return status.Errorf(codes.NotFound, "ledger update for milestoneIndex %d not found", currentIndex)
				}
				payload, err := NewLedgerUpdate(msDiff.Index, msDiff.Outputs, msDiff.Spents)
				if err != nil {
					return err
				}
				if err := srv.Send(payload); err != nil {
					return fmt.Errorf("send error: %w", err)
				}
			}
		}
		return nil
	}

	if err := sendPreviousMilestoneDiffs(milestone.Index(req.GetStartMilestoneIndex())); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		index := task.Param(0).(milestone.Index)
		newOutputs := task.Param(1).(utxo.Outputs)
		newSpents := task.Param(2).(utxo.Spents)
		payload, err := NewLedgerUpdate(index, newOutputs, newSpents)
		if err != nil {
			Plugin.LogInfof("send error: %v", err)
			cancel()
		}
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("send error: %v", err)
			cancel()
		}
		task.Return(nil)
	})
	closure := events.NewClosure(func(index milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents) {
		wp.Submit(index, newOutputs, newSpents)
	})
	wp.Start()
	deps.Tangle.Events.LedgerUpdated.Attach(closure)
	<-ctx.Done()
	deps.Tangle.Events.LedgerUpdated.Detach(closure)
	wp.Stop()
	return ctx.Err()
}

func (s *INXServer) ListenToTreasuryUpdates(req *inx.LedgerRequest, srv inx.INX_ListenToTreasuryUpdatesServer) error {
	var sentTreasuryUpdate bool
	sendPreviousMilestoneDiffs := func(startIndex milestone.Index) error {
		deps.UTXOManager.ReadLockLedger()
		defer deps.UTXOManager.ReadUnlockLedger()

		ledgerIndex, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()

		if startIndex > 0 {
			// Stream all available milestone diffs first
			pruningIndex := deps.Storage.SnapshotInfo().PruningIndex
			if startIndex <= pruningIndex {
				return status.Errorf(codes.InvalidArgument, "given startMilestoneIndex %d is older than the current pruningIndex %d", startIndex, pruningIndex)
			}

			if err != nil {
				return status.Error(codes.Unavailable, "error accessing the UTXO ledger")
			}
			for currentIndex := startIndex; currentIndex <= ledgerIndex; currentIndex++ {
				msDiff, err := deps.UTXOManager.MilestoneDiffWithoutLocking(currentIndex)
				if err != nil {
					return status.Errorf(codes.NotFound, "treasury update for milestoneIndex %d not found", currentIndex)
				}
				if msDiff.TreasuryOutput != nil {
					payload, err := NewTreasuryUpdate(msDiff.Index, msDiff.TreasuryOutput, msDiff.SpentTreasuryOutput)
					if err != nil {
						return err
					}
					if err := srv.Send(payload); err != nil {
						return fmt.Errorf("send error: %w", err)
					}
					sentTreasuryUpdate = true
				}
			}
		}
		if !sentTreasuryUpdate {
			// Since treasury mutations do not happen on every milestone, send the stored unspent output that we have
			treasuryOutput, err := deps.UTXOManager.UnspentTreasuryOutputWithoutLocking()
			if err != nil {
				return err
			}
			payload, err := NewTreasuryUpdate(ledgerIndex, treasuryOutput, treasuryOutput)
			if err != nil {
				return err
			}
			if err := srv.Send(payload); err != nil {
				return fmt.Errorf("send error: %w", err)
			}
		}
		return nil
	}

	if err := sendPreviousMilestoneDiffs(milestone.Index(req.GetStartMilestoneIndex())); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		index := task.Param(0).(milestone.Index)
		tm := task.Param(1).(*utxo.TreasuryMutationTuple)
		payload, err := NewTreasuryUpdate(index, tm.NewOutput, tm.SpentOutput)
		if err != nil {
			Plugin.LogInfof("Send error: %v", err)
			cancel()
		}
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("send error: %v", err)
			cancel()
		}
		task.Return(nil)
	})
	closure := events.NewClosure(func(index milestone.Index, tuple *utxo.TreasuryMutationTuple) {
		wp.Submit(index, tuple)
	})
	wp.Start()
	deps.Tangle.Events.TreasuryMutated.Attach(closure)
	<-ctx.Done()
	deps.Tangle.Events.TreasuryMutated.Detach(closure)
	wp.Stop()
	return ctx.Err()
}

func (s *INXServer) ListenToMigrationReceipts(_ *inx.NoParams, srv inx.INX_ListenToMigrationReceiptsServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		receipt := task.Param(0).(*iotago.ReceiptMilestoneOpt)
		payload, err := inx.WrapReceipt(receipt)
		if err != nil {
			Plugin.LogInfof("send error: %v", err)
			cancel()
		}
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("send error: %v", err)
			cancel()
		}
		task.Return(nil)
	})
	closure := events.NewClosure(func(receipt *iotago.ReceiptMilestoneOpt) {
		wp.Submit(receipt)
	})
	wp.Start()
	deps.Tangle.Events.NewReceipt.Attach(closure)
	<-ctx.Done()
	deps.Tangle.Events.NewReceipt.Detach(closure)
	wp.Stop()
	return ctx.Err()
}
