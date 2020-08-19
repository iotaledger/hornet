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

	oldNewTxCount       uint32
	oldConfirmedTxCount uint32

	// Index of the first milestone that was sync after node start
	firstSyncedMilestone = milestone.Index(0)

	ErrMilestoneNotFound = errors.New("milestone not found")
	ErrDivisionByZero    = errors.New("division by zero")
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

func markTransactionAsSolid(cachedTxMeta *tangle.CachedMetadata) {
	defer cachedTxMeta.Release(true)

	// Construct the complete bundle if the tail got solid (before setting solid flag => otherwise not threadsafe)
	if cachedTxMeta.GetMetadata().IsTail() {
		cachedTx := tangle.GetCachedTransactionOrNil(cachedTxMeta.GetMetadata().GetTxHash())
		if cachedTx == nil {
			log.Panicf("markTransactionAsSolid: Transaction not found: %v", cachedTxMeta.GetMetadata().GetTxHash().Trytes())
		}
		tangle.OnTailTransactionSolid(cachedTx) // tx pass +1
	}

	// update the solidity flags of this transaction
	cachedTxMeta.GetMetadata().SetSolid(true)

	Events.TransactionSolid.Trigger(cachedTxMeta.GetMetadata().GetTxHash())

	if cachedTxMeta.GetMetadata().IsTail() {
		cachedBndl := tangle.GetCachedBundleOrNil(cachedTxMeta.GetMetadata().GetTxHash()) // bundle +1
		if cachedBndl == nil {
			// bundle must be created here
			log.Panicf("markTransactionAsSolid: Bundle not found: %v", cachedTxMeta.GetMetadata().GetTxHash().Trytes())
		}
		defer cachedBndl.Release(true) // bundle -1

		// search all referenced tails of this bundle
		approveeTailTxHashes, err := dag.FindAllTails(cachedBndl.GetBundle().GetTailHash(), true, true)
		if err != nil {
			log.Panic(err)
		}

		invalid := false
		for approveeTailTxHash := range approveeTailTxHashes {
			cachedApproveeBndl := tangle.GetCachedBundleOrNil(hornet.Hash(approveeTailTxHash)) // bundle +1
			if cachedApproveeBndl == nil {
				// bundle must be created here
				log.Panicf("BundleSolid: TxHash: %v, approvee bundle not found: TailTxHash: %v", cachedTxMeta.GetMetadata().GetTxHash().Trytes(), hornet.Hash(approveeTailTxHash).Trytes())
			}
			bndl := cachedApproveeBndl.GetBundle() // bundle -1
			cachedApproveeBndl.Release(true)

			if bndl.IsInvalidPastCone() || !bndl.IsValid() || !bndl.ValidStrictSemantics() {
				// bundle has an invalid past cone
				invalid = true
				break
			}
		}

		if invalid {
			cachedBndl.GetBundle().SetInvalidPastCone(true)
		}

		Events.BundleSolid.Trigger(cachedBndl) // bundle pass +1
	}
}

