package inx

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/workerpool"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	inx "github.com/iotaledger/inx/go"
	iotago "github.com/iotaledger/iota.go/v3"
)

func NewLedgerOutput(o *utxo.Output) (*inx.LedgerOutput, error) {
	return &inx.LedgerOutput{
		OutputId:                 inx.NewOutputId(o.OutputID()),
		BlockId:                  inx.NewBlockId(o.BlockID()),
		MilestoneIndexBooked:     o.MilestoneIndexBooked(),
		MilestoneTimestampBooked: o.MilestoneTimestampBooked(),
		Output: &inx.RawOutput{
			Data: o.Bytes(),
		},
	}, nil
}

func NewLedgerSpent(s *utxo.Spent) (*inx.LedgerSpent, error) {
	output, err := NewLedgerOutput(s.Output())
	if err != nil {
		return nil, err
	}

	l := &inx.LedgerSpent{
		Output:                  output,
		TransactionIdSpent:      inx.NewTransactionId(s.TransactionIDSpent()),
		MilestoneIndexSpent:     s.MilestoneIndexSpent(),
		MilestoneTimestampSpent: s.MilestoneTimestampSpent(),
	}

	return l, nil
}

func NewLedgerUpdateBatchBegin(index iotago.MilestoneIndex, newOutputsCount int, newSpentsCount int) *inx.LedgerUpdate {
	return &inx.LedgerUpdate{
		Op: &inx.LedgerUpdate_BatchMarker{
			BatchMarker: &inx.LedgerUpdate_Marker{
				MilestoneIndex: index,
				MarkerType:     inx.LedgerUpdate_Marker_BEGIN,
				CreatedCount:   uint32(newOutputsCount),
				ConsumedCount:  uint32(newSpentsCount),
			},
		},
	}
}

func NewLedgerUpdateBatchEnd(index iotago.MilestoneIndex, newOutputsCount int, newSpentsCount int) *inx.LedgerUpdate {
	return &inx.LedgerUpdate{
		Op: &inx.LedgerUpdate_BatchMarker{
			BatchMarker: &inx.LedgerUpdate_Marker{
				MilestoneIndex: index,
				MarkerType:     inx.LedgerUpdate_Marker_END,
				CreatedCount:   uint32(newOutputsCount),
				ConsumedCount:  uint32(newSpentsCount),
			},
		},
	}
}

func NewLedgerUpdateBatchOperationCreated(output *utxo.Output) (*inx.LedgerUpdate, error) {
	o, err := NewLedgerOutput(output)
	if err != nil {
		return nil, err
	}

	return &inx.LedgerUpdate{
		Op: &inx.LedgerUpdate_Created{
			Created: o,
		},
	}, nil
}

func NewLedgerUpdateBatchOperationConsumed(spent *utxo.Spent) (*inx.LedgerUpdate, error) {
	s, err := NewLedgerSpent(spent)
	if err != nil {
		return nil, err
	}

	return &inx.LedgerUpdate{
		Op: &inx.LedgerUpdate_Consumed{
			Consumed: s,
		},
	}, nil
}

