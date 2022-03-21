package tangle

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

var (
	ErrMilestoneNotFound = errors.New("milestone not found")
	ErrDivisionByZero    = errors.New("division by zero")
)

type ConfirmedMilestoneMetric struct {
	MilestoneIndex         milestone.Index `json:"ms_index"`
	MPS                    float64         `json:"mps"`
	RMPS                   float64         `json:"rmps"`
	ReferencedRate         float64         `json:"referenced_rate"`
	TimeSinceLastMilestone float64         `json:"time_since_last_ms"`
}

// TriggerSolidifier can be used to manually trigger the solidifier from other plugins.
func (t *Tangle) TriggerSolidifier() {
	t.milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), true)
}

func (t *Tangle) markMessageAsSolid(cachedMsgMeta *storage.CachedMetadata) {
	defer cachedMsgMeta.Release(true) // meta -1

	// update the solidity flags of this message
	cachedMsgMeta.Metadata().SetSolid(true)

	t.Events.MessageSolid.Trigger(cachedMsgMeta)
	t.messageSolidSyncEvent.Trigger(cachedMsgMeta.Metadata().MessageID().ToMapKey())
}

// SolidQueueCheck traverses a milestone and checks if it is solid.
// Missing messages are requested.
// Can be aborted if the given context is canceled.
func (t *Tangle) SolidQueueCheck(
	ctx context.Context,
	memcachedTraverserStorage dag.TraverserStorage,
	milestoneIndex milestone.Index,
	parents hornet.MessageIDs) (solid bool, aborted bool) {

	ts := time.Now()

	msgsChecked := 0
	var messageIDsToSolidify hornet.MessageIDs
	messageIDsToRequest := make(map[string]struct{})

	parentsTraverser := dag.NewParentsTraverser(memcachedTraverserStorage)

	// collect all msg to solidify by traversing the tangle
	if err := parentsTraverser.Traverse(
		ctx,
		parents,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			// if the msg is solid, there is no need to traverse its parents
			return !cachedMsgMeta.Metadata().IsSolid(), nil
		},
		// consumer
		func(cachedMsgMeta *storage.CachedMetadata) error { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			// mark the msg as checked
			msgsChecked++

			// collect the txToSolidify in an ordered way
			messageIDsToSolidify = append(messageIDsToSolidify, cachedMsgMeta.Metadata().MessageID())

			return nil
		},
		// called on missing parents
		func(parentMessageID hornet.MessageID) error {
			// msg does not exist => request missing msg
			messageIDsToRequest[parentMessageID.ToMapKey()] = struct{}{}
			return nil
		},
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false); err != nil {
		if errors.Is(err, common.ErrOperationAborted) {
			return false, true
		}
		t.LogPanic(err)
	}

	tCollect := time.Now()

	if len(messageIDsToRequest) > 0 {
		messageIDs := make(hornet.MessageIDs, 0, len(messageIDsToRequest))
		for messageID := range messageIDsToRequest {
			messageIDs = append(messageIDs, hornet.MessageIDFromMapKey(messageID))
		}
		requested := t.requester.RequestMultiple(messageIDs, milestoneIndex, true)
		t.LogWarnf("Stopped solidifier due to missing msg -> Requested missing msgs (%d/%d), collect: %v", requested, len(messageIDs), tCollect.Sub(ts).Truncate(time.Millisecond))
		return false, false
	}

	// no messages to request => the whole cone is solid
	// we mark all messages as solid in order from oldest to latest (needed for the tip pool)
	for _, messageID := range messageIDsToSolidify {
		cachedMsgMeta, err := memcachedTraverserStorage.CachedMessageMetadata(messageID)
		if err != nil {
			t.LogPanicf("solidQueueCheck: Get message metadata failed: %v, Error: %w", messageID.ToHex(), err)
			return
		}
		if cachedMsgMeta == nil {
			t.LogPanicf("solidQueueCheck: Message metadata not found: %v", messageID.ToHex())
			return
		}

		t.markMessageAsSolid(cachedMsgMeta.Retain()) // meta pass +1
		cachedMsgMeta.Release(true)                  // meta -1
	}

	tSolid := time.Now()

	if t.syncManager.IsNodeAlmostSynced() {
		// propagate solidity to the future cone (msgs attached to the msgs of this milestone)
		if err := t.futureConeSolidifier.SolidifyFutureConesWithMetadataMemcache(ctx, memcachedTraverserStorage, messageIDsToSolidify); err != nil {
			t.LogDebugf("SolidifyFutureConesWithMetadataMemcache failed: %s", err)
		}
	}

	t.LogInfof("Solidifier finished: msgs: %d, collect: %v, solidity %v, propagation: %v, total: %v", msgsChecked, tCollect.Sub(ts).Truncate(time.Millisecond), tSolid.Sub(tCollect).Truncate(time.Millisecond), time.Since(tSolid).Truncate(time.Millisecond), time.Since(ts).Truncate(time.Millisecond))
	return true, false
}