// solidQueueCheck traverses a milestone and checks if it is solid
// Missing tx are requested
// Can be aborted with abortSignal
// all cachedTxMetas have to be released outside.
func solidQueueCheck(cachedTxMetas map[string]*tangle.CachedMetadata, milestoneIndex milestone.Index, cachedMsTailTxMeta *tangle.CachedMetadata, abortSignal chan struct{}) (solid bool, aborted bool) {
	defer cachedMsTailTxMeta.Release(true)

	ts := time.Now()

	if _, exists := cachedTxMetas[string(cachedMsTailTxMeta.GetMetadata().GetTxHash())]; !exists {
		// release the transactions at the end to speed up calculation
		cachedTxMetas[string(cachedMsTailTxMeta.GetMetadata().GetTxHash())] = cachedMsTailTxMeta.Retain()
	}

	txsChecked := 0
	var txsToSolidify hornet.Hashes
	txsToRequest := make(map[string]struct{})

	// collect all tx to solidify by traversing the tangle
	if err := dag.TraverseApprovees(cachedMsTailTxMeta.GetMetadata().GetTxHash(),
		// traversal stops if no more transactions pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedTxMeta *tangle.CachedMetadata) (bool, error) { // meta +1
			defer cachedTxMeta.Release(true) // meta -1

			if _, exists := cachedTxMetas[string(cachedTxMeta.GetMetadata().GetTxHash())]; !exists {
				// release the tx metadata at the end to speed up calculation
				cachedTxMetas[string(cachedTxMeta.GetMetadata().GetTxHash())] = cachedTxMeta.Retain()
			}

			// if the tx is solid, there is no need to traverse its approvees
			return !cachedTxMeta.GetMetadata().IsSolid(), nil
		},
		// consumer
		func(cachedTxMeta *tangle.CachedMetadata) error { // meta +1
			defer cachedTxMeta.Release(true) // meta -1

			// mark the tx as checked
			txsChecked++

			// collect the txToSolidify in an ordered way
			txsToSolidify = append(txsToSolidify, cachedTxMeta.GetMetadata().GetTxHash())

			return nil
		},
		// called on missing approvees
		func(approveeHash hornet.Hash) error {
			// tx does not exist => request missing tx
			txsToRequest[string(approveeHash)] = struct{}{}
			return nil
		},
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		true, false, false, abortSignal); err != nil {
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
		gossip.RequestMultiple(txHashes, milestoneIndex, true)
		log.Warnf("Stopped solidifier due to missing tx -> Requested missing txs (%d), collect: %v", len(txHashes), tCollect.Sub(ts).Truncate(time.Millisecond))
		return false, false
	}

	// no transactions to request => the whole cone is solid
	// we mark all transactions as solid in order from oldest to latest (needed for the tip pool)
	for _, txHash := range txsToSolidify {
		cachedTxMeta, exists := cachedTxMetas[string(txHash)]
		if !exists {
			log.Panicf("solidQueueCheck: Tx not found: %v", txHash.Trytes())
		}

		markTransactionAsSolid(cachedTxMeta.Retain())
	}

	tSolid := time.Now()

	if tangle.IsNodeSyncedWithThreshold() {
		// propagate solidity to the future cone (txs attached to the txs of this milestone)
		solidifyFutureCone(cachedTxMetas, txsToSolidify, false, abortSignal)
	}

	log.Infof("Solidifier finished: txs: %d, collect: %v, solidity %v, propagation: %v, total: %v", txsChecked, tCollect.Sub(ts).Truncate(time.Millisecond), tSolid.Sub(tCollect).Truncate(time.Millisecond), time.Since(tSolid).Truncate(time.Millisecond), time.Since(ts).Truncate(time.Millisecond))
	return true, false
}

// solidifyFutureConeOfTx updates the solidity of the future cone (transactions approving the given transaction).
// we have to walk the future cone, and update the solidity of all transactions by walking their past cone.
// as a special property, invocations of the yielded function share the same 'already traversed' set to circumvent
// walking the future cone of the same transactions multiple times.
func solidifyFutureConeOfTx(cachedTxMeta *tangle.CachedMetadata) error {

	cachedTxMetas := make(map[string]*tangle.CachedMetadata)
	cachedTxMetas[string(cachedTxMeta.GetMetadata().GetTxHash())] = cachedTxMeta

	defer func() {
		// release all tx metadata at the end
		for _, cachedTxMeta := range cachedTxMetas {
			// normal solidification could be part of a cone of old milestones while synching => no need to keep this in cache
			cachedTxMeta.Release(true) // meta -1
		}
	}()

	txHashes := hornet.Hashes{cachedTxMeta.GetMetadata().GetTxHash()}

	return solidifyFutureCone(cachedTxMetas, txHashes, true, nil)
}

