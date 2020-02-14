package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
)

var (
	milestoneSolidifierWorkerCount = 2 // must be two, so a new request can abort another, in case it is an older milestone
	milestoneSolidifierQueueSize   = 2
	milestoneSolidifierWorkerPool  *workerpool.WorkerPool

	signalChanMilestoneStopSolidification     chan struct{}
	signalChanMilestoneStopSolidificationLock syncutils.Mutex

	solidifierMilestoneIndex     milestone_index.MilestoneIndex = 0
	solidifierMilestoneIndexLock syncutils.RWMutex

	solidifierLock syncutils.RWMutex

	maxMissingMilestoneSearchDepth = 120000 // 1000 TPS at 2 min milestone interval
)

// checkSolidity checks if a single transaction is solid
func checkSolidity(cachedTx *tangle.CachedTransaction, addToApproversCache bool) (solid bool, newlySolid bool) {

	defer cachedTx.Release() // tx -1

	if cachedTx.GetTransaction().IsSolid() {
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

		cachedApproveeTx := tangle.GetCachedTransaction(approveeHash) // tx +1
		if !cachedApproveeTx.Exists() || !cachedApproveeTx.GetTransaction().IsSolid() {
			isSolid = false

			if addToApproversCache {
				// Add this Transaction as approver to the approvee if it is unknown or not solid yet
				tangle.StoreApprover(approveeHash, cachedTx.GetTransaction().GetHash()).Release()
			}
			cachedApproveeTx.Release() // tx -1
			break
		}
		cachedApproveeTx.Release() // tx -1
	}

	if isSolid {
		// update the solidity flags of this transaction and its approvers
		cachedTx.GetTransaction().SetSolid(true)
		Events.TransactionSolid.Trigger(cachedTx) // tx pass +1
	}

	return isSolid, isSolid
}

func registerApproverOfApprovee(approver trinary.Hash, approveeHash trinary.Hash, approvers map[string]map[string]struct{}) {
	// The approvee is not solid yet, we need to collect its approvers
	approversMap, exists := approvers[approveeHash]
	if !exists {
		approversMap = make(map[string]struct{})
		approvers[approveeHash] = approversMap
	}

	// Add the main tx to the approvers list of this approvee
	approversMap[approver] = struct{}{}
}