func (t *Tangle) newMilestoneSolidificationCtx() (context.Context, context.CancelFunc) {
	t.milestoneSolidificationCtxLock.Lock()
	defer t.milestoneSolidificationCtxLock.Unlock()

	// milestone solidification can be canceled by node shutdown or external signal
	ctx, ctxCancel := context.WithCancel(t.shutdownCtx)
	t.milestoneSolidificationCancelFunc = ctxCancel
	return ctx, ctxCancel
}

func (t *Tangle) AbortMilestoneSolidification() {
	t.milestoneSolidificationCtxLock.Lock()
	defer t.milestoneSolidificationCtxLock.Unlock()

	if t.milestoneSolidificationCancelFunc != nil {
		// cancel ongoing solidifications
		t.milestoneSolidificationCancelFunc()
		t.milestoneSolidificationCancelFunc = nil
	}
}

// solidifyMilestone tries to solidify the next known non-solid milestone and requests missing msg
func (t *Tangle) solidifyMilestone(newMilestoneIndex milestone.Index, force bool) {

	/* How milestone solidification works:

	- A Milestone comes in and gets validated
	- Request milestone parent1/parent2 without traversion
	- Everytime a request queue gets empty, start the solidifier for the next known non-solid milestone
	- If msg are missing, they are requested by the solidifier
	- The traversion can be aborted with a signal and restarted
	*/
	if !force {
		/*
			If solidification is not forced, we will only run the solidifier under one of the following conditions:
				- newMilestoneIndex==0 (triggersignal) and solidifierMilestoneIndex==0 (no ongoing solidification)
				- newMilestoneIndex==solidMilestoneIndex+1 (next milestone)
				- newMilestoneIndex!=0 (new milestone received) and solidifierMilestoneIndex!=0 (ongoing solidification) and newMilestoneIndex<solidifierMilestoneIndex (milestone older than ongoing solidification)
				- newMilestoneIndex!=0 (new milestone received) and solidifierMilestoneIndex==0 (no ongoing solidification) and RequestQueue().Empty() (request queue is already empty)

			The following events trigger the solidifier in the node:
				- new valid milestone was processed (newMilestoneIndex=index, force=false)
				- a milestone was missing in the cone at solidifier run (newMilestoneIndex=0, force=true)
				- WebAPI call (newMilestoneIndex=0, force=true)
				- milestones in warp sync range were already in database at warpsync startup (newMilestoneIndex==0, force=true)
				- another milestone was successfully solidified (newMilestoneIndex=0, force=false)
				- request queue gets empty and node is not synced (newMilestoneIndex=0, force=true)
		*/

		t.solidifierMilestoneIndexLock.RLock()
		triggerSignal := (newMilestoneIndex == 0) && (t.solidifierMilestoneIndex == 0)
		nextMilestoneSignal := newMilestoneIndex == t.syncManager.ConfirmedMilestoneIndex()+1
		olderMilestoneDetected := (newMilestoneIndex != 0) && ((t.solidifierMilestoneIndex != 0) && (newMilestoneIndex < t.solidifierMilestoneIndex))
		newMilestoneReqQueueEmptySignal := (t.solidifierMilestoneIndex == 0) && (newMilestoneIndex != 0) && t.requestQueue.Empty()
		if !(triggerSignal || nextMilestoneSignal || olderMilestoneDetected || newMilestoneReqQueueEmptySignal) {
			// Do not run solidifier
			t.solidifierMilestoneIndexLock.RUnlock()
			return
		}
		t.solidifierMilestoneIndexLock.RUnlock()
	}

	// Stop possible other newer solidifications
	t.AbortMilestoneSolidification()

	t.solidifierLock.Lock()
	defer t.solidifierLock.Unlock()

	currentConfirmedIndex := t.syncManager.ConfirmedMilestoneIndex()
	latestIndex := t.syncManager.LatestMilestoneIndex()

	if currentConfirmedIndex == latestIndex && latestIndex != 0 {
		// Latest milestone already solid
		return
	}

	// Always traverse the oldest non-solid milestone, either it gets solid, or something is missing that should be requested.
	cachedMilestoneToSolidify := t.milestoneManager.FindClosestNextMilestoneOrNil(currentConfirmedIndex) // milestone +1
	if cachedMilestoneToSolidify == nil {
		// No newer milestone available
		return
	}

	// Release shouldn't be forced, to cache the latest milestones
	defer cachedMilestoneToSolidify.Release() // milestone -1

	milestoneIndexToSolidify := cachedMilestoneToSolidify.Milestone().Index
	t.setSolidifierMilestoneIndex(milestoneIndexToSolidify)

	milestoneSolidificationCtx, milestoneSolidificationCancelFunc := t.newMilestoneSolidificationCtx()
	defer milestoneSolidificationCancelFunc()

	messagesMemcache := storage.NewMessagesMemcache(t.storage.CachedMessage)
	metadataMemcache := storage.NewMetadataMemcache(t.storage.CachedMessageMetadata)
	memcachedTraverserStorage := dag.NewMemcachedTraverserStorage(t.storage, metadataMemcache)

	defer func() {
		// all releases are forced since the cone is referenced and not needed anymore
		memcachedTraverserStorage.Cleanup(true)

		// release all messages at the end
		messagesMemcache.Cleanup(true)

		// Release all message metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	t.LogInfof("Run solidity check for Milestone (%d)...", milestoneIndexToSolidify)
	if becameSolid, aborted := t.SolidQueueCheck(milestoneSolidificationCtx, memcachedTraverserStorage, milestoneIndexToSolidify, hornet.MessageIDs{cachedMilestoneToSolidify.Milestone().MessageID}); !becameSolid { // meta pass +1
		if aborted {
			// check was aborted due to older milestones/other solidifier running
			t.LogInfof("Aborted solid queue check for milestone %d", milestoneIndexToSolidify)
		} else {
			// Milestone not solid yet and missing msg were requested
			t.Events.MilestoneSolidificationFailed.Trigger(milestoneIndexToSolidify)
			t.LogInfof("Milestone couldn't be solidified! %d", milestoneIndexToSolidify)
		}
		t.setSolidifierMilestoneIndex(0)
		return
	}

	if (currentConfirmedIndex + 1) < milestoneIndexToSolidify {

		// Milestone is stable, but some Milestones are missing in between
		// => check if they were found, or search for them in the solidified cone
		cachedMilestoneClosestNext := t.milestoneManager.FindClosestNextMilestoneOrNil(currentConfirmedIndex) // milestone +1
		milestoneIndexClosestNext := cachedMilestoneClosestNext.Milestone().Index
		if milestoneIndexClosestNext == milestoneIndexToSolidify {
			t.LogInfof("Milestones missing between (%d) and (%d). Search for missing milestones...", currentConfirmedIndex, milestoneIndexClosestNext)

			// no Milestones found in between => search an older milestone in the solid cone
			if found, err := t.searchMissingMilestones(milestoneSolidificationCtx, currentConfirmedIndex, milestoneIndexClosestNext, cachedMilestoneToSolidify.Milestone().MessageID); !found {
				if err != nil {
					// no milestones found => this should not happen!
					t.LogPanicf("Milestones missing between (%d) and (%d).", currentConfirmedIndex, milestoneIndexClosestNext)
				}
				t.LogInfof("Aborted search for missing milestones between (%d) and (%d).", currentConfirmedIndex, milestoneIndexClosestNext)
			}
		}
		cachedMilestoneClosestNext.Release() // milestone -1

		// rerun to solidify the older one
		t.setSolidifierMilestoneIndex(0)

		t.milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), true)
		return
	}

	var timeStartConfirmation, timeSetConfirmedMilestoneIndex, timeUpdateConeRootIndexes, timeConfirmedMilestoneChanged, timeConfirmedMilestoneIndexChanged, timeMilestoneConfirmedSyncEvent, timeMilestoneConfirmed time.Time

	timeStart := time.Now()
	confirmedMilestoneStats, confirmationMetrics, err := whiteflag.ConfirmMilestone(
		t.storage.UTXOManager(),
		memcachedTraverserStorage,
		messagesMemcache.CachedMessage,
		cachedMilestoneToSolidify.Milestone().MessageID,
		whiteflag.DefaultWhiteFlagTraversalCondition,
		whiteflag.DefaultCheckMessageReferencedFunc,
		whiteflag.DefaultSetMessageReferencedFunc,
		t.serverMetrics,
		func(msgMeta *storage.CachedMetadata, index milestone.Index, confTime uint64) {
			t.Events.MessageReferenced.Trigger(msgMeta, index, confTime)
		},
		func(confirmation *whiteflag.Confirmation) {
			timeStartConfirmation = time.Now()
			if err := t.syncManager.SetConfirmedMilestoneIndex(milestoneIndexToSolidify); err != nil {
				t.LogPanicf("SetConfirmedMilestoneIndex failed: %s", err)
			}
			timeSetConfirmedMilestoneIndex = time.Now()
			if t.syncManager.IsNodeAlmostSynced() {
				// propagate new cone root indexes to the future cone (needed for URTS, heaviest branch tipselection, message broadcasting, etc...)
				// we can safely ignore errors of the future cone solidifier.
				_ = dag.UpdateConeRootIndexes(milestoneSolidificationCtx, memcachedTraverserStorage, confirmation.Mutations.MessagesReferenced, confirmation.MilestoneIndex)
			}
			timeUpdateConeRootIndexes = time.Now()
			t.Events.ConfirmedMilestoneChanged.Trigger(cachedMilestoneToSolidify) // milestone pass +1
			timeConfirmedMilestoneChanged = time.Now()
			t.Events.ConfirmedMilestoneIndexChanged.Trigger(milestoneIndexToSolidify)
			timeConfirmedMilestoneIndexChanged = time.Now()
			t.milestoneConfirmedSyncEvent.Trigger(milestoneIndexToSolidify)
			timeMilestoneConfirmedSyncEvent = time.Now()
			t.Events.MilestoneConfirmed.Trigger(confirmation)
			timeMilestoneConfirmed = time.Now()
		},
		func(index milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents) {
			t.Events.LedgerUpdated.Trigger(index, newOutputs, newSpents)
		},
		func(index milestone.Index, output *utxo.Output) {
			t.Events.NewUTXOOutput.Trigger(index, output)
		},
		func(index milestone.Index, spent *utxo.Spent) {
			t.Events.NewUTXOSpent.Trigger(index, spent)
		},
		func(rt *utxo.ReceiptTuple) error {
			if t.receiptService != nil {
				if t.receiptService.ValidationEnabled {
					if err := t.receiptService.ValidateWithoutLocking(rt.Receipt); err != nil {
						if err := common.IsSoftError(err); err != nil && t.receiptService.IgnoreSoftErrors {
							t.LogWarnf("soft error encountered during receipt validation: %s", err)
							return nil
						}
						return err
					}
				}
				if t.receiptService.BackupEnabled {
					if err := t.receiptService.Backup(rt); err != nil {
						return fmt.Errorf("unable to confirm milestone due to receipt backup failure: %w", err)
					}
				}
			}
			t.Events.NewReceipt.Trigger(rt.Receipt)
			return nil
		})

	if err != nil {
		t.LogPanic(err)
	}

	t.LogInfof("Milestone confirmed (%d): txsReferenced: %v, txsValue: %v, txsZeroValue: %v, txsConflicting: %v, collect: %v, total: %v",
		confirmedMilestoneStats.Index,
		confirmedMilestoneStats.MessagesReferenced,
		confirmedMilestoneStats.MessagesIncludedWithTransactions,
		confirmedMilestoneStats.MessagesExcludedWithoutTransactions,
		confirmedMilestoneStats.MessagesExcludedWithConflictingTransactions,
		confirmationMetrics.DurationWhiteflag.Truncate(time.Millisecond),
		time.Since(timeStart).Truncate(time.Millisecond),
	)

	confirmationMetrics.DurationSetConfirmedMilestoneIndex = timeSetConfirmedMilestoneIndex.Sub(timeStartConfirmation)
	confirmationMetrics.DurationUpdateConeRootIndexes = timeUpdateConeRootIndexes.Sub(timeSetConfirmedMilestoneIndex)
	confirmationMetrics.DurationConfirmedMilestoneChanged = timeConfirmedMilestoneChanged.Sub(timeUpdateConeRootIndexes)
	confirmationMetrics.DurationConfirmedMilestoneIndexChanged = timeConfirmedMilestoneIndexChanged.Sub(timeConfirmedMilestoneChanged)
	confirmationMetrics.DurationMilestoneConfirmedSyncEvent = timeMilestoneConfirmedSyncEvent.Sub(timeConfirmedMilestoneIndexChanged)
	confirmationMetrics.DurationMilestoneConfirmed = timeMilestoneConfirmed.Sub(timeMilestoneConfirmedSyncEvent)
	confirmationMetrics.DurationTotal = time.Since(timeStart)

	t.Events.ConfirmationMetricsUpdated.Trigger(confirmationMetrics)

	var rmpsMessage string
	if metric, err := t.calcConfirmedMilestoneMetric(cachedMilestoneToSolidify.Retain(), confirmedMilestoneStats.Index); err == nil { // milestone pass +1
		if t.syncManager.IsNodeSynced() {
			// Only trigger the metrics event if the node is sync (otherwise the MPS and conf.rate is wrong)
			if t.firstSyncedMilestone == 0 {
				t.firstSyncedMilestone = confirmedMilestoneStats.Index
			}
		} else {
			// reset the variable if unsynced
			t.firstSyncedMilestone = 0
		}

		if t.syncManager.IsNodeSynced() && (confirmedMilestoneStats.Index > t.firstSyncedMilestone+1) {
			t.lastConfirmedMilestoneMetricLock.Lock()
			t.lastConfirmedMilestoneMetric = metric
			t.lastConfirmedMilestoneMetricLock.Unlock()

			// Ignore the first two milestones after node was sync (otherwise the MPS and conf.rate is wrong)
			rmpsMessage = fmt.Sprintf(", %0.2f MPS, %0.2f RMPS, %0.2f%% ref.rate", metric.MPS, metric.RMPS, metric.ReferencedRate)
			t.Events.NewConfirmedMilestoneMetric.Trigger(metric)
		} else {
			rmpsMessage = fmt.Sprintf(", %0.2f RMPS", metric.RMPS)
		}
	}

	t.LogInfof("New confirmed milestone: %d%s", confirmedMilestoneStats.Index, rmpsMessage)

	// Run check for next milestone
	t.setSolidifierMilestoneIndex(0)

	if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
		// do not trigger the next solidification if the node was shut down
		return
	}

	t.milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), false)
}

