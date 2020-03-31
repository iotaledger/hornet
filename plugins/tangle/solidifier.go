package tangle

import (
	"errors"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
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

	revalidationMilestoneIndex = milestone.Index(0)

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

func markTransactionAsSolid(cachedTx *tangle.CachedTransaction) {
	// Construct the complete bundle if the tail got solid (before setting solid flag => otherwise not threadsafe)
	if cachedTx.GetTransaction().IsTail() {
		tangle.OnTailTransactionSolid(cachedTx.Retain()) // tx pass +1
	}

	// update the solidity flags of this transaction
	cachedTx.GetMetadata().SetSolid(true)

	Events.TransactionSolid.Trigger(cachedTx) // tx pass +1

	cachedTx.Release(true)
}

// checkSolidity checks if a single transaction is solid
func checkSolidity(cachedTx *tangle.CachedTransaction) (solid bool, newlySolid bool) {

	// Normal solidification could be part of a cone of old milestones while synching
	// => no need to keep this in cache
	// If future cone solidifier calls this, it has it's own Release with cacheTime anyway
	defer cachedTx.Release(true) // tx -1

	if cachedTx.GetMetadata().IsSolid() {
		return true, false
	}

	isSolid := true

	approveeHashes := []trinary.Hash{cachedTx.GetTransaction().GetTrunk()}
	if cachedTx.GetTransaction().GetTrunk() != cachedTx.GetTransaction().GetBranch() {
		approveeHashes = append(approveeHashes, cachedTx.GetTransaction().GetBranch())
	}

	for _, approveeHash := range approveeHashes {
		if tangle.SolidEntryPointsContain(approveeHash) {
			// Ignore solid entry points (snapshot milestone included)
			continue
		}

		cachedApproveeTx := tangle.GetCachedTransactionOrNil(approveeHash) // tx +1
		if cachedApproveeTx == nil {
			isSolid = false
			break
		}

		if !cachedApproveeTx.GetMetadata().IsSolid() {
			isSolid = false
			cachedApproveeTx.Release(true) // tx -1
			break
		}
		cachedApproveeTx.Release(true) // tx -1
	}

	if isSolid {
		markTransactionAsSolid(cachedTx.Retain())
	}

	return isSolid, isSolid
}

// solidQueueCheck traverses a milestone and checks if it is solid
// Missing tx are requested
// Can be aborted with abortSignal
func solidQueueCheck(milestoneIndex milestone.Index, cachedMsTailTx *tangle.CachedTransaction, revalidate bool, abortSignal chan struct{}) (solid bool, aborted bool) {

	ts := time.Now()

	solidMilestoneIndex := tangle.GetSolidMilestoneIndex()

	cachedTxs := make(map[trinary.Hash]*tangle.CachedTransaction)
	cachedTxs[cachedMsTailTx.GetTransaction().GetHash()] = cachedMsTailTx

	defer func() {
		// Release all txs at the end
		for _, cachedTx := range cachedTxs {
			// Normal solidification could be part of a cone of old milestones while synching => no need to keep this in cache
			cachedTx.Release(true) // tx -1
		}
	}()

	txsToTraverse := make(map[trinary.Hash]struct{})
	txsToTraverse[cachedMsTailTx.GetTransaction().GetHash()] = struct{}{}

	txsChecked := make(map[trinary.Hash]struct{})
	txsToSolidify := make(map[trinary.Hash]struct{})
	txsToRequest := make(map[trinary.Hash]struct{})

	// Collect all tx to check by traversing the tangle
	// Loop as long as new transactions are added in every loop cycle
	for len(txsToTraverse) != 0 {

		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)
			txsToSolidify[txHash] = struct{}{}

			if daemon.IsStopped() {
				return false, true
			}

			select {
			case <-abortSignal:
				return false, true
			default:
				// Go on with the check
			}

			cachedTx, exists := cachedTxs[txHash]
			if !exists {
				cachedTx = tangle.GetCachedTransactionOrNil(txHash) // tx +1
				if cachedTx == nil {
					log.Panicf("solidQueueCheck: Tx not found: %v", txHash)
				}
				cachedTxs[txHash] = cachedTx
			}

			approveeHashes := []trinary.Hash{cachedTx.GetTransaction().GetTrunk()}
			if cachedTx.GetTransaction().GetTrunk() != cachedTx.GetTransaction().GetBranch() {
				approveeHashes = append(approveeHashes, cachedTx.GetTransaction().GetBranch())
			}

			for _, approveeHash := range approveeHashes {
				if tangle.SolidEntryPointsContain(approveeHash) {
					// Ignore solid entry points (snapshot milestone included)
					continue
				}

				if _, checked := txsChecked[approveeHash]; checked {
					// Approvee Tx was already checked
					continue
				}

				cachedApproveeTx, exists := cachedTxs[approveeHash]
				if !exists {
					cachedApproveeTx = tangle.GetCachedTransactionOrNil(approveeHash) // tx +1
					if cachedApproveeTx == nil {
						// Tx does not exist => request missing tx
						txsToRequest[approveeHash] = struct{}{}

						// Mark the tx as checked
						txsChecked[approveeHash] = struct{}{}
						continue
					}
					cachedTxs[approveeHash] = cachedApproveeTx
				}

				// Mark the tx as checked
				var approveeSolid bool
				if !revalidate {
					approveeSolid = cachedApproveeTx.GetMetadata().IsSolid()
				} else {
					// The metadata of this cone may be corrupted => do not trust the solid flags
					if confirmed, at := cachedApproveeTx.GetMetadata().GetConfirmed(); confirmed {
						if at <= solidMilestoneIndex {
							// Mark the tx as solid if it was confirmed by an valid milestone
							approveeSolid = true
						} else {
							// Corrupted Tx was confirmed by an invalid milestone => reset metadata
							cachedApproveeTx.GetMetadata().Reset()
						}
					} else {
						// Corrupted Tx was not confirmed, but could be solid => reset metadata
						cachedApproveeTx.GetMetadata().Reset()
					}

					// We should also delete corrupted bundle information (it will be reapplied at solidification and confirmation).
					// This also handles the special case if a milestone bundle was stored, but the milestone is missing in the database.
					if !approveeSolid && cachedApproveeTx.GetTransaction().IsTail() && (approveeHash != consts.NullHashTrytes) {
						if cachedBndl := tangle.GetCachedBundleOrNil(approveeHash); cachedBndl != nil {

							// Reset corrupted meta tags of the bundle
							cachedBndl.GetBundle().ResetSolidAndConfirmed()

							// Reapplies missing spent addresses to the database
							cachedBndl.GetBundle().ApplySpentAddresses()

							if cachedBndl.GetBundle().IsMilestone() {
								// Reapply milestone information to database
								_, cachedMilestone := tangle.StoreMilestone(cachedBndl.GetBundle())

								// Do not force release, since it is loaded again
								cachedMilestone.Release() // milestone +-0

								// Always fire the event to abort the walk and release the cached transactions
								// => otherwise the walked cone could become to big and lead to OOM
								tangle.Events.ReceivedValidMilestone.Trigger(cachedBndl) // bundle pass +1

								// Do not force release, since it is loaded again
								cachedBndl.Release()
								return false, true
							}
							cachedBndl.Release(true)
						}
					}
				}

				// Mark the tx as checked
				txsChecked[approveeHash] = struct{}{}

				if !approveeSolid {
					// Traverse this approvee
					txsToTraverse[approveeHash] = struct{}{}
				}
			}
		}
	}
	tc := time.Now()

	if len(txsToRequest) > 0 {
		var txHashes []string
		for txHash := range txsToRequest {
			txHashes = append(txHashes, txHash)
		}
		gossip.RequestMultiple(txHashes, milestoneIndex, true)
		log.Warnf("Stopped solidifier due to missing tx -> Requested missing txs (%d), collect: %v", len(txHashes), tc.Sub(ts).Truncate(time.Millisecond))
		return false, false
	}

	// No transactions to request => the whole cone is solid
	// We can mark all transactions in random order as solid

	for txHash := range txsToSolidify {
		cachedTx, exists := cachedTxs[txHash]
		if !exists {
			log.Panicf("solidQueueCheck: Tx not found: %v", txHash)
		}

		if revalidate {
			// If this cone is maybe corrupted, the transaction is added again to the database to store all additional information
			cachedTx, _ := tangle.AddTransactionToStorage(cachedTx.GetTransaction(), tangle.GetLatestMilestoneIndex(), true, true, true)
			cachedTx.Release(true)
		}

		markTransactionAsSolid(cachedTx.Retain())
	}

	if !revalidate && tangle.IsNodeSyncedWithThreshold() {
		// Propagate solidity to the future cone (txs attached to the txs of this milestone)

		// All solidified txs are newly solidified => propagate all
		for txHash := range txsToSolidify {
			for _, approverHash := range tangle.GetApproverHashes(txHash, true) {
				cachedApproverTx := tangle.GetCachedTransactionOrNil(approverHash) // tx +1
				if cachedApproverTx == nil {
					continue
				}

				if cachedApproverTx.GetMetadata().IsSolid() {
					// Do not propagate already solid Txs

					// Do no force release here, otherwise cacheTime for new Tx could be ignored
					cachedApproverTx.Release() // tx -1
					continue
				}

				if _, added := gossipSolidifierWorkerPool.Submit(cachedApproverTx.Retain()); !added { // tx pass +1
					// Do no force release here, otherwise cacheTime for new Tx could be ignored
					cachedApproverTx.Release() // tx -1
				}

				// Do no force release here, otherwise cacheTime for new Tx could be ignored
				cachedApproverTx.Release() // tx -1
			}
		}
	}

	log.Infof("Solidifier finished: txs: %d, collect: %v, total: %v", len(txsChecked), tc.Sub(ts).Truncate(time.Millisecond), time.Since(ts).Truncate(time.Millisecond))
	return true, false
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
	- If we miss more than WARP_SYNC_THRESHOLD milestones in our requests, request them via warp sync

	*/

	if !force {
		/*
			If solidification is not forced, we will only run the solidifier under one of the following conditions:
				- newMilestoneIndex==0 (triggersignal) and solidifierMilestoneIndex==0 (no ongoing solidification)
				- newMilestoneIndex==solidMilestoneIndex+1 (next milestone)
				- newMilestoneIndex!=0 (new milestone received) and solidifierMilestoneIndex!=0 (ongoing solidification) and newMilestoneIndex<solidifierMilestoneIndex (milestone older than ongoing solidification)
		*/

		solidifierMilestoneIndexLock.RLock()
		triggerSignal := (newMilestoneIndex == 0) && (solidifierMilestoneIndex == 0)
		nextMilestoneSignal := newMilestoneIndex == tangle.GetSolidMilestoneIndex()+1
		olderMilestoneDetected := (newMilestoneIndex != 0) && ((solidifierMilestoneIndex != 0) && (newMilestoneIndex < solidifierMilestoneIndex))
		if !(triggerSignal || nextMilestoneSignal || olderMilestoneDetected) {
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

	log.Infof("Run solidity check for Milestone (%d)...", milestoneIndexToSolidify)
	revalidateMilestone := (revalidationMilestoneIndex != 0) && (milestoneIndexToSolidify <= revalidationMilestoneIndex)

	if becameSolid, aborted := solidQueueCheck(milestoneIndexToSolidify, cachedMsToSolidify.GetBundle().GetTail(), revalidateMilestone, signalChanMilestoneStopSolidification); !becameSolid { // tx pass +1
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
		if cachedClosestNextMs.GetBundle().GetMilestoneIndex() == cachedMsToSolidify.GetBundle().GetMilestoneIndex() {
			log.Panicf("Milestones missing between (%d) and (%d).", currentSolidIndex, cachedClosestNextMs.GetBundle().GetMilestoneIndex())
		}
		cachedClosestNextMs.Release() // bundle -1

		// rerun to solidify the older one
		setSolidifierMilestoneIndex(0)

		milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), true)
		return
	}

	tangle.WriteLockLedger()
	defer tangle.WriteUnlockLedger()
	confirmMilestone(milestoneIndexToSolidify, cachedMsToSolidify.GetBundle().GetTail()) // tx pass +1

	tangle.SetSolidMilestone(cachedMsToSolidify.Retain())    // bundle pass +1
	Events.SolidMilestoneChanged.Trigger(cachedMsToSolidify) // bundle pass +1

	var ctpsMessage string
	if metric, err := getConfirmedMilestoneMetric(cachedMsToSolidify.GetBundle().GetTail(), milestoneIndexToSolidify); err == nil {
		ctpsMessage = fmt.Sprintf(", %0.2f TPS, %0.2f CTPS, %0.2f%% conf.rate", metric.TPS, metric.CTPS, metric.ConfirmationRate)
		if tangle.IsNodeSynced() {
			// Only trigger the metrics event if the node is sync (otherwise the TPS and conf.rate is wrong)
			if firstSyncedMilestone == 0 {
				firstSyncedMilestone = milestoneIndexToSolidify
			}

			if milestoneIndexToSolidify > firstSyncedMilestone+1 {
				// Ignore the first two milestones after node was sync (otherwise the TPS and conf.rate is wrong)
				Events.NewConfirmedMilestoneMetric.Trigger(metric)
			}
		}
	}

	log.Infof("New solid milestone: %d%s", milestoneIndexToSolidify, ctpsMessage)

	if (revalidationMilestoneIndex != 0) && milestoneIndexToSolidify > revalidationMilestoneIndex {
		revalidationMilestoneIndex = 0
		log.Info("Final stage of database revalidation successful. Your database is consistent again.")
	}

	// Run check for next milestone
	setSolidifierMilestoneIndex(0)

	milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), false)
}