// solidQueueCheck traverses a milestone and checks if it is solid
// Missing tx are requested
// Can be aborted with abortSignal
func solidQueueCheck(milestoneIndex milestone_index.MilestoneIndex, cachedMsTailTx *tangle.CachedTransaction, abortSignal chan struct{}) (solid bool, aborted bool) {

	defer cachedMsTailTx.Release() // tx -1

	ts := time.Now()

	txsChecked := make(map[string]bool) // isSolid
	txsToTraverse := make(map[string]struct{})
	approvers := make(map[string]map[string]struct{})
	entryTxs := make(map[string]struct{})
	txsToRequest := make(map[string]struct{})
	txsToTraverse[cachedMsTailTx.GetTransaction().GetHash()] = struct{}{}

	// Collect all tx to check by traversing the tangle
	// Loop as long as new transactions are added in every loop cycle
	for len(txsToTraverse) != 0 {

		for txHash := range txsToTraverse {
			select {
			case <-abortSignal:
				return false, true
			default:
				// Go on with the check
			}

			delete(txsToTraverse, txHash)
			isEntryTx := true

			cachedTx := tangle.GetCachedTransaction(txHash) // tx +1
			if !cachedTx.Exists() {
				log.Panicf("solidQueueCheck: Tx not found: %v", txHash)
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

				// we add each transaction's approvers to the map, whether the approvee
				// exists or not, as we will not start any concrete solidifiction if any approvee is missing
				registerApproverOfApprovee(cachedTx.GetTransaction().GetHash(), approveeHash, approvers)

				if isSolid, checked := txsChecked[approveeHash]; checked {
					// Tx was already checked
					if !isSolid {
						isEntryTx = false
					}
					continue
				}

				cachedApproveeTx := tangle.GetCachedTransaction(approveeHash) // tx +1
				if !cachedApproveeTx.Exists() {
					isEntryTx = false
					txsToRequest[approveeHash] = struct{}{}

					// Mark the tx as checked
					txsChecked[approveeHash] = false
					cachedApproveeTx.Release() // tx -1
					continue
				}

				// Mark the tx as checked
				txsChecked[approveeHash] = cachedApproveeTx.GetTransaction().IsSolid()

				if !cachedApproveeTx.GetTransaction().IsSolid() {
					isEntryTx = false

					// Traverse this approvee
					txsToTraverse[approveeHash] = struct{}{}
				}
				cachedApproveeTx.Release() // tx -1
			}

			if isEntryTx {
				// Trunk and branch are solid, this tx is an entry point to start the solidify walk
				entryTxs[cachedTx.GetTransaction().GetHash()] = struct{}{}
			}
			cachedTx.Release() // tx -1
		}
	}
	tc := time.Now()

	if len(txsToRequest) > 0 {
		var txHashes []string
		for txHash := range txsToRequest {
			txHashes = append(txHashes, txHash)
		}
		gossip.RequestMulti(txHashes, milestoneIndex)
		log.Warnf("Stopped solidifier due to missing tx -> Requested missing txs (%d)", len(txHashes))
		return false, false
	}

	if len(entryTxs) == 0 {
		log.Panicf("Solidification failed! No solid entry points for subtangle found! (%d)", milestoneIndex)
	}

	// Loop as long as new solid transactions are found in every loop cycle
	newSolidTxFound := true
	loopCnt := 0
	for newSolidTxFound {
		loopCnt++
		newSolidTxFound = false

		for entryTxHash := range entryTxs {
			select {
			case <-abortSignal:
				return false, true
			default:
				// Go on with the check
			}

			cachedEntryTx := tangle.GetCachedTransaction(entryTxHash) // tx +1
			if !cachedEntryTx.Exists() {
				log.Panicf("solidQueueCheck: EntryTx not found: %v", entryTxHash)
			}

			if solid, newlySolid := checkSolidity(cachedEntryTx.Retain(), false); solid {
				// Add all tx to the map that approve this solid transaction
				for approverTxHash := range approvers[entryTxHash] {
					entryTxs[approverTxHash] = struct{}{}
				}

				if newlySolid && tangle.IsNodeSyncedWithThreshold() {
					// Propagate solidity to the future cone (txs attached to the txs of this milestone)
					gossipSolidifierWorkerPool.Submit(cachedEntryTx.Retain()) // tx pass +1
				}

				// Delete the tx from the map since it is solid
				delete(entryTxs, entryTxHash)
				newSolidTxFound = true
			}
			cachedEntryTx.Release() // tx -1
		}
	}

	// Subtangle is solid if all tx were deleted from the map
	queueSolid := len(entryTxs) == 0

	log.Infof("Solidifier finished (%d): passed: %v, tx: %d, collect: %v, total: %v, entryTx: %d", loopCnt, queueSolid, len(txsChecked), tc.Sub(ts), time.Since(ts), len(entryTxs))
	return queueSolid, false
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
func solidifyMilestone(msIndexEmptiedQueue milestone_index.MilestoneIndex, forceAbort bool) {

	/* How milestone solidification works:

	- A Milestone comes in and gets validated
	- Request milestone trunk/branch without traversion
	- Everytime a request queue gets empty, start the solidifier for the next known non-solid milestone
	- If tx are missing, they are requested by the solidifier
	- If an older queue gets empty than the current solidification index, the traversion can be aborted with a signal and restarted
	- If we miss more than WARP_SYNC_THRESHOLD milestones in our requests, request them via warp sync

	- Special situation startup:
		- RequestMilestonesAndTraverse:
			- loop over all other non-solid milestones from latest to oldest, traverse and request, remove other milestones hashes during the walk
			- this will request all unknown tx in parallel => improve sync speed
			- this should be done without blowing up the RAM
			- don't stop that traversion if older milestone comes in, its only once and helps at startup
	*/

	if !forceAbort {
		solidifierMilestoneIndexLock.RLock()
		if (solidifierMilestoneIndex != 0) && ((msIndexEmptiedQueue == 0) || (msIndexEmptiedQueue >= solidifierMilestoneIndex)) {
			// Another older milestone solidification is already running
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
	defer cachedMsToSolidify.Release() // bundle -1

	milestoneIndexToSolidify := cachedMsToSolidify.GetBundle().GetMilestoneIndex()
	setSolidifierMilestoneIndex(milestoneIndexToSolidify)

	signalChanMilestoneStopSolidificationLock.Lock()
	signalChanMilestoneStopSolidification = make(chan struct{})
	signalChanMilestoneStopSolidificationLock.Unlock()

	log.Infof("Run solidity check for Milestone (%d)...", milestoneIndexToSolidify)

	if becameSolid, aborted := solidQueueCheck(milestoneIndexToSolidify, cachedMsToSolidify.GetBundle().GetTail(), signalChanMilestoneStopSolidification); !becameSolid { // tx pass +1
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
			log.Infof("Milestones missing between (%d) and (%d). Search for missing milestones...", currentSolidIndex, milestoneIndexToSolidify)

			// No Milestones found in between => search an older milestone in the solid cone
			if found, aborted := searchMissingMilestone(currentSolidIndex, milestoneIndexToSolidify, cachedMsToSolidify.GetBundle().GetTail(), maxMissingMilestoneSearchDepth, signalChanMilestoneStopSolidification); !found { // tx pass +1
				if aborted {
					log.Infof("Aborted search for missing milestones between (%d) and (%d).", currentSolidIndex, milestoneIndexToSolidify)
				} else {
					// No milestones found => this should not happen!
					log.Panicf("Milestones missing between (%d) and (%d).", currentSolidIndex, milestoneIndexToSolidify)
				}
			}
		}
		cachedClosestNextMs.Release() // bundle -1

		// rerun to solidify the older one
		setSolidifierMilestoneIndex(0)

		milestoneSolidifierWorkerPool.TrySubmit(milestone_index.MilestoneIndex(0), true)
		return
	}

	tangle.WriteLockLedger()
	defer tangle.WriteUnlockLedger()
	confirmMilestone(milestoneIndexToSolidify, cachedMsToSolidify.GetBundle().GetTail()) // tx pass +1

	tangle.SetSolidMilestone(cachedMsToSolidify.Retain())    // bundle pass +1
	Events.SolidMilestoneChanged.Trigger(cachedMsToSolidify) // bundle pass +1
	log.Infof("New solid milestone: %d", milestoneIndexToSolidify)

	// Run check for next milestone
	setSolidifierMilestoneIndex(0)

	milestoneSolidifierWorkerPool.TrySubmit(milestone_index.MilestoneIndex(0), true)
}

func searchMissingMilestone(solidMilestoneIndex milestone_index.MilestoneIndex, startMilestoneIndex milestone_index.MilestoneIndex, cachedMsTailTx *tangle.CachedTransaction, maxSearchDepth int, abortSignal chan struct{}) (found bool, aborted bool) {

	defer cachedMsTailTx.Release() // tx -1

	var loopCnt int
	var milestoneFound bool

	ts := time.Now()

	txsChecked := make(map[string]struct{})
	txsToTraverse := make(map[string]struct{})
	txsToTraverse[cachedMsTailTx.GetTransaction().GetHash()] = struct{}{}

	// Search milestones by traversing the tangle
	for loopCnt = 0; (len(txsToTraverse) != 0) && (loopCnt < maxSearchDepth); loopCnt++ {

		for txHash := range txsToTraverse {
			select {
			case <-abortSignal:
				return false, true
			default:
				// Go on with the check
			}
			delete(txsToTraverse, txHash)

			cachedTx := tangle.GetCachedTransaction(txHash) // tx +1
			if !cachedTx.Exists() {
				log.Panicf("searchMissingMilestone: Transaction not found: %v", txHash)
			}

			approveeHashes := []trinary.Hash{cachedTx.GetTransaction().GetTrunk()}
			if cachedTx.GetTransaction().GetTrunk() != cachedTx.GetTransaction().GetBranch() {
				approveeHashes = append(approveeHashes, cachedTx.GetTransaction().GetBranch())
			}
			cachedTx.Release() // tx -1

			for _, approveeHash := range approveeHashes {
				if tangle.SolidEntryPointsContain(approveeHash) {
					// Ignore solid entry points (snapshot milestone included)
					continue
				}

				if _, checked := txsChecked[approveeHash]; checked {
					// Tx was already checked
					continue
				}

				cachedApproveeTx := tangle.GetCachedTransaction(approveeHash) // tx +1
				if !cachedApproveeTx.Exists() {
					log.Panicf("searchMissingMilestone: Transaction not found: %v", approveeHash)
				}

				if !cachedApproveeTx.GetTransaction().IsTail() {
					cachedApproveeTx.Release() // tx -1
					continue
				}

				if tangle.IsMaybeMilestone(cachedApproveeTx.Retain()) { // tx pass +1
					// This tx could belong to a milestone
					// => load bundle, and start the milestone check

					cachedBndl := tangle.GetBundleOfTailTransactionOrNil(cachedApproveeTx.GetTransaction().Tx.Hash) // bundle +1
					if cachedBndl == nil {
						log.Panicf("searchMissingMilestone: Tx: %v, Bundle not found: %v", approveeHash, cachedApproveeTx.GetTransaction().Tx.Bundle)
					}

					isMilestone, err := tangle.CheckIfMilestone(cachedBndl.Retain()) // bundle pass +1
					if err != nil {
						log.Infof("searchMissingMilestone: Milestone check failed: %s", err.Error())
					}

					if isMilestone {
						msIndex := cachedBndl.GetBundle().GetMilestoneIndex()
						if (msIndex > solidMilestoneIndex) && (msIndex < startMilestoneIndex) {
							// Milestone found!
							milestoneFound = true
							processValidMilestone(cachedBndl.Retain()) // bundle pass +1
							cachedApproveeTx.Release()                 // tx -1
							cachedBndl.Release()                       // bundle -1
							break
						}
					}

					cachedBndl.Release() // bundle -1
				}

				cachedApproveeTx.Release() // tx -1

				// Traverse this approvee
				txsToTraverse[approveeHash] = struct{}{}

				// Mark the tx as checked
				txsChecked[approveeHash] = struct{}{}
			}
		}
	}

	log.Infof("searchMissingMilestone finished (%d): found: %v, checked txs: %d, total: %v", loopCnt, milestoneFound, len(txsChecked), time.Since(ts))
	return milestoneFound, false
}

func setSolidifierMilestoneIndex(index milestone_index.MilestoneIndex) {
	solidifierMilestoneIndexLock.Lock()
	solidifierMilestoneIndex = index
	solidifierMilestoneIndexLock.Unlock()
}

func GetSolidifierMilestoneIndex() milestone_index.MilestoneIndex {
	solidifierMilestoneIndexLock.RLock()
	defer solidifierMilestoneIndexLock.RUnlock()
	return solidifierMilestoneIndex
}
