package tangle

import (
	"errors"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/gohornet/hornet/plugins/gossip"
)

const (
	solidifierThreshold = 60 * time.Second
)

var (
	milestoneSolidifierWorkerCount = 2 // must be two, so a new request can abort another, in case it is an older milestone
	milestoneSolidifierQueueSize   = 2
	milestoneSolidifierWorkerPool  *workerpool.WorkerPool

	signalChanMilestoneStopSolidification     chan struct{}
	signalChanMilestoneStopSolidificationLock syncutils.Mutex

	solidifierMilestoneIndex     milestone.Index = 0
	solidifierMilestoneIndexLock syncutils.RWMutex

	solidifierLock syncutils.RWMutex

	oldNewMsgCount       uint32
	oldConfirmedMsgCount uint32

	// Index of the first milestone that was sync after node start
	firstSyncedMilestone = milestone.Index(0)

	ErrMilestoneNotFound     = errors.New("milestone not found")
	ErrDivisionByZero        = errors.New("division by zero")
	ErrMissingMilestoneFound = errors.New("missing milestone found")
)

type ConfirmedMilestoneMetric struct {
	MilestoneIndex         milestone.Index `json:"ms_index"`
	MPS                    float64         `json:"tps"`
	CMPS                   float64         `json:"ctps"`
	ConfirmationRate       float64         `json:"conf_rate"`
	TimeSinceLastMilestone float64         `json:"time_since_last_ms"`
}

// TriggerSolidifier can be used to manually trigger the solidifier from other plugins.
func TriggerSolidifier() {
	milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), true)
}

func markMessageAsSolid(cachedMetadata *tangle.CachedMetadata) {
	defer cachedMetadata.Release(true)

	// update the solidity flags of this message
	cachedMetadata.GetMetadata().SetSolid(true)

	Events.MessageSolid.Trigger(cachedMetadata)
}

// solidQueueCheck traverses a milestone and checks if it is solid
// Missing msg are requested
// Can be aborted with abortSignal
// all cachedMsgMetas have to be released outside.
func solidQueueCheck(cachedMessageMetas map[string]*tangle.CachedMetadata, milestoneIndex milestone.Index, milestoneMessageID *hornet.MessageID, abortSignal chan struct{}) (solid bool, aborted bool) {
	ts := time.Now()

	msgsChecked := 0
	var messageIDsToSolidify hornet.MessageIDs
	messageIDsToRequest := make(map[string]struct{})

	// collect all msg to solidify by traversing the tangle
	if err := dag.TraverseParents(milestoneMessageID,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *tangle.CachedMetadata) (bool, error) { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			cachedMsgMetaMapKey := cachedMsgMeta.GetMetadata().GetMessageID().MapKey()
			if _, exists := cachedMessageMetas[cachedMsgMetaMapKey]; !exists {
				// release the msg metadata at the end to speed up calculation
				cachedMessageMetas[cachedMsgMetaMapKey] = cachedMsgMeta.Retain()
			}

			// if the msg is solid, there is no need to traverse its parents
			return !cachedMsgMeta.GetMetadata().IsSolid(), nil
		},
		// consumer
		func(cachedMsgMeta *tangle.CachedMetadata) error { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			// mark the msg as checked
			msgsChecked++

			// collect the txToSolidify in an ordered way
			messageIDsToSolidify = append(messageIDsToSolidify, cachedMsgMeta.GetMetadata().GetMessageID())

			return nil
		},
		// called on missing parents
		func(parentMessageID *hornet.MessageID) error {
			// msg does not exist => request missing msg
			messageIDsToRequest[parentMessageID.MapKey()] = struct{}{}
			return nil
		},
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false, abortSignal); err != nil {
		if err == tangle.ErrOperationAborted {
			return false, true
		}
		log.Panic(err)
	}

	tCollect := time.Now()

	if len(messageIDsToRequest) > 0 {
		var messageIDs hornet.MessageIDs
		for messageID := range messageIDsToRequest {
			messageIDs = append(messageIDs, hornet.MessageIDFromMapKey(messageID))
		}
		requested := gossip.RequestMultiple(messageIDs, milestoneIndex, true)
		log.Warnf("Stopped solidifier due to missing msg -> Requested missing msgs (%d/%d), collect: %v", requested, len(messageIDs), tCollect.Sub(ts).Truncate(time.Millisecond))
		return false, false
	}

	// no messages to request => the whole cone is solid
	// we mark all messages as solid in order from oldest to latest (needed for the tip pool)
	for _, messageID := range messageIDsToSolidify {
		cachedMsgMeta, exists := cachedMessageMetas[messageID.MapKey()]
		if !exists {
			log.Panicf("solidQueueCheck: Message not found: %v", messageID.Hex())
		}

		markMessageAsSolid(cachedMsgMeta.Retain())
	}

	tSolid := time.Now()

	if tangle.IsNodeSyncedWithThreshold() {
		// propagate solidity to the future cone (msgs attached to the msgs of this milestone)
		solidifyFutureCone(cachedMessageMetas, messageIDsToSolidify, abortSignal)
	}

	log.Infof("Solidifier finished: msgs: %d, collect: %v, solidity %v, propagation: %v, total: %v", msgsChecked, tCollect.Sub(ts).Truncate(time.Millisecond), tSolid.Sub(tCollect).Truncate(time.Millisecond), time.Since(tSolid).Truncate(time.Millisecond), time.Since(ts).Truncate(time.Millisecond))
	return true, false
}