// solidifyFutureCone updates the solidity of the future cone (transactions approving the given transactions).
// we have to walk the future cone, and update the solidity of all transactions by walking their past cone.
// as a special property, invocations of the yielded function share the same 'already traversed' set to circumvent
// walking the future cone of the same transactions multiple times.
// all cachedTxMetas have to be released outside.
func solidifyFutureCone(cachedTxMetas map[string]*tangle.CachedMetadata, txHashes hornet.Hashes, gossipSolidify bool, abortSignal chan struct{}) error {
	traversed := map[string]struct{}{}
	nonSolidTxs := map[string]struct{}{}

	// this is an efficient way to update the whole cone, because it will not be recursive,
	// since we check if we already traversed the future cone of the transactions we want to walk.
	for _, txHash := range txHashes {

		if err := dag.TraverseApprovers(txHash,
			// traversal stops if no more transactions pass the given condition
			func(cachedTxMeta *tangle.CachedMetadata) (bool, error) { // meta +1
				defer cachedTxMeta.Release(true) // meta -1

				if _, exists := cachedTxMetas[string(cachedTxMeta.GetMetadata().GetTxHash())]; !exists {
					// release the tx metadata at the end to speed up calculation
					cachedTxMetas[string(cachedTxMeta.GetMetadata().GetTxHash())] = cachedTxMeta.Retain()
				}

				if _, previouslyTraversed := traversed[string(cachedTxMeta.GetMetadata().GetTxHash())]; previouslyTraversed {
					// do not walk the future cone again if we already traversed this tx before
					return false, nil
				}

				// mark the tx as traversed, so we don't check the past cone again if it was not solid
				traversed[string(cachedTxMeta.GetMetadata().GetTxHash())] = struct{}{}

				coneSolid, err := solidifyPastCone(cachedTxMetas, nonSolidTxs, cachedTxMeta.Retain(), abortSignal)
				if err != nil {
					return false, err
				}

				if !coneSolid {
					// mark the tx to have a non-solid cone
					// this is used to reduce recursion
					nonSolidTxs[string(cachedTxMeta.GetMetadata().GetTxHash())] = struct{}{}
				}

				if coneSolid && gossipSolidify && time.Since(time.Unix(int64(cachedTxMeta.GetMetadata().GetSolidificationTimestamp()), 0)) > solidifierThreshold {
					// Skip older transactions in the gossip solidifier
					return false, nil
				}

				// do not walk the future cone if the current past cone is already non-solid
				return coneSolid, nil
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

// solidifyPastCone checks the solidity of a transaction by walking its past cone and marking other solid transactions as solid.
// as a special property, invocations of the yielded function share the same 'nonSolidTxs' set to circumvent
// walking the past cone of the same transactions multiple times.
// all cachedTxMetas have to be released outside.
func solidifyPastCone(cachedTxMetas map[string]*tangle.CachedMetadata, nonSolidTxs map[string]struct{}, startTxMeta *tangle.CachedMetadata, abortSignal chan struct{}) (bool, error) {
	defer startTxMeta.Release(true) // meta -1

	if _, exists := cachedTxMetas[string(startTxMeta.GetMetadata().GetTxHash())]; !exists {
		// release the tx metadata at the end to speed up calculation
		cachedTxMetas[string(startTxMeta.GetMetadata().GetTxHash())] = startTxMeta.Retain()
	}

	if startTxMeta.GetMetadata().IsSolid() {
		// tx metadata is already solid, no need to traverse
		return true, nil
	}

	// traverse the approvees of this transaction to check if the whole cone is solid.
	if err := dag.TraverseApprovees(startTxMeta.GetMetadata().GetTxHash(),
		// traversal stops if no more transactions pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedTxMeta *tangle.CachedMetadata) (bool, error) { // meta +1
			defer cachedTxMeta.Release(true) // meta -1

			if _, exists := cachedTxMetas[string(cachedTxMeta.GetMetadata().GetTxHash())]; !exists {
				// release the tx metadata at the end to speed up calculation
				cachedTxMetas[string(cachedTxMeta.GetMetadata().GetTxHash())] = cachedTxMeta.Retain()
			}

			if _, nonSolid := nonSolidTxs[string(cachedTxMeta.GetMetadata().GetTxHash())]; nonSolid {
				// the transaction was already marked to have a non-solid cone before, so we
				// know that a transaction is missing in the past cone
				return false, tangle.ErrTransactionNotFound
			}

			// if the tx is solid, there is no need to traverse its approvees
			return !cachedTxMeta.GetMetadata().IsSolid(), nil
		},
		// consumer
		func(cachedTxMeta *tangle.CachedMetadata) error { // meta +1
			defer cachedTxMeta.Release(true) // meta -1

			// we can mark all consumed txs as solid, since we do a DFS for non-solid txs and return an error at onMissingApprovee
			markTransactionAsSolid(cachedTxMeta.Retain())

			return nil
		},
		// called on missing approvees
		func(approveeHash hornet.Hash) error {
			// tx does not exist => the cone is not solid
			return tangle.ErrTransactionNotFound
		},
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		true, false, false, abortSignal); err != nil {
		if err == tangle.ErrTransactionNotFound {
			// a transaction was missing in the cone => the startTx is not solid
			return false, nil
		}
		return false, err
	}

	// there was no error in the traversal, which also means there was no transaction missing.
	// the whole cone of this transaction is now marked as solid.
	return true, nil
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
	cachedMsToSolidify := tangle.FindClosestNextMilestoneOrNil(currentSolidIndex) // bundle +1
	if cachedMsToSolidify == nil {
		// No newer milestone available
		return
	}

	// Release shouldn't be forced, to cache the latest milestones
	defer cachedMsToSolidify.Release() // bundle -1

	milestoneIndexToSolidify := cachedMsToSolidify.GetBundle().GetMilestoneIndex()
	setSolidifierMilestoneIndex(milestoneIndexToSolidify)

	signalChanMilestoneStopSolidificationLock.Lock()
	signalChanMilestoneStopSolidification = make(chan struct{})
	signalChanMilestoneStopSolidificationLock.Unlock()

	cachedTxMetas := make(map[string]*tangle.CachedMetadata)

	defer func() {
		// release all tx metadata at the end
		for _, cachedTxMeta := range cachedTxMetas {
			// normal solidification could be part of a cone of old milestones while synching => no need to keep this in cache
			cachedTxMeta.Release(true) // meta -1
		}
	}()

	log.Infof("Run solidity check for Milestone (%d)...", milestoneIndexToSolidify)
	if becameSolid, aborted := solidQueueCheck(cachedTxMetas, milestoneIndexToSolidify, cachedMsToSolidify.GetBundle().GetTailMetadata(), signalChanMilestoneStopSolidification); !becameSolid { // meta pass +1
		if aborted {
			// check was aborted due to older milestones/other solidifier running
			log.Infof("Aborted solid queue check for milestone %d", milestoneIndexToSolidify)
		} else {
			// Milestone not solid yet and missing tx were requested
			log.Infof("Milestone couldn't be solidified! %d", milestoneIndexToSolidify)
		}
		setSolidifierMilestoneIndex(0)
		return
	}

	if (currentSolidIndex + 1) < milestoneIndexToSolidify {

		// Milestone is stable, but some Milestones are missing in between
		// => check if they were found, or search for them in the solidified cone
		cachedClosestNextMs := tangle.FindClosestNextMilestoneOrNil(currentSolidIndex) // bundle +1
		if cachedClosestNextMs.GetBundle().GetMilestoneIndex() == milestoneIndexToSolidify {
			log.Panicf("Milestones missing between (%d) and (%d).", currentSolidIndex, cachedClosestNextMs.GetBundle().GetMilestoneIndex())
		}
		cachedClosestNextMs.Release() // bundle -1

		// rerun to solidify the older one
		setSolidifierMilestoneIndex(0)

		milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), true)
		return
	}

	conf, err := whiteflag.ConfirmMilestone(cachedTxMetas, cachedMsToSolidify.Retain(), func(txMeta *tangle.CachedMetadata, index milestone.Index, confTime int64) {
		Events.TransactionConfirmed.Trigger(txMeta, index, confTime)
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
		conf.TxsConfirmed,
		conf.TxsValue,
		conf.TxsZeroValue,
		conf.TxsConflicting,
		conf.Collecting.Truncate(time.Millisecond),
		conf.Total.Truncate(time.Millisecond),
	)

	var ctpsMessage string
	if metric, err := getConfirmedMilestoneMetric(cachedMsToSolidify.GetBundle().GetTail(), conf.Index); err == nil {
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

func getConfirmedMilestoneMetric(cachedMsTailTx *tangle.CachedTransaction, milestoneIndexToSolidify milestone.Index) (*ConfirmedMilestoneMetric, error) {

	newMilestoneTimestamp := time.Unix(cachedMsTailTx.GetTransaction().GetTimestamp(), 0)
	cachedMsTailTx.Release()

	oldMilestone := tangle.GetCachedMilestoneOrNil(milestoneIndexToSolidify - 1) // milestone +1
	if oldMilestone == nil {
		return nil, ErrMilestoneNotFound
	}
	defer oldMilestone.Release(true) // milestone -1

	oldMilestoneTailTx := tangle.GetCachedTransactionOrNil(oldMilestone.GetMilestone().Hash)
	if oldMilestoneTailTx == nil {
		return nil, ErrMilestoneNotFound
	}
	defer oldMilestoneTailTx.Release(true)

	oldMilestoneTimestamp := time.Unix(oldMilestoneTailTx.GetTransaction().GetTimestamp(), 0)
	timeDiff := newMilestoneTimestamp.Sub(oldMilestoneTimestamp).Seconds()
	if timeDiff == 0 {
		return nil, ErrDivisionByZero
	}

	newNewTxCount := metrics.SharedServerMetrics.NewTransactions.Load()
	newTxDiff := utils.GetUint32Diff(newNewTxCount, oldNewTxCount)
	oldNewTxCount = newNewTxCount

	newConfirmedTxCount := metrics.SharedServerMetrics.ConfirmedTransactions.Load()
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