func NewTreasuryUpdate(index iotago.MilestoneIndex, created *utxo.TreasuryOutput, consumed *utxo.TreasuryOutput) (*inx.TreasuryUpdate, error) {
	u := &inx.TreasuryUpdate{
		MilestoneIndex: index,
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

func (s *Server) ReadOutput(_ context.Context, id *inx.OutputId) (*inx.OutputResponse, error) {
	// we need to lock the ledger here to have the correct index for unspent info of the output.
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
			LedgerIndex: ledgerIndex,
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
		LedgerIndex: ledgerIndex,
		Payload: &inx.OutputResponse_Spent{
			Spent: ledgerSpent,
		},
	}, nil
}

func (s *Server) ReadUnspentOutputs(_ *inx.NoParams, srv inx.INX_ReadUnspentOutputsServer) error {
	// we need to lock the ledger here to have the correct index for unspent info of the output.
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
			LedgerIndex: ledgerIndex,
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

func (s *Server) ListenToLedgerUpdates(req *inx.MilestoneRangeRequest, srv inx.INX_ListenToLedgerUpdatesServer) error {

	snapshotInfo := deps.Storage.SnapshotInfo()
	if snapshotInfo == nil {
		return common.ErrSnapshotInfoNotFound
	}

	createLedgerUpdatePayloadAndSend := func(msIndex iotago.MilestoneIndex, outputs utxo.Outputs, spents utxo.Spents) error {

		// Send Begin
		if err := srv.Send(NewLedgerUpdateBatchBegin(msIndex, len(outputs), len(spents))); err != nil {
			return fmt.Errorf("send error: %w", err)
		}

		// Send consumed
		for _, spent := range spents {
			payload, err := NewLedgerUpdateBatchOperationConsumed(spent)
			if err != nil {
				return err
			}

			if err := srv.Send(payload); err != nil {
				return fmt.Errorf("send error: %w", err)
			}
		}

		// Send created
		for _, output := range outputs {
			payload, err := NewLedgerUpdateBatchOperationCreated(output)
			if err != nil {
				return err
			}

			if err := srv.Send(payload); err != nil {
				return fmt.Errorf("send error: %w", err)
			}
		}

		// Send End
		if err := srv.Send(NewLedgerUpdateBatchEnd(msIndex, len(outputs), len(spents))); err != nil {
			return fmt.Errorf("send error: %w", err)
		}

		return nil
	}

	sendMilestoneDiffsRange := func(startIndex iotago.MilestoneIndex, endIndex iotago.MilestoneIndex) error {
		for currentIndex := startIndex; currentIndex <= endIndex; currentIndex++ {
			msDiff, err := deps.UTXOManager.MilestoneDiffWithoutLocking(currentIndex)
			if err != nil {
				return status.Errorf(codes.NotFound, "ledger update for milestoneIndex %d not found", currentIndex)
			}

			if err := createLedgerUpdatePayloadAndSend(msDiff.Index, msDiff.Outputs, msDiff.Spents); err != nil {
				return err
			}
		}

		return nil
	}

	// if a startIndex is given, we send all available milestone diffs including the start index.
	// if an endIndex is given, we send all available milestone diffs up to and including min(ledgerIndex, endIndex).
	// if no startIndex is given, but an endIndex, we don't send previous milestone diffs.
	sendPreviousMilestoneDiffs := func(startIndex iotago.MilestoneIndex, endIndex iotago.MilestoneIndex) (iotago.MilestoneIndex, error) {
		if startIndex == 0 {
			// no need to send previous milestone diffs
			return 0, nil
		}

		ledgerIndex, err := deps.UTXOManager.ReadLedgerIndex()
		if err != nil {
			return 0, status.Error(codes.Unavailable, "error accessing the UTXO ledger")
		}

		if startIndex > ledgerIndex {
			// no need to send previous milestone diffs
			return 0, nil
		}

		// Stream all available milestone diffs first
		pruningIndex := snapshotInfo.PruningIndex()
		if startIndex <= pruningIndex {
			return 0, status.Errorf(codes.InvalidArgument, "given startMilestoneIndex %d is older than the current pruningIndex %d", startIndex, pruningIndex)
		}

		if endIndex == 0 || endIndex > ledgerIndex {
			endIndex = ledgerIndex
		}

		if err := sendMilestoneDiffsRange(startIndex, endIndex); err != nil {
			return 0, err
		}

		return endIndex, nil
	}

	stream := &streamRange{
		start: req.GetStartMilestoneIndex(),
		end:   req.GetEndMilestoneIndex(),
	}

	var err error
	stream.lastSent, err = sendPreviousMilestoneDiffs(stream.start, stream.end)
	if err != nil {
		return err
	}

	if stream.isBounded() && stream.lastSent >= stream.end {
		// We are done sending, so close the stream
		return nil
	}

	catchUpFunc := func(start iotago.MilestoneIndex, end iotago.MilestoneIndex) error {
		if err := sendMilestoneDiffsRange(start, end); err != nil {
			Plugin.LogErrorf("sendMilestoneDiffsRange error: %v", err)

			return err
		}

		return nil
	}

	sendFunc := func(task *workerpool.Task, index iotago.MilestoneIndex) error {
		newOutputs, ok := task.Param(1).(utxo.Outputs)
		if !ok {
			err := fmt.Errorf("expected utxo.Outputs, got %T", task.Param(1))
			Plugin.LogErrorf("send error: %v", err)

			return err
		}

		newSpents, ok := task.Param(2).(utxo.Spents)
		if !ok {
			err := fmt.Errorf("expected utxo.Spents, got %T", task.Param(2))
			Plugin.LogErrorf("send error: %v", err)

			return err
		}

		if err := createLedgerUpdatePayloadAndSend(index, newOutputs, newSpents); err != nil {
			Plugin.LogErrorf("send error: %v", err)

			return err
		}

		return nil
	}

	var innerErr error
	ctx, cancel := context.WithCancel(Plugin.Daemon().ContextStopped())

	wp := workerpool.New(func(task workerpool.Task) {
		defer task.Return(nil)

		done, err := handleRangedSend(&task, task.Param(0).(iotago.MilestoneIndex), stream, catchUpFunc, sendFunc)
		switch {
		case err != nil:
			innerErr = err
			cancel()

		case done:
			cancel()
		}

	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	onLedgerUpdated := events.NewClosure(func(index iotago.MilestoneIndex, newOutputs utxo.Outputs, newSpents utxo.Spents) {
		wp.Submit(index, newOutputs, newSpents)
	})

	wp.Start()
	deps.Tangle.Events.LedgerUpdated.Hook(onLedgerUpdated)
	<-ctx.Done()
	deps.Tangle.Events.LedgerUpdated.Detach(onLedgerUpdated)

	// We need to wait until all tasks are done, otherwise we might call
	// "SendMsg" and "CloseSend" in parallel on the grpc stream, which is
	// not safe according to the grpc docs.
	wp.StopAndWait()

	return innerErr
}

func (s *Server) ListenToTreasuryUpdates(req *inx.MilestoneRangeRequest, srv inx.INX_ListenToTreasuryUpdatesServer) error {

	snapshotInfo := deps.Storage.SnapshotInfo()
	if snapshotInfo == nil {
		return common.ErrSnapshotInfoNotFound
	}

	var treasuryUpdateSent bool

	createTreasuryUpdatePayloadAndSend := func(msIndex iotago.MilestoneIndex, treasuryOutput *utxo.TreasuryOutput, spentTreasuryOutput *utxo.TreasuryOutput) error {
		if treasuryOutput != nil {
			payload, err := NewTreasuryUpdate(msIndex, treasuryOutput, spentTreasuryOutput)
			if err != nil {
				return err
			}

			if err := srv.Send(payload); err != nil {
				return fmt.Errorf("send error: %w", err)
			}

			treasuryUpdateSent = true
		}

		return nil
	}

	sendTreasuryUpdatesRange := func(startIndex iotago.MilestoneIndex, endIndex iotago.MilestoneIndex) error {
		for currentIndex := startIndex; currentIndex <= endIndex; currentIndex++ {
			msDiff, err := deps.UTXOManager.MilestoneDiffWithoutLocking(currentIndex)
			if err != nil {
				return status.Errorf(codes.NotFound, "ledger update for milestoneIndex %d not found", currentIndex)
			}

			if err := createTreasuryUpdatePayloadAndSend(msDiff.Index, msDiff.TreasuryOutput, msDiff.SpentTreasuryOutput); err != nil {
				return err
			}
		}

		return nil
	}

	// if a startIndex is given, we send all available treasury updates including the start index.
	// if an endIndex is given, we send all available treasury updates up to and including min(ledgerIndex, endIndex).
	// if no startIndex is given, but an endIndex, we don't send previous treasury updates.
	sendPreviousTreasuryUpdates := func(startIndex iotago.MilestoneIndex, endIndex iotago.MilestoneIndex) (iotago.MilestoneIndex, error) {
		if startIndex == 0 {
			// no need to send treasury updates diffs
			return 0, nil
		}

		ledgerIndex, err := deps.UTXOManager.ReadLedgerIndex()
		if err != nil {
			return 0, status.Error(codes.Unavailable, "error accessing the UTXO ledger")
		}

		if startIndex > ledgerIndex {
			// no need to send treasury updates diffs
			return 0, nil
		}

		// Stream all available milestone diffs first
		pruningIndex := snapshotInfo.PruningIndex()
		if startIndex <= pruningIndex {
			return 0, status.Errorf(codes.InvalidArgument, "given startMilestoneIndex %d is older than the current pruningIndex %d", startIndex, pruningIndex)
		}

		if endIndex == 0 || endIndex > ledgerIndex {
			endIndex = ledgerIndex
		}

		if err := sendTreasuryUpdatesRange(startIndex, endIndex); err != nil {
			return 0, err
		}

		return endIndex, nil
	}

	sendCurrentTreasuryOutput := func() (iotago.MilestoneIndex, error) {

		getCurrentTreasuryOutputAndIndex := func() (iotago.MilestoneIndex, *utxo.TreasuryOutput, error) {
			deps.UTXOManager.ReadLockLedger()
			defer deps.UTXOManager.ReadUnlockLedger()

			ledgerIndex, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
			if err != nil {
				return 0, nil, status.Error(codes.Unavailable, "error accessing the UTXO ledger")
			}

			treasuryOutput, err := deps.UTXOManager.UnspentTreasuryOutputWithoutLocking()
			if err != nil {
				return 0, nil, status.Errorf(codes.Unavailable, "error accessing the UTXO ledger %s", err)
			}

			return ledgerIndex, treasuryOutput, nil
		}

		ledgerIndex, treasuryOutput, err := getCurrentTreasuryOutputAndIndex()
		if err != nil {
			return 0, err
		}

		if err := createTreasuryUpdatePayloadAndSend(ledgerIndex, treasuryOutput, treasuryOutput); err != nil {
			return 0, err
		}

		return ledgerIndex, nil
	}

	stream := &streamRange{
		start: req.GetStartMilestoneIndex(),
		end:   req.GetEndMilestoneIndex(),
	}

	var err error
	stream.lastSent, err = sendPreviousTreasuryUpdates(stream.start, stream.end)
	if err != nil {
		return err
	}

	if !treasuryUpdateSent {
		// Since treasury mutations do not happen on every milestone, send the stored unspent output that we have
		ledgerIndex, err := sendCurrentTreasuryOutput()
		if err != nil {
			return err
		}

		stream.lastSent = ledgerIndex
	}

	if stream.isBounded() && stream.lastSent >= stream.end {
		// We are done sending, so close the stream
		return nil
	}

	catchUpFunc := func(start iotago.MilestoneIndex, end iotago.MilestoneIndex) error {
		err := sendTreasuryUpdatesRange(start, end)
		if err != nil {
			Plugin.LogErrorf("sendTreasuryUpdatesRange error: %v", err)
		}

		return err
	}

	sendFunc := func(task *workerpool.Task, index iotago.MilestoneIndex) error {
		tm, ok := task.Param(1).(*utxo.TreasuryMutationTuple)
		if !ok {
			err := fmt.Errorf("expected *utxo.TreasuryMutationTuple, got %T", task.Param(1))
			Plugin.LogErrorf("send error: %v", err)

			return err
		}

		if err := createTreasuryUpdatePayloadAndSend(index, tm.NewOutput, tm.SpentOutput); err != nil {
			Plugin.LogErrorf("send error: %v", err)

			return err
		}

		return nil
	}

	var innerErr error
	ctx, cancel := context.WithCancel(Plugin.Daemon().ContextStopped())

	wp := workerpool.New(func(task workerpool.Task) {
		defer task.Return(nil)

		done, err := handleRangedSend(&task, task.Param(0).(iotago.MilestoneIndex), stream, catchUpFunc, sendFunc)
		switch {
		case err != nil:
			innerErr = err
			cancel()

		case done:
			cancel()
		}

	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	onTreasuryMutated := events.NewClosure(func(index iotago.MilestoneIndex, tuple *utxo.TreasuryMutationTuple) {
		wp.Submit(index, tuple)
	})

	wp.Start()
	deps.Tangle.Events.TreasuryMutated.Hook(onTreasuryMutated)
	<-ctx.Done()
	deps.Tangle.Events.TreasuryMutated.Detach(onTreasuryMutated)

	// We need to wait until all tasks are done, otherwise we might call
	// "SendMsg" and "CloseSend" in parallel on the grpc stream, which is
	// not safe according to the grpc docs.
	wp.StopAndWait()

	return innerErr
}

func (s *Server) ListenToMigrationReceipts(_ *inx.NoParams, srv inx.INX_ListenToMigrationReceiptsServer) error {
	ctx, cancel := context.WithCancel(Plugin.Daemon().ContextStopped())

	wp := workerpool.New(func(task workerpool.Task) {
		defer task.Return(nil)

		receipt, ok := task.Param(0).(*iotago.ReceiptMilestoneOpt)
		if !ok {
			Plugin.LogErrorf("send error: expected *iotago.ReceiptMilestoneOpt, got %T", task.Param(0))
			cancel()

			return
		}

		payload, err := inx.WrapReceipt(receipt)
		if err != nil {
			Plugin.LogErrorf("serialize error: %v", err)
			cancel()

			return
		}

		if err := srv.Send(payload); err != nil {
			Plugin.LogErrorf("send error: %v", err)
			cancel()
		}

	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	onNewReceipt := events.NewClosure(func(receipt *iotago.ReceiptMilestoneOpt) {
		wp.Submit(receipt)
	})

	wp.Start()
	deps.Tangle.Events.NewReceipt.Hook(onNewReceipt)
	<-ctx.Done()
	deps.Tangle.Events.NewReceipt.Detach(onNewReceipt)

	// We need to wait until all tasks are done, otherwise we might call
	// "SendMsg" and "CloseSend" in parallel on the grpc stream, which is
	// not safe according to the grpc docs.
	wp.StopAndWait()

	return ctx.Err()
}