// solidifyFutureConeOfMsg updates the solidity of the future cone (messages approving the given message).
// we have to walk the future cone, if a message became newly solid during the walk.
func solidifyFutureConeOfMsg(cachedMsgMeta *tangle.CachedMetadata) error {

	cachedMsgMetas := make(map[string]*tangle.CachedMetadata)
	cachedMsgMetas[cachedMsgMeta.GetMetadata().GetMessageID().MapKey()] = cachedMsgMeta

	defer func() {
		// release all msg metadata at the end
		for _, cachedMsgMeta := range cachedMsgMetas {
			// normal solidification could be part of a cone of old milestones while synching => no need to keep this in cache
			cachedMsgMeta.Release(true) // meta -1
		}
	}()

	messageIDs := hornet.MessageIDs{cachedMsgMeta.GetMetadata().GetMessageID()}

	return solidifyFutureCone(cachedMsgMetas, messageIDs, nil)
}

// solidifyFutureCone updates the solidity of the future cone (messages approving the given messages).
// we have to walk the future cone, if a message became newly solid during the walk.
// all cachedMsgMetas have to be released outside.
func solidifyFutureCone(cachedMsgMetas map[string]*tangle.CachedMetadata, messageIDs hornet.MessageIDs, abortSignal chan struct{}) error {

	for _, messageID := range messageIDs {

		startMessageID := messageID

		if err := dag.TraverseChildren(messageID,
			// traversal stops if no more messages pass the given condition
			func(cachedMsgMeta *tangle.CachedMetadata) (bool, error) { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1

				cachedMsgMetaMapKey := cachedMsgMeta.GetMetadata().GetMessageID().MapKey()
				if _, exists := cachedMsgMetas[cachedMsgMetaMapKey]; !exists {
					// release the msg metadata at the end to speed up calculation
					cachedMsgMetas[cachedMsgMetaMapKey] = cachedMsgMeta.Retain()
				}

				if cachedMsgMeta.GetMetadata().IsSolid() && *startMessageID != *cachedMsgMeta.GetMetadata().GetMessageID() {
					// do not walk the future cone if the current message is already solid, except it was the startTx
					return false, nil
				}

				// check if current message is solid by checking the solidity of its parents
				parentMessageIDs := hornet.MessageIDs{cachedMsgMeta.GetMetadata().GetParent1MessageID()}
				if *cachedMsgMeta.GetMetadata().GetParent1MessageID() != *cachedMsgMeta.GetMetadata().GetParent2MessageID() {
					parentMessageIDs = append(parentMessageIDs, cachedMsgMeta.GetMetadata().GetParent2MessageID())
				}

				for _, parentMessageID := range parentMessageIDs {
					if tangle.SolidEntryPointsContain(parentMessageID) {
						// Ignore solid entry points (snapshot milestone included)
						continue
					}

					cachedParentTxMeta := tangle.GetCachedMessageMetadataOrNil(parentMessageID) // meta +1
					if cachedParentTxMeta == nil {
						// parent is missing => message is not solid
						// do not walk the future cone if the current message is not solid
						return false, nil
					}

					if !cachedParentTxMeta.GetMetadata().IsSolid() {
						// parent is not solid => message is not solid
						// do not walk the future cone if the current message is not solid
						cachedParentTxMeta.Release(true) // meta -1
						return false, nil
					}
					cachedParentTxMeta.Release(true) // meta -1
				}

				// mark current message as solid
				markMessageAsSolid(cachedMsgMeta.Retain())

				// walk the future cone since the message got newly solid
				return true, nil
			},
			// consumer
			// no need to consume here
			nil,
			true, abortSignal); err != nil {
			return err
		}
	}
	return nil
}