func (t *Tangle) calcConfirmedMilestoneMetric(cachedMilestone *storage.CachedMilestone, milestoneIndexToSolidify milestone.Index) (*ConfirmedMilestoneMetric, error) {
	defer cachedMilestone.Release(true) // milestone -1

	cachedMilestoneOld := t.storage.CachedMilestoneOrNil(milestoneIndexToSolidify - 1) // milestone +1
	if cachedMilestoneOld == nil {
		return nil, ErrMilestoneNotFound
	}
	defer cachedMilestoneOld.Release(true) // milestone -1

	timeDiff := cachedMilestone.Milestone().Timestamp.Sub(cachedMilestoneOld.Milestone().Timestamp).Seconds()
	if timeDiff == 0 {
		return nil, ErrDivisionByZero
	}

	newNewMsgCount := t.serverMetrics.NewMessages.Load()
	newMsgDiff := utils.Uint32Diff(newNewMsgCount, t.oldNewMsgCount)
	t.oldNewMsgCount = newNewMsgCount

	newReferencedMsgCount := t.serverMetrics.ReferencedMessages.Load()
	referencedMsgDiff := utils.Uint32Diff(newReferencedMsgCount, t.oldReferencedMsgCount)
	t.oldReferencedMsgCount = newReferencedMsgCount

	referencedRate := 0.0
	if newMsgDiff != 0 {
		referencedRate = (float64(referencedMsgDiff) / float64(newMsgDiff)) * 100.0
	}

	metric := &ConfirmedMilestoneMetric{
		MilestoneIndex:         milestoneIndexToSolidify,
		MPS:                    float64(newMsgDiff) / timeDiff,
		RMPS:                   float64(referencedMsgDiff) / timeDiff,
		ReferencedRate:         referencedRate,
		TimeSinceLastMilestone: timeDiff,
	}

	return metric, nil
}

