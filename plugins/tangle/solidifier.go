package tangle

import (
	"bytes"
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

	oldNewTxCount       uint32
	oldConfirmedTxCount uint32

	// Index of the first milestone that was sync after node start
	firstSyncedMilestone = milestone.Index(0)

	ErrMilestoneNotFound     = errors.New("milestone not found")
	ErrDivisionByZero        = errors.New("division by zero")
	ErrMissingMilestoneFound = errors.New("missing milestone found")
)

type ConfirmedMilestoneMetric struct {
	MilestoneIndex         milestone.Index `json:"ms_index"`
	TPS                    float64         `json:"tps"`
	CTPS                   float64         `json:"ctps"`
	ConfirmationRate       float64         `json:"conf_rate"`
	TimeSinceLastMilestone float64         `json:"time_since_last_ms"`
}

// TriggerSolidifier can be used to manually trigger the solidifier from other plugins.
func TriggerSolidifier() {
	milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), true)
}

func markMessageAsSolid(cachedMetadata *tangle.CachedMetadata) {
	defer cachedMetadata.Release(true)

	// update the solidity flags of this transaction
	cachedMetadata.GetMetadata().SetSolid(true)

	Events.MessageSolid.Trigger(cachedMetadata)
}

// solidQueueCheck traverses a milestone and checks if it is solid
// Missing tx are requested
// Can be aborted with abortSignal
// all cachedMsgMetas have to be released outside.
func solidQueueCheck(cachedMessageMetas map[string]*tangle.CachedMetadata, milestoneIndex milestone.Index, cachedMetadata *tangle.CachedMetadata, abortSignal chan struct{}) (solid bool, aborted bool) {
	defer cachedMetadata.Release(true)

	ts := time.Now()

	if _, exists := cachedMessageMetas[string(cachedMetadata.GetMetadata().GetMessageID())]; !exists {
		// release the transactions at the end to speed up calculation
		cachedMessageMetas[string(cachedMetadata.GetMetadata().GetMessageID())] = cachedMetadata.Retain()
	}

	txsChecked := 0
	var txsToSolidify hornet.Hashes
	txsToRequest := make(map[string]struct{})

	// collect all tx to solidify by traversing the tangle
	if err := dag.TraverseParents(cachedMetadata.GetMetadata().GetMessageID(),
		// traversal stops if no more transactions pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *tangle.CachedMetadata) (bool, error) { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			if _, exists := cachedMessageMetas[string(cachedMsgMeta.GetMetadata().GetMessageID())]; !exists {
				// release the tx metadata at the end to speed up calculation
				cachedMessageMetas[string(cachedMsgMeta.GetMetadata().GetMessageID())] = cachedMsgMeta.Retain()
			}

			// if the tx is solid, there is no need to traverse its parents
			return !cachedMsgMeta.GetMetadata().IsSolid(), nil
		},
		// consumer
		func(cachedMsgMeta *tangle.CachedMetadata) error { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			// mark the tx as checked
			txsChecked++

			// collect the txToSolidify in an ordered way
			txsToSolidify = append(txsToSolidify, cachedMsgMeta.GetMetadata().GetMessageID())

			return nil
		},
		// called on missing parents
		func(parentHash hornet.Hash) error {
			// tx does not exist => request missing tx
			txsToRequest[string(parentHash)] = struct{}{}
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

	if len(txsToRequest) > 0 {
		var txHashes hornet.Hashes
		for txHash := range txsToRequest {
			txHashes = append(txHashes, hornet.Hash(txHash))
		}
		requested := gossip.RequestMultiple(txHashes, milestoneIndex, true)
		log.Warnf("Stopped solidifier due to missing tx -> Requested missing txs (%d/%d), collect: %v", requested, len(txHashes), tCollect.Sub(ts).Truncate(time.Millisecond))
		return false, false
	}

	// no transactions to request => the whole cone is solid
	// we mark all transactions as solid in order from oldest to latest (needed for the tip pool)
	for _, txHash := range txsToSolidify {
		cachedMsgMeta, exists := cachedMessageMetas[string(txHash)]
		if !exists {
			log.Panicf("solidQueueCheck: Tx not found: %v", txHash.Hex())
		}

		markMessageAsSolid(cachedMsgMeta.Retain())
	}

	tSolid := time.Now()

	if tangle.IsNodeSyncedWithThreshold() {
		// propagate solidity to the future cone (txs attached to the txs of this milestone)
		solidifyFutureCone(cachedMessageMetas, txsToSolidify, abortSignal)
	}

	log.Infof("Solidifier finished: txs: %d, collect: %v, solidity %v, propagation: %v, total: %v", txsChecked, tCollect.Sub(ts).Truncate(time.Millisecond), tSolid.Sub(tCollect).Truncate(time.Millisecond), time.Since(tSolid).Truncate(time.Millisecond), time.Since(ts).Truncate(time.Millisecond))
	return true, false
}