func abortMilestoneSolidification() {
	signalChanMilestoneStopSolidificationLock.Lock()
	if signalChanMilestoneStopSolidification != nil {
		close(signalChanMilestoneStopSolidification)
		signalChanMilestoneStopSolidification = nil
	}
	signalChanMilestoneStopSolidificationLock.Unlock()
}

// solidifyMilestone tries to solidify the next known non-solid milestone and requests missing msg
func solidifyMilestone(newMilestoneIndex milestone.Index, force bool) {

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

		solidifierMilestoneIndexLock.RLock()
		triggerSignal := (newMilestoneIndex == 0) && (solidifierMilestoneIndex == 0)
		nextMilestoneSignal := newMilestoneIndex == tangle.GetSolidMilestoneIndex()+1
		olderMilestoneDetected := (newMilestoneIndex != 0) && ((solidifierMilestoneIndex != 0) && (newMilestoneIndex < solidifierMilestoneIndex))
		newMilestoneReqQueueEmptySignal := (solidifierMilestoneIndex == 0) && (newMilestoneIndex != 0) && gossip.RequestQueue().Empty()
		if !(triggerSignal || nextMilestoneSignal || olderMilestoneDetected || newMilestoneReqQueueEmptySignal) {
			// Do not run solidifier
			solidifierMilestoneIndexLock.RUnlock()
			return
		}
		solidifierMilestoneIndexLock.RUnlock()
	}

	// Stop possible other newer solidifications
	abortMilestoneSolidification()

	solidifierLock.Lock()
	defer solidifierLock.Unlock()

	currentSolidIndex := tangle.GetSolidMilestoneIndex()
	latestIndex := tangle.GetLatestMilestoneIndex()

	if currentSolidIndex == latestIndex && latestIndex != 0 {
		// Latest milestone already solid
		return
	}

	// Always traverse the oldest non-solid milestone, either it gets solid, or something is missing that should be requested.
	cachedMsToSolidify := tangle.FindClosestNextMilestoneOrNil(currentSolidIndex) // message +1
	if cachedMsToSolidify == nil {
		// No newer milestone available
		return
	}

	// Release shouldn't be forced, to cache the latest milestones
	defer cachedMsToSolidify.Release() // message -1

	milestoneIndexToSolidify := cachedMsToSolidify.GetMilestone().Index
	setSolidifierMilestoneIndex(milestoneIndexToSolidify)

	signalChanMilestoneStopSolidificationLock.Lock()
	signalChanMilestoneStopSolidification = make(chan struct{})
	signalChanMilestoneStopSolidificationLock.Unlock()

	cachedMsgMetas := make(map[string]*tangle.CachedMetadata)

	defer func() {
		// release all msg metadata at the end
		for _, cachedMsgMeta := range cachedMsgMetas {
			// normal solidification could be part of a cone of old milestones while synching => no need to keep this in cache
			cachedMsgMeta.Release(true) // meta -1
		}
	}()

	log.Infof("Run solidity check for Milestone (%d)...", milestoneIndexToSolidify)
	if becameSolid, aborted := solidQueueCheck(cachedMsgMetas, milestoneIndexToSolidify, cachedMsToSolidify.GetMilestone().MessageID, signalChanMilestoneStopSolidification); !becameSolid { // meta pass +1
		if aborted {
			// check was aborted due to older milestones/other solidifier running
			log.Infof("Aborted solid queue check for milestone %d", milestoneIndexToSolidify)
		} else {
			// Milestone not solid yet and missing msg were requested
			Events.MilestoneSolidificationFailed.Trigger(milestoneIndexToSolidify)
			log.Infof("Milestone couldn't be solidified! %d", milestoneIndexToSolidify)
		}
		setSolidifierMilestoneIndex(0)
		return
	}

	if (currentSolidIndex + 1) < milestoneIndexToSolidify {

		// Milestone is stable, but some Milestones are missing in between
		// => check if they were found, or search for them in the solidified cone
		cachedClosestNextMs := tangle.FindClosestNextMilestoneOrNil(currentSolidIndex) // message +1
		if cachedClosestNextMs.GetMilestone().Index == milestoneIndexToSolidify {
			log.Panicf("Milestones missing between (%d) and (%d).", currentSolidIndex, cachedClosestNextMs.GetMilestone().Index)
		}
		cachedClosestNextMs.Release() // message -1

		// rerun to solidify the older one
		setSolidifierMilestoneIndex(0)

		milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), true)
		return
	}

	conf, err := whiteflag.ConfirmMilestone(cachedMsgMetas, cachedMsToSolidify.GetMilestone().MessageID, func(msgMeta *tangle.CachedMetadata, index milestone.Index, confTime uint64) {
		Events.MessageConfirmed.Trigger(msgMeta, index, confTime)
	}, func(confirmation *whiteflag.Confirmation) {
		tangle.SetSolidMilestoneIndex(milestoneIndexToSolidify)
		Events.SolidMilestoneChanged.Trigger(cachedMsToSolidify) // milestone pass +1
		Events.SolidMilestoneIndexChanged.Trigger(milestoneIndexToSolidify)
		Events.MilestoneConfirmed.Trigger(confirmation)
	})

	if err != nil {
		log.Panic(err)
	}

	log.Infof("Milestone confirmed (%d): txsConfirmed: %v, txsValue: %v, txsZeroValue: %v, txsConflicting: %v, collect: %v, total: %v",
		conf.Index,
		conf.MessagesConfirmed,
		conf.MessagesIncludedWithTransactions,
		conf.MessagesExcludedWithoutTransactions,
		conf.MessagesExcludedWithConflictingTransactions,
		conf.Collecting.Truncate(time.Millisecond),
		conf.Total.Truncate(time.Millisecond),
	)

	var ctpsMessage string
	if metric, err := getConfirmedMilestoneMetric(cachedMsToSolidify.Retain(), conf.Index); err == nil {
		if tangle.IsNodeSynced() {
			// Only trigger the metrics event if the node is sync (otherwise the TPS and conf.rate is wrong)
			if firstSyncedMilestone == 0 {
				firstSyncedMilestone = conf.Index
			}
		} else {
			// reset the variable if unsynced
			firstSyncedMilestone = 0
		}

		if tangle.IsNodeSynced() && (conf.Index > firstSyncedMilestone+1) {
			// Ignore the first two milestones after node was sync (otherwise the TPS and conf.rate is wrong)
			ctpsMessage = fmt.Sprintf(", %0.2f TPS, %0.2f CTPS, %0.2f%% conf.rate", metric.MPS, metric.CMPS, metric.ConfirmationRate)
			Events.NewConfirmedMilestoneMetric.Trigger(metric)
		} else {
			ctpsMessage = fmt.Sprintf(", %0.2f CTPS", metric.CMPS)
		}
	}

	log.Infof("New solid milestone: %d%s", conf.Index, ctpsMessage)

	// Run check for next milestone
	setSolidifierMilestoneIndex(0)

	if daemon.IsStopped() {
		// do not trigger the next solidification if the node was shut down
		return
	}

	milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), false)
}