func getConfirmedMilestoneMetric(cachedMsTailTx *tangle.CachedTransaction, milestoneIndexToSolidify milestone.Index) (*ConfirmedMilestoneMetric, error) {

	newMilestoneTimestamp := time.Unix(cachedMsTailTx.GetTransaction().GetTimestamp(), 0)
	cachedMsTailTx.Release()

	oldMilestone := tangle.GetCachedMilestoneOrNil(milestoneIndexToSolidify - 1)
	if oldMilestone == nil {
		return nil, ErrMilestoneNotFound
	}
	defer oldMilestone.Release(true)

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
	newTxDiff := metrics.GetUint32Diff(newNewTxCount, oldNewTxCount)
	oldNewTxCount = newNewTxCount

	newConfirmedTxCount := metrics.SharedServerMetrics.ConfirmedTransactions.Load()
	confirmedTxDiff := metrics.GetUint32Diff(newConfirmedTxCount, oldConfirmedTxCount)
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

func requestAllMissingTxsOfKnownMilestones(solidMilestoneIndex milestone.Index, knownLatestMilestone milestone.Index) {

	if solidMilestoneIndex == 0 || knownLatestMilestone == 0 || solidMilestoneIndex == knownLatestMilestone {
		// don't request anything if we are sync (or don't know about a newer ms)
		return
	}

	ts := time.Now()
	log.Info("Requesting non-solid milestones...")

	txsChecked := make(map[trinary.Hash]struct{})
	for milestoneIndex := solidMilestoneIndex + 1; milestoneIndex < knownLatestMilestone; milestoneIndex++ {
		cachedMs := tangle.GetCachedMilestoneOrNil(milestoneIndex)
		if cachedMs == nil {
			// Milestone unknown => continue
			continue
		}

		cachedBndl := tangle.GetCachedBundleOrNil(cachedMs.GetMilestone().Hash)
		if cachedBndl == nil {
			// Milestone unknown => continue
			cachedMs.Release(true)
			continue
		}
		cachedMs.Release(true)

		cachedTailTx := cachedBndl.GetBundle().GetTail()
		cachedBndl.Release(true)

		txsToTraverse := make(map[trinary.Hash]struct{})
		txsToTraverse[cachedTailTx.GetTransaction().GetHash()] = struct{}{}

		// Do not force release since it is loaded again
		cachedTailTx.Release()

		// Collect all tx to check by traversing the tangle
		// Loop as long as new transactions are added in every loop cycle
		for len(txsToTraverse) != 0 {

			for txHash := range txsToTraverse {
				delete(txsToTraverse, txHash)

				if daemon.IsStopped() {
					return
				}

				cachedTx := tangle.GetCachedTransactionOrNil(txHash) // tx +1
				if cachedTx == nil {
					log.Panicf("requestAllMissingTxsOfKnownMilestones: Tx not found: %v", txHash)
				}

				approveeHashes := []trinary.Hash{cachedTx.GetTransaction().GetTrunk()}
				if cachedTx.GetTransaction().GetTrunk() != cachedTx.GetTransaction().GetBranch() {
					approveeHashes = append(approveeHashes, cachedTx.GetTransaction().GetBranch())
				}
				cachedTx.Release(true)

				for _, approveeHash := range approveeHashes {
					if tangle.SolidEntryPointsContain(approveeHash) {
						// Ignore solid entry points (snapshot milestone included)
						continue
					}

					if _, checked := txsChecked[approveeHash]; checked {
						// Approvee Tx was already checked
						continue
					}
					txsChecked[approveeHash] = struct{}{}

					cachedApproveeTx := tangle.GetCachedTransactionOrNil(approveeHash) // tx +1
					if cachedApproveeTx == nil {
						// Tx does not exist => request it
						gossip.Request(approveeHash, milestoneIndex, true)
						continue
					}

					if cachedApproveeTx.GetMetadata().IsSolid() {
						cachedApproveeTx.Release(true)
						continue
					}

					// Do not force release since it is loaded again
					cachedApproveeTx.Release()

					// Traverse this approvee
					txsToTraverse[approveeHash] = struct{}{}
				}
			}
		}
	}
	log.Infof("Requesting non-solid milestones finished, took: %v", time.Since(ts).Truncate(time.Second))
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