// solidifyFutureConeOfTx updates the solidity of the future cone (transactions approving the given transaction).
// we have to walk the future cone, if a transaction became newly solid during the walk.
func solidifyFutureConeOfTx(cachedMsgMeta *tangle.CachedMetadata) error {

	cachedMsgMetas := make(map[string]*tangle.CachedMetadata)
	cachedMsgMetas[string(cachedMsgMeta.GetMetadata().GetMessageID())] = cachedMsgMeta

	defer func() {
		// release all tx metadata at the end
		for _, cachedMsgMeta := range cachedMsgMetas {
			// normal solidification could be part of a cone of old milestones while synching => no need to keep this in cache
			cachedMsgMeta.Release(true) // meta -1
		}
	}()

	txHashes := hornet.Hashes{cachedMsgMeta.GetMetadata().GetMessageID()}

	return solidifyFutureCone(cachedMsgMetas, txHashes, nil)
}

// solidifyFutureCone updates the solidity of the future cone (transactions approving the given transactions).
// we have to walk the future cone, if a transaction became newly solid during the walk.
// all cachedMsgMetas have to be released outside.
func solidifyFutureCone(cachedMsgMetas map[string]*tangle.CachedMetadata, txHashes hornet.Hashes, abortSignal chan struct{}) error {

	for _, txHash := range txHashes {

		startTxHash := txHash

		if err := dag.TraverseChildren(txHash,
			// traversal stops if no more transactions pass the given condition
			func(cachedMsgMeta *tangle.CachedMetadata) (bool, error) { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1

				if _, exists := cachedMsgMetas[string(cachedMsgMeta.GetMetadata().GetMessageID())]; !exists {
					// release the tx metadata at the end to speed up calculation
					cachedMsgMetas[string(cachedMsgMeta.GetMetadata().GetMessageID())] = cachedMsgMeta.Retain()
				}

				if cachedMsgMeta.GetMetadata().IsSolid() && !bytes.Equal(startTxHash, cachedMsgMeta.GetMetadata().GetMessageID()) {
					// do not walk the future cone if the current transaction is already solid, except it was the startTx
					return false, nil
				}

				// check if current transaction is solid by checking the solidity of its parents
				parentHashes := hornet.Hashes{cachedMsgMeta.GetMetadata().GetParent1MessageID()}
				if !bytes.Equal(cachedMsgMeta.GetMetadata().GetParent1MessageID(), cachedMsgMeta.GetMetadata().GetParent2MessageID()) {
					parentHashes = append(parentHashes, cachedMsgMeta.GetMetadata().GetParent2MessageID())
				}

				for _, parentHash := range parentHashes {
					if tangle.SolidEntryPointsContain(parentHash) {
						// Ignore solid entry points (snapshot milestone included)
						continue
					}

					cachedParentTxMeta := tangle.GetCachedMessageMetadataOrNil(parentHash) // meta +1
					if cachedParentTxMeta == nil {
						// parent is missing => transaction is not solid
						// do not walk the future cone if the current transaction is not solid
						return false, nil
					}

					if !cachedParentTxMeta.GetMetadata().IsSolid() {
						// parent is not solid => transaction is not solid
						// do not walk the future cone if the current transaction is not solid
						cachedParentTxMeta.Release(true) // meta -1
						return false, nil
					}
					cachedParentTxMeta.Release(true) // meta -1
				}

				// mark current transaction as solid
				markMessageAsSolid(cachedMsgMeta.Retain())

				// walk the future cone since the transaction got newly solid
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

// solidifyMilestone tries to solidify the next known non-solid milestone and requests missing tx
func solidifyMilestone(newMilestoneIndex milestone.Index, force bool) {

	/* How milestone solidification works:

	- A Milestone comes in and gets validated
	- Request milestone trunk/branch without traversion
	- Everytime a request queue gets empty, start the solidifier for the next known non-solid milestone
	- If tx are missing, they are requested by the solidifier
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

	milestoneIndexToSolidify := cachedMsToSolidify.GetMessage().GetMilestoneIndex()
	setSolidifierMilestoneIndex(milestoneIndexToSolidify)

	signalChanMilestoneStopSolidificationLock.Lock()
	signalChanMilestoneStopSolidification = make(chan struct{})
	signalChanMilestoneStopSolidificationLock.Unlock()

	cachedMsgMetas := make(map[string]*tangle.CachedMetadata)

	defer func() {
		// release all tx metadata at the end
		for _, cachedMsgMeta := range cachedMsgMetas {
			// normal solidification could be part of a cone of old milestones while synching => no need to keep this in cache
			cachedMsgMeta.Release(true) // meta -1
		}
	}()

	log.Infof("Run solidity check for Milestone (%d)...", milestoneIndexToSolidify)
	if becameSolid, aborted := solidQueueCheck(cachedMsgMetas, milestoneIndexToSolidify, cachedMsToSolidify.GetMessage().GetTailMetadata(), signalChanMilestoneStopSolidification); !becameSolid { // meta pass +1
		if aborted {
			// check was aborted due to older milestones/other solidifier running
			log.Infof("Aborted solid queue check for milestone %d", milestoneIndexToSolidify)
		} else {
			// Milestone not solid yet and missing tx were requested
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
		if cachedClosestNextMs.GetMessage().GetMilestoneIndex() == milestoneIndexToSolidify {
			log.Panicf("Milestones missing between (%d) and (%d).", currentSolidIndex, cachedClosestNextMs.GetMessage().GetMilestoneIndex())
		}
		cachedClosestNextMs.Release() // message -1

		// rerun to solidify the older one
		setSolidifierMilestoneIndex(0)

		milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), true)
		return
	}

	conf, err := whiteflag.ConfirmMilestone(cachedMsgMetas, cachedMsToSolidify.Retain(), func(txMeta *tangle.CachedMetadata, index milestone.Index, confTime int64) {
		Events.MessageConfirmed.Trigger(txMeta, index, confTime)
	}, func(confirmation *whiteflag.Confirmation) {
		tangle.SetSolidMilestoneIndex(milestoneIndexToSolidify)
		Events.SolidMilestoneChanged.Trigger(cachedMsToSolidify) // bundle pass +1
		Events.SolidMilestoneIndexChanged.Trigger(milestoneIndexToSolidify)
		Events.MilestoneConfirmed.Trigger(confirmation)
	})

	if err != nil {
		log.Panic(err)
	}

	log.Infof("Milestone confirmed (%d): txsConfirmed: %v, txsValue: %v, txsZeroValue: %v, txsConflicting: %v, collect: %v, total: %v",
		conf.Index,
		conf.MessagesConfirmed,
		conf.MessagesValue,
		conf.MessagesZeroValue,
		conf.MessagesConflicting,
		conf.Collecting.Truncate(time.Millisecond),
		conf.Total.Truncate(time.Millisecond),
	)

	var ctpsMessage string
	if metric, err := getConfirmedMilestoneMetric(cachedMsToSolidify.GetMessage().GetTail(), conf.Index); err == nil {
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
			ctpsMessage = fmt.Sprintf(", %0.2f TPS, %0.2f CTPS, %0.2f%% conf.rate", metric.TPS, metric.CTPS, metric.ConfirmationRate)
			Events.NewConfirmedMilestoneMetric.Trigger(metric)
		} else {
			ctpsMessage = fmt.Sprintf(", %0.2f CTPS", metric.CTPS)
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

func getConfirmedMilestoneMetric(cachedMsTailTx *tangle.CachedMessage, milestoneIndexToSolidify milestone.Index) (*ConfirmedMilestoneMetric, error) {

	newMilestoneTimestamp := time.Unix(cachedMsTailTx.GetMessage().GetTimestamp(), 0)
	cachedMsTailTx.Release()

	oldMilestone := tangle.GetCachedMilestoneOrNil(milestoneIndexToSolidify - 1) // milestone +1
	if oldMilestone == nil {
		return nil, ErrMilestoneNotFound
	}
	defer oldMilestone.Release(true) // milestone -1

	oldMilestoneTailTx := tangle.GetCachedMessageOrNil(oldMilestone.GetMilestone().MessageID)
	if oldMilestoneTailTx == nil {
		return nil, ErrMilestoneNotFound
	}
	defer oldMilestoneTailTx.Release(true)

	oldMilestoneTimestamp := time.Unix(oldMilestoneTailTx.GetMessage().GetTimestamp(), 0)
	timeDiff := newMilestoneTimestamp.Sub(oldMilestoneTimestamp).Seconds()
	if timeDiff == 0 {
		return nil, ErrDivisionByZero
	}

	newNewTxCount := metrics.SharedServerMetrics.NewTransactions.Load()
	newTxDiff := utils.GetUint32Diff(newNewTxCount, oldNewTxCount)
	oldNewTxCount = newNewTxCount

	newConfirmedTxCount := metrics.SharedServerMetrics.ConfirmedMessages.Load()
	confirmedTxDiff := utils.GetUint32Diff(newConfirmedTxCount, oldConfirmedTxCount)
	oldConfirmedTxCount = newConfirmedTxCount

	confRate := 0.0
	if newTxDiff != 0 {
		confRate = (float64(confirmedTxDiff) / float64(newTxDiff)) * 100.0
	}

	metric := &ConfirmedMilestoneMetric{
		MilestoneIndex:         milestoneIndexToSolidify,
		TPS:                    float64(newTxDiff) / timeDiff,
		CTPS:                   float64(confirmedTxDiff) / timeDiff,
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