func getConfirmedMilestoneMetric(cachedMilestone *tangle.CachedMilestone, milestoneIndexToSolidify milestone.Index) (*ConfirmedMilestoneMetric, error) {
	defer cachedMilestone.Release(true)

	oldMilestone := tangle.GetCachedMilestoneOrNil(milestoneIndexToSolidify - 1) // milestone +1
	if oldMilestone == nil {
		return nil, ErrMilestoneNotFound
	}
	defer oldMilestone.Release(true) // milestone -1

	timeDiff := cachedMilestone.GetMilestone().Timestamp.Sub(oldMilestone.GetMilestone().Timestamp).Seconds()
	if timeDiff == 0 {
		return nil, ErrDivisionByZero
	}

	newNewMsgCount := metrics.SharedServerMetrics.NewMessages.Load()
	newMsgDiff := utils.GetUint32Diff(newNewMsgCount, oldNewMsgCount)
	oldNewMsgCount = newNewMsgCount

	newConfirmedMsgCount := metrics.SharedServerMetrics.ConfirmedMessages.Load()
	confirmedMsgDiff := utils.GetUint32Diff(newConfirmedMsgCount, oldConfirmedMsgCount)
	oldConfirmedMsgCount = newConfirmedMsgCount

	confRate := 0.0
	if newMsgDiff != 0 {
		confRate = (float64(confirmedMsgDiff) / float64(newMsgDiff)) * 100.0
	}

	metric := &ConfirmedMilestoneMetric{
		MilestoneIndex:         milestoneIndexToSolidify,
		MPS:                    float64(newMsgDiff) / timeDiff,
		CMPS:                   float64(confirmedMsgDiff) / timeDiff,
		ConfirmationRate:       confRate,
		TimeSinceLastMilestone: timeDiff,
	}

	return metric, nil
}

func setSolidifierMilestoneIndex(index milestone.Index) {
	solidifierMilestoneIndexLock.Lock()
	solidifierMilestoneIndex = index
	solidifierMilestoneIndexLock.Unlock()
}

func GetSolidifierMilestoneIndex() milestone.Index {
	solidifierMilestoneIndexLock.RLock()
	defer solidifierMilestoneIndexLock.RUnlock()
	return solidifierMilestoneIndex
}
