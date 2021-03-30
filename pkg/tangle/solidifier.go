package tangle

import (
	"bytes"
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

const (
	solidifierThreshold = 60 * time.Second

	milestoneSolidifierWorkerCount = 2 // must be two, so a new request can abort another, in case it is an older milestone
	milestoneSolidifierQueueSize   = 2
)

var (
	ErrMilestoneNotFound     = errors.New("milestone not found")
	ErrDivisionByZero        = errors.New("division by zero")
	ErrMissingMilestoneFound = errors.New("missing milestone found")
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

func (t *Tangle) markMessageAsSolid(cachedMetadata *storage.CachedMetadata) {
	defer cachedMetadata.Release(true)

	// update the solidity flags of this message
	cachedMetadata.GetMetadata().SetSolid(true)

	t.Events.MessageSolid.Trigger(cachedMetadata)
	t.messageSolidSyncEvent.Trigger(cachedMetadata.GetMetadata().GetMessageID().ToMapKey())
}

// SolidQueueCheck traverses a milestone and checks if it is solid.
// Missing messages are requested.
// Can be aborted with abortSignal.
// metadataMemcache has to be cleaned up outside.
func (t *Tangle) SolidQueueCheck(
	messagesMemcache *storage.MessagesMemcache,
	metadataMemcache *storage.MetadataMemcache,
	milestoneIndex milestone.Index,
	parents hornet.MessageIDs,
	abortSignal chan struct{}) (solid bool, aborted bool) {

	ts := time.Now()

	msgsChecked := 0
	var messageIDsToSolidify hornet.MessageIDs
	messageIDsToRequest := make(map[string]struct{})

	// we don't need to call cleanup at the end, because we pass our own metadataMemcache.
	parentsTraverser := dag.NewParentTraverser(t.storage, abortSignal, metadataMemcache)

	// collect all msg to solidify by traversing the tangle
	if err := parentsTraverser.Traverse(parents,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			// if the msg is solid, there is no need to traverse its parents
			return !cachedMsgMeta.GetMetadata().IsSolid(), nil
		},
		// consumer
		func(cachedMsgMeta *storage.CachedMetadata) error { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			// mark the msg as checked
			msgsChecked++

			// collect the txToSolidify in an ordered way
			messageIDsToSolidify = append(messageIDsToSolidify, cachedMsgMeta.GetMetadata().GetMessageID())

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
		if err == common.ErrOperationAborted {
			return false, true
		}
		t.log.Panic(err)
	}

	tCollect := time.Now()

	if len(messageIDsToRequest) > 0 {
		messageIDs := make(hornet.MessageIDs, 0, len(messageIDsToRequest))
		for messageID := range messageIDsToRequest {
			messageIDs = append(messageIDs, hornet.MessageIDFromMapKey(messageID))
		}
		requested := t.requester.RequestMultiple(messageIDs, milestoneIndex, true)
		t.log.Warnf("Stopped solidifier due to missing msg -> Requested missing msgs (%d/%d), collect: %v", requested, len(messageIDs), tCollect.Sub(ts).Truncate(time.Millisecond))
		return false, false
	}

	// no messages to request => the whole cone is solid
	// we mark all messages as solid in order from oldest to latest (needed for the tip pool)
	for _, messageID := range messageIDsToSolidify {
		cachedMsgMeta := metadataMemcache.GetCachedMetadataOrNil(messageID)
		if cachedMsgMeta == nil {
			t.log.Panicf("solidQueueCheck: Message metadata not found: %v", messageID.ToHex())
		}

		t.markMessageAsSolid(cachedMsgMeta.Retain())
	}

	tSolid := time.Now()

	if t.storage.IsNodeAlmostSynced() {
		// propagate solidity to the future cone (msgs attached to the msgs of this milestone)
		t.solidifyFutureCone(messagesMemcache, metadataMemcache, messageIDsToSolidify, abortSignal)
	}

	t.log.Infof("Solidifier finished: msgs: %d, collect: %v, solidity %v, propagation: %v, total: %v", msgsChecked, tCollect.Sub(ts).Truncate(time.Millisecond), tSolid.Sub(tCollect).Truncate(time.Millisecond), time.Since(tSolid).Truncate(time.Millisecond), time.Since(ts).Truncate(time.Millisecond))
	return true, false
}

// SolidifyFutureConeOfMsg updates the solidity of the future cone (messages approving the given message).
// we have to walk the future cone, if a message became newly solid during the walk.
func (t *Tangle) SolidifyFutureConeOfMsg(cachedMsgMeta *storage.CachedMetadata) error {
	// normal solidification could be part of a cone of old milestones while synching => no need to keep this in cache
	defer cachedMsgMeta.Release(true)

	messagesMemcache := storage.NewMessagesMemcache(t.storage)
	metadataMemcache := storage.NewMetadataMemcache(t.storage)
	defer messagesMemcache.Cleanup(true)
	defer metadataMemcache.Cleanup(true)

	return t.solidifyFutureCone(messagesMemcache, metadataMemcache, hornet.MessageIDs{cachedMsgMeta.GetMetadata().GetMessageID()}, nil)
}

// solidifyFutureCone updates the solidity of the future cone (messages approving the given messages).
// we have to walk the future cone, if a message became newly solid during the walk.
// metadataMemcache has to be cleaned up outside.
func (t *Tangle) solidifyFutureCone(messageMemcache *storage.MessagesMemcache, metadataMemcache *storage.MetadataMemcache, messageIDs hornet.MessageIDs, abortSignal chan struct{}) error {

	// we do not cleanup the traverser to not cleanup the MetadataMemcache
	childrenTraverser := dag.NewChildrenTraverser(t.storage, abortSignal, metadataMemcache)

	for _, messageID := range messageIDs {

		startMessageID := messageID

		if err := childrenTraverser.Traverse(messageID,
			// traversal stops if no more messages pass the given condition
			func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1

				if cachedMsgMeta.GetMetadata().IsSolid() && !bytes.Equal(startMessageID, cachedMsgMeta.GetMetadata().GetMessageID()) {
					// do not walk the future cone if the current message is already solid, except it was the startTx
					return false, nil
				}

				// check if current message is solid by checking the solidity of its parents
				for _, parentMessageID := range cachedMsgMeta.GetMetadata().GetParents() {
					if t.storage.SolidEntryPointsContain(parentMessageID) {
						// Ignore solid entry points (snapshot milestone included)
						continue
					}

					cachedParentMsgMeta := metadataMemcache.GetCachedMetadataOrNil(parentMessageID) // meta +1
					if cachedParentMsgMeta == nil {
						// parent is missing => message is not solid
						// do not walk the future cone if the current message is not solid
						return false, nil
					}

					if !cachedParentMsgMeta.GetMetadata().IsSolid() {
						// parent is not solid => message is not solid
						// do not walk the future cone if the current message is not solid
						return false, nil
					}
				}

				// mark current message as solid
				t.markMessageAsSolid(cachedMsgMeta.Retain())

				// walk the future cone since the message got newly solid
				return true, nil
			},
			// consumer
			// no need to consume here
			nil,
			true); err != nil {
			return err
		}
	}
	return nil
}

func (t *Tangle) AbortMilestoneSolidification() {
	t.signalChanMilestoneStopSolidificationLock.Lock()
	if t.signalChanMilestoneStopSolidification != nil {
		close(t.signalChanMilestoneStopSolidification)
		t.signalChanMilestoneStopSolidification = nil
	}
	t.signalChanMilestoneStopSolidificationLock.Unlock()
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
		nextMilestoneSignal := newMilestoneIndex == t.storage.GetConfirmedMilestoneIndex()+1
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

	currentConfirmedIndex := t.storage.GetConfirmedMilestoneIndex()
	latestIndex := t.storage.GetLatestMilestoneIndex()

	if currentConfirmedIndex == latestIndex && latestIndex != 0 {
		// Latest milestone already solid
		return
	}

	// Always traverse the oldest non-solid milestone, either it gets solid, or something is missing that should be requested.
	cachedMsToSolidify := t.storage.FindClosestNextMilestoneOrNil(currentConfirmedIndex) // message +1
	if cachedMsToSolidify == nil {
		// No newer milestone available
		return
	}

	// Release shouldn't be forced, to cache the latest milestones
	defer cachedMsToSolidify.Release() // message -1

	milestoneIndexToSolidify := cachedMsToSolidify.GetMilestone().Index
	t.setSolidifierMilestoneIndex(milestoneIndexToSolidify)

	t.signalChanMilestoneStopSolidificationLock.Lock()
	t.signalChanMilestoneStopSolidification = make(chan struct{})
	t.signalChanMilestoneStopSolidificationLock.Unlock()

	messagesMemcache := storage.NewMessagesMemcache(t.storage)
	metadataMemcache := storage.NewMetadataMemcache(t.storage)

	// release all messages at the end
	defer messagesMemcache.Cleanup(true)
	defer metadataMemcache.Cleanup(true)

	t.log.Infof("Run solidity check for Milestone (%d)...", milestoneIndexToSolidify)
	if becameSolid, aborted := t.SolidQueueCheck(messagesMemcache, metadataMemcache, milestoneIndexToSolidify, hornet.MessageIDs{cachedMsToSolidify.GetMilestone().MessageID}, t.signalChanMilestoneStopSolidification); !becameSolid { // meta pass +1
		if aborted {
			// check was aborted due to older milestones/other solidifier running
			t.log.Infof("Aborted solid queue check for milestone %d", milestoneIndexToSolidify)
		} else {
			// Milestone not solid yet and missing msg were requested
			t.Events.MilestoneSolidificationFailed.Trigger(milestoneIndexToSolidify)
			t.log.Infof("Milestone couldn't be solidified! %d", milestoneIndexToSolidify)
		}
		t.setSolidifierMilestoneIndex(0)
		return
	}

	if (currentConfirmedIndex + 1) < milestoneIndexToSolidify {

		// Milestone is stable, but some Milestones are missing in between
		// => check if they were found, or search for them in the solidified cone
		cachedClosestNextMs := t.storage.FindClosestNextMilestoneOrNil(currentConfirmedIndex) // message +1
		cachedClosestNextMsIndex := cachedClosestNextMs.GetMilestone().Index
		if cachedClosestNextMsIndex == milestoneIndexToSolidify {
			t.log.Infof("Milestones missing between (%d) and (%d). Search for missing milestones...", currentConfirmedIndex, cachedClosestNextMsIndex)

			// no Milestones found in between => search an older milestone in the solid cone
			if found, err := t.searchMissingMilestone(currentConfirmedIndex, cachedClosestNextMsIndex, cachedMsToSolidify.GetMilestone().MessageID, t.signalChanMilestoneStopSolidification); !found {
				if err != nil {
					// no milestones found => this should not happen!
					t.log.Panicf("Milestones missing between (%d) and (%d).", currentConfirmedIndex, cachedClosestNextMsIndex)
				}
				t.log.Infof("Aborted search for missing milestones between (%d) and (%d).", currentConfirmedIndex, cachedClosestNextMsIndex)
			}
		}
		cachedClosestNextMs.Release() // message -1

		// rerun to solidify the older one
		t.setSolidifierMilestoneIndex(0)

		t.milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), true)
		return
	}

	var timeStartConfirmation, timeSetConfirmedMilestoneIndex, timeConfirmedMilestoneChanged, timeConfirmedMilestoneIndexChanged, timeMilestoneConfirmedSyncEvent, timeMilestoneConfirmed time.Time

	timeStart := time.Now()
	confirmedMilestoneStats, confirmationMetrics, err := whiteflag.ConfirmMilestone(t.storage, t.serverMetrics, messagesMemcache, metadataMemcache, cachedMsToSolidify.GetMilestone().MessageID,
		func(msgMeta *storage.CachedMetadata, index milestone.Index, confTime uint64) {
			t.Events.MessageReferenced.Trigger(msgMeta, index, confTime)
		},
		func(confirmation *whiteflag.Confirmation) {
			timeStartConfirmation = time.Now()
			t.storage.SetConfirmedMilestoneIndex(milestoneIndexToSolidify)
			timeSetConfirmedMilestoneIndex = time.Now()
			t.Events.ConfirmedMilestoneChanged.Trigger(cachedMsToSolidify) // milestone pass +1
			timeConfirmedMilestoneChanged = time.Now()
			t.Events.ConfirmedMilestoneIndexChanged.Trigger(milestoneIndexToSolidify)
			timeConfirmedMilestoneIndexChanged = time.Now()
			t.milestoneConfirmedSyncEvent.Trigger(milestoneIndexToSolidify)
			timeMilestoneConfirmedSyncEvent = time.Now()
			t.Events.MilestoneConfirmed.Trigger(confirmation)
			timeMilestoneConfirmed = time.Now()
		},
		func(output *utxo.Output) {
			t.Events.NewUTXOOutput.Trigger(output)
		},
		func(spent *utxo.Spent) {
			t.Events.NewUTXOSpent.Trigger(spent)
		},
		func(rt *utxo.ReceiptTuple) error {
			if t.receiptService != nil {
				if t.receiptService.ValidationEnabled {
					if err := t.receiptService.Validate(rt.Receipt); err != nil {
						if err := common.IsSoftError(err); err != nil && t.receiptService.IgnoreSoftErrors {
							t.log.Warnf("soft error encountered during receipt validation: %s", err.Error())
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
		t.log.Panic(err)
	}

	t.log.Infof("Milestone confirmed (%d): txsReferenced: %v, txsValue: %v, txsZeroValue: %v, txsConflicting: %v, collect: %v, total: %v",
		confirmedMilestoneStats.Index,
		confirmedMilestoneStats.MessagesReferenced,
		confirmedMilestoneStats.MessagesIncludedWithTransactions,
		confirmedMilestoneStats.MessagesExcludedWithoutTransactions,
		confirmedMilestoneStats.MessagesExcludedWithConflictingTransactions,
		confirmationMetrics.DurationWhiteflag.Truncate(time.Millisecond),
		time.Since(timeStart).Truncate(time.Millisecond),
	)

	confirmationMetrics.DurationSetConfirmedMilestoneIndex = timeSetConfirmedMilestoneIndex.Sub(timeStartConfirmation)
	confirmationMetrics.DurationConfirmedMilestoneChanged = timeConfirmedMilestoneChanged.Sub(timeSetConfirmedMilestoneIndex)
	confirmationMetrics.DurationConfirmedMilestoneIndexChanged = timeConfirmedMilestoneIndexChanged.Sub(timeConfirmedMilestoneChanged)
	confirmationMetrics.DurationMilestoneConfirmedSyncEvent = timeMilestoneConfirmedSyncEvent.Sub(timeConfirmedMilestoneIndexChanged)
	confirmationMetrics.DurationMilestoneConfirmed = timeMilestoneConfirmed.Sub(timeMilestoneConfirmedSyncEvent)
	confirmationMetrics.DurationTotal = time.Since(timeStart)

	t.Events.ConfirmationMetricsUpdated.Trigger(confirmationMetrics)

	var rmpsMessage string
	if metric, err := t.getConfirmedMilestoneMetric(cachedMsToSolidify.Retain(), confirmedMilestoneStats.Index); err == nil {
		if t.storage.IsNodeSynced() {
			// Only trigger the metrics event if the node is sync (otherwise the MPS and conf.rate is wrong)
			if t.firstSyncedMilestone == 0 {
				t.firstSyncedMilestone = confirmedMilestoneStats.Index
			}
		} else {
			// reset the variable if unsynced
			t.firstSyncedMilestone = 0
		}

		if t.storage.IsNodeSynced() && (confirmedMilestoneStats.Index > t.firstSyncedMilestone+1) {
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

	t.log.Infof("New confirmed milestone: %d%s", confirmedMilestoneStats.Index, rmpsMessage)

	// Run check for next milestone
	t.setSolidifierMilestoneIndex(0)

	if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
		// do not trigger the next solidification if the node was shut down
		return
	}

	t.milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), false)
}

func (t *Tangle) getConfirmedMilestoneMetric(cachedMilestone *storage.CachedMilestone, milestoneIndexToSolidify milestone.Index) (*ConfirmedMilestoneMetric, error) {
	defer cachedMilestone.Release(true)

	oldMilestone := t.storage.GetCachedMilestoneOrNil(milestoneIndexToSolidify - 1) // milestone +1
	if oldMilestone == nil {
		return nil, ErrMilestoneNotFound
	}
	defer oldMilestone.Release(true) // milestone -1

	timeDiff := cachedMilestone.GetMilestone().Timestamp.Sub(oldMilestone.GetMilestone().Timestamp).Seconds()
	if timeDiff == 0 {
		return nil, ErrDivisionByZero
	}

	newNewMsgCount := t.serverMetrics.NewMessages.Load()
	newMsgDiff := utils.GetUint32Diff(newNewMsgCount, t.oldNewMsgCount)
	t.oldNewMsgCount = newNewMsgCount

	newReferencedMsgCount := t.serverMetrics.ReferencedMessages.Load()
	referencedMsgDiff := utils.GetUint32Diff(newReferencedMsgCount, t.oldReferencedMsgCount)
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

// searchMissingMilestone searches milestones in the cone that are not persisted in the DB yet by traversing the tangle
func (t *Tangle) searchMissingMilestone(confirmedMilestoneIndex milestone.Index, startMilestoneIndex milestone.Index, milestoneMessageID hornet.MessageID, abortSignal chan struct{}) (found bool, err error) {

	var milestoneFound bool

	ts := time.Now()

	if err := dag.TraverseParentsOfMessage(t.storage, milestoneMessageID,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			// if the message is referenced by an older milestone, there is no need to traverse its parents
			if referenced, at := cachedMsgMeta.GetMetadata().GetReferenced(); referenced && (at <= confirmedMilestoneIndex) {
				return false, nil
			}

			return true, nil
		},
		// consumer
		func(cachedMsgMeta *storage.CachedMetadata) error { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			cachedMessage := t.storage.GetCachedMessageOrNil(cachedMsgMeta.GetMetadata().GetMessageID()) // message +1
			if cachedMessage == nil {
				return fmt.Errorf("%w message ID: %s", common.ErrMessageNotFound, cachedMsgMeta.GetMetadata().GetMessageID().ToHex())
			}
			defer cachedMessage.Release(true) // message -1

			ms := t.storage.VerifyMilestone(cachedMessage.GetMessage())
			if ms == nil {
				return nil
			}

			msIndex := milestone.Index(ms.Index)
			if (msIndex <= confirmedMilestoneIndex) || (msIndex >= startMilestoneIndex) {
				return nil
			}

			// milestone found!
			t.storage.StoreMilestone(cachedMessage.Retain(), ms)

			return ErrMissingMilestoneFound // we return this as an error to stop the traverser
		},
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false, abortSignal); err != nil {
		if err == common.ErrOperationAborted {
			return false, nil
		} else if err == ErrMissingMilestoneFound {
			milestoneFound = true
		} else {
			return false, err
		}
	}

	t.log.Infof("searchMissingMilestone finished, found: %v, took: %v", milestoneFound, time.Since(ts).Truncate(time.Millisecond))
	return milestoneFound, nil
}