func (t *Tangle) setSolidifierMilestoneIndex(index milestone.Index) {
	t.solidifierMilestoneIndexLock.Lock()
	t.solidifierMilestoneIndex = index
	t.solidifierMilestoneIndexLock.Unlock()
}

// searchMissingMilestones searches milestones in the cone that are not persisted in the DB yet by traversing the tangle
func (t *Tangle) searchMissingMilestones(ctx context.Context, confirmedMilestoneIndex milestone.Index, startMilestoneIndex milestone.Index, milestoneMessageID hornet.MessageID) (found bool, err error) {

	var milestoneFound bool

	ts := time.Now()

	if err := dag.TraverseParentsOfMessage(
		ctx,
		t.storage,
		milestoneMessageID,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			// if the message is referenced by an older milestone, there is no need to traverse its parents
			if referenced, at := cachedMsgMeta.Metadata().ReferencedWithIndex(); referenced && (at <= confirmedMilestoneIndex) {
				return false, nil
			}

			cachedMsg := t.storage.CachedMessageOrNil(cachedMsgMeta.Metadata().MessageID()) // message +1
			if cachedMsg == nil {
				return false, fmt.Errorf("%w message ID: %s", common.ErrMessageNotFound, cachedMsgMeta.Metadata().MessageID().ToHex())
			}
			defer cachedMsg.Release(true) // message -1

			ms := t.milestoneManager.VerifyMilestone(cachedMsg.Message())
			if ms == nil {
				return true, nil
			}

			msIndex := milestone.Index(ms.Index)
			if (msIndex <= confirmedMilestoneIndex) || (msIndex >= startMilestoneIndex) {
				return true, nil
			}

			// milestone found!
			t.milestoneManager.StoreMilestone(cachedMsg.Retain(), ms, false) // message pass +1

			milestoneFound = true
			return true, nil // we keep searching for all missing milestones
		},
		// consumer
		nil,
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false); err != nil {
		if errors.Is(err, common.ErrOperationAborted) {
			return false, nil
		} else {
			return false, err
		}
	}

	t.LogInfof("searchMissingMilestone finished, found: %v, took: %v", milestoneFound, time.Since(ts).Truncate(time.Millisecond))
	return milestoneFound, nil
}
