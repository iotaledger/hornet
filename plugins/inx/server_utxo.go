package inx

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	inx "github.com/iotaledger/inx/go"
	iotago "github.com/iotaledger/iota.go/v3"
)

func NewLedgerOutput(o *utxo.Output) (*inx.LedgerOutput, error) {
	output, err := inx.WrapOutput(o.Output())
	if err != nil {
		return nil, err
	}
	return &inx.LedgerOutput{
		OutputId:                 inx.NewOutputId(o.OutputID()),
		BlockId:                  inx.NewBlockId(o.BlockID()),
		MilestoneIndexBooked:     uint32(o.MilestoneIndex()),
		MilestoneTimestampBooked: o.MilestoneTimestamp(),
		Output:                   output,
	}, nil
}

func NewLedgerSpent(s *utxo.Spent) (*inx.LedgerSpent, error) {
	output, err := NewLedgerOutput(s.Output())
	if err != nil {
		return nil, err
	}
	l := &inx.LedgerSpent{
		Output:                  output,
		TransactionIdSpent:      inx.NewTransactionId(s.TargetTransactionID()),
		MilestoneIndexSpent:     uint32(s.MilestoneIndex()),
		MilestoneTimestampSpent: s.MilestoneTimestamp(),
	}

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

func (s *INXServer) ListenToLedgerUpdates(req *inx.MilestoneRangeRequest, srv inx.INX_ListenToLedgerUpdatesServer) error {

	createLedgerUpdatePayloadAndSend := func(msIndex milestone.Index, outputs utxo.Outputs, spents utxo.Spents) error {
		payload, err := NewLedgerUpdate(msIndex, outputs, spents)
		if err != nil {
			return err
		}
		if err := srv.Send(payload); err != nil {
			return fmt.Errorf("send error: %w", err)
		}
		return nil
	}

	sendMilestoneDiffsRange := func(startIndex milestone.Index, endIndex milestone.Index) error {
		for currentIndex := startIndex; currentIndex <= endIndex; currentIndex++ {
			msDiff, err := deps.UTXOManager.MilestoneDiff(currentIndex)
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
	// if no startIndex is given, but an endIndex, we do not send previous milestone diffs.
	sendPreviousMilestoneDiffs := func(startIndex milestone.Index, endIndex milestone.Index) (milestone.Index, error) {
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
		pruningIndex := deps.Storage.SnapshotInfo().PruningIndex
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

	requestStartIndex := milestone.Index(req.GetStartMilestoneIndex())
	requestEndIndex := milestone.Index(req.GetEndMilestoneIndex())

	lastSentIndex, err := sendPreviousMilestoneDiffs(requestStartIndex, requestEndIndex)
	if err != nil {
		return err
	}

	if requestEndIndex > 0 && lastSentIndex >= requestEndIndex {
		// We are done sending, so close the stream
		return nil
	}

	var innerErr error
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		index := task.Param(0).(milestone.Index)

		if requestStartIndex > 0 && index < requestStartIndex {
			// Skip this because it is before the index we requested
			task.Return(nil)
			return
		}

		if requestStartIndex > 0 && index-1 > lastSentIndex {
			// we missed some ledger updates in between
			startIndex := requestStartIndex
			if startIndex < lastSentIndex+1 {
				startIndex = lastSentIndex + 1
			}

			endIndex := index - 1
			if requestEndIndex > 0 && endIndex > requestEndIndex {
				endIndex = requestEndIndex
			}

			if err := sendMilestoneDiffsRange(startIndex, endIndex); err != nil {
				Plugin.LogInfof("sendMilestoneDiffsRange error: %v", err)
				innerErr = err
				cancel()
				task.Return(nil)
				return
			}

			// remember the last index we sent
			lastSentIndex = endIndex
		}

		if requestEndIndex > 0 && index > requestEndIndex {
			// We are done sending the updates
			innerErr = nil
			cancel()
			task.Return(nil)
			return
		}

		newOutputs := task.Param(1).(utxo.Outputs)
		newSpents := task.Param(2).(utxo.Spents)

		if err := createLedgerUpdatePayloadAndSend(index, newOutputs, newSpents); err != nil {
			Plugin.LogInfof("send error: %v", err)
			innerErr = err
			cancel()
			task.Return(nil)
			return
		}

		if requestEndIndex > 0 && index >= requestEndIndex {
			// We are done sending the updates
			innerErr = nil
			cancel()
		}

		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	closure := events.NewClosure(func(index milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents) {
		wp.Submit(index, newOutputs, newSpents)
	})

	wp.Start()
	deps.Tangle.Events.LedgerUpdated.Attach(closure)
	<-ctx.Done()
	deps.Tangle.Events.LedgerUpdated.Detach(closure)
	wp.Stop()

	return innerErr
}

func (s *INXServer) ListenToTreasuryUpdates(req *inx.MilestoneRangeRequest, srv inx.INX_ListenToTreasuryUpdatesServer) error {

	var treasuryUpdateSent bool

	createTreasuryUpdatePayloadAndSend := func(msIndex milestone.Index, treasuryOutput *utxo.TreasuryOutput, spentTreasuryOutput *utxo.TreasuryOutput) error {
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

	sendTreasuryUpdatesRange := func(startIndex milestone.Index, endIndex milestone.Index) error {
		for currentIndex := startIndex; currentIndex <= endIndex; currentIndex++ {
			msDiff, err := deps.UTXOManager.MilestoneDiff(currentIndex)
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
	// if no startIndex is given, but an endIndex, we do not send previous treasury updates.
	sendPreviousTreasuryUpdates := func(startIndex milestone.Index, endIndex milestone.Index) (milestone.Index, error) {
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
		pruningIndex := deps.Storage.SnapshotInfo().PruningIndex
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

	sendCurrentTreasuryOutput := func() (milestone.Index, error) {

		getCurrentTreasuryOutputAndIndex := func() (milestone.Index, *utxo.TreasuryOutput, error) {
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

	requestStartIndex := milestone.Index(req.GetStartMilestoneIndex())
	requestEndIndex := milestone.Index(req.GetEndMilestoneIndex())

	lastSentIndex, err := sendPreviousTreasuryUpdates(requestStartIndex, requestEndIndex)
	if err != nil {
		return err
	}

	if !treasuryUpdateSent {
		// Since treasury mutations do not happen on every milestone, send the stored unspent output that we have
		ledgerIndex, err := sendCurrentTreasuryOutput()
		if err != nil {
			return err
		}
		lastSentIndex = ledgerIndex
	}

	if requestEndIndex > 0 && lastSentIndex >= requestEndIndex {
		// We are done sending, so close the stream
		return nil
	}

	var innerErr error
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		index := task.Param(0).(milestone.Index)

		if requestStartIndex > 0 && index < requestStartIndex {
			// Skip this because it is before the index we requested
			task.Return(nil)
			return
		}

		if requestStartIndex > 0 && index-1 > lastSentIndex {
			// we missed some treasury updates in between
			startIndex := requestStartIndex
			if startIndex < lastSentIndex+1 {
				startIndex = lastSentIndex + 1
			}

			endIndex := index - 1
			if requestEndIndex > 0 && endIndex > requestEndIndex {
				endIndex = requestEndIndex
			}

			if err := sendTreasuryUpdatesRange(startIndex, endIndex); err != nil {
				Plugin.LogInfof("sendTreasuryUpdatesRange error: %v", err)
				innerErr = err
				cancel()
				task.Return(nil)
				return
			}

			// remember the last index we sent
			lastSentIndex = endIndex
		}

		if requestEndIndex > 0 && index > requestEndIndex {
			// We are done sending the updates
			innerErr = nil
			cancel()
			task.Return(nil)
			return
		}

		tm := task.Param(1).(*utxo.TreasuryMutationTuple)

		if err := createTreasuryUpdatePayloadAndSend(index, tm.NewOutput, tm.SpentOutput); err != nil {
			Plugin.LogInfof("send error: %v", err)
			innerErr = err
			cancel()
			task.Return(nil)
			return
		}

		if requestEndIndex > 0 && index >= requestEndIndex {
			// We are done sending the updates
			innerErr = nil
			cancel()
		}

		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	closure := events.NewClosure(func(index milestone.Index, tuple *utxo.TreasuryMutationTuple) {
		wp.Submit(index, tuple)
	})

	wp.Start()
	deps.Tangle.Events.TreasuryMutated.Attach(closure)
	<-ctx.Done()
	deps.Tangle.Events.TreasuryMutated.Detach(closure)
	wp.Stop()

	return innerErr
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
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))
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
