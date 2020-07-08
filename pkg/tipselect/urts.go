package tipselect

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"go.uber.org/atomic"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/utils"
)

// Score defines the score of a tip.
type Score int

// TipSelectionFunc is a function which performs a tipselection and returns two tips.
type TipSelectionFunc = func() (hornet.Hashes, error)

// TipSelStats holds the stats for a tipselection run.
type TipSelStats struct {
	// The duration of the tip-selection for both walks.
	Duration time.Duration `json:"duration"`
	// The amount of steps taken, respectively transactions walked towards the present of the graph.
	StepsTaken uint64 `json:"steps_taken"`
	// The amount of steps jumped, meaning approvers selected without validating, as they were
	// walked/validated into by the previous walk.
	StepsJumped uint64 `json:"steps_jumped"`
	// The amount of transactions which were evaluated.
	Evaluated uint64 `json:"evaluated"`
}

// TipCaller is used to signal tip events.
func TipCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Tip))(params[0].(*Tip))
}

// WalkerStatsCaller is used to signal tip selection events.
func WalkerStatsCaller(handler interface{}, params ...interface{}) {
	handler.(func(*TipSelStats))(params[0].(*TipSelStats))
}

const (
	// ScoreLazy is a lazy zip and should not be selected.
	ScoreLazy Score = iota
	// ScoreSemiLazy is a somewhat lazy tip.
	ScoreSemiLazy
	// ScoreNonLazy is a non-lazy tip.
	ScoreNonLazy
)

var (
	// ErrNoTipsAvailable is returned when no tips are available in the node.
	ErrNoTipsAvailable = errors.New("no tips available")
)

// Tip defines a tip.
type Tip struct {
	// Score is the score of the tip.
	Score Score
	// Hash is the transaction hash of the tip.
	Hash hornet.Hash
	// TimeFirstApprover is the timestamp the tip was referenced for the first time by another transaction.
	TimeFirstApprover time.Time
	// ApproversCount is the amount the tip was referenced by other transactions.
	ApproversCount *atomic.Uint32
}

// Events represents events happening on the tip-selector.
type Events struct {
	// TipAdded is fired when a tip is added.
	TipAdded *events.Event
	// TipRemoved is fired when a tip is removed.
	TipRemoved *events.Event
	// TipSelPerformed is fired when a tipselection was performed.
	TipSelPerformed *events.Event
}

// TipSelector manages a list of tips and emits events for their removal and addition.
type TipSelector struct {
	// maxDeltaTxYoungestRootSnapshotIndexToLSMI is the maximum allowed delta
	// value for the YTRSI of a given transaction in relation to the current LSMI.
	maxDeltaTxYoungestRootSnapshotIndexToLSMI milestone.Index
	// maxDeltaTxApproveesOldestRootSnapshotIndexToLSMI is the maximum allowed delta
	// value between OTRSI of the approvees of a given transaction in relation to the current LSMI.
	maxDeltaTxApproveesOldestRootSnapshotIndexToLSMI milestone.Index
	// belowMaxDepth is a threshold value which indicates that a transaction
	// is not relevant in relation to the recent parts of the tangle.
	belowMaxDepth milestone.Index
	// maxReferencedTipAgeSeconds is the maximum time a tip remains in the tip pool
	// after it was referenced by the first transaction.
	maxReferencedTipAgeSeconds time.Duration
	// maxApprovers is the maximum amount of references by other transactions
	// before the tip is removed from the tip pool.
	maxApprovers uint32

	// tipsMap contains only semi- and non-lazy tips.
	tipsMap  map[string]*Tip
	tipsLock syncutils.RWMutex
	// scoreSum is the sum of the score of all tips.
	scoreSum int
	// Events are the events that are triggered by the TipSelector.
	Events Events
}

// New creates a new tip-selector.
func New(maxDeltaTxYoungestRootSnapshotIndexToLSMI int, maxDeltaTxApproveesOldestRootSnapshotIndexToLSMI int, belowMaxDepth int, maxReferencedTipAgeSeconds time.Duration, maxApprovers uint32) *TipSelector {
	return &TipSelector{
		maxDeltaTxYoungestRootSnapshotIndexToLSMI:        milestone.Index(maxDeltaTxYoungestRootSnapshotIndexToLSMI),
		maxDeltaTxApproveesOldestRootSnapshotIndexToLSMI: milestone.Index(maxDeltaTxApproveesOldestRootSnapshotIndexToLSMI),
		belowMaxDepth:              milestone.Index(belowMaxDepth),
		maxReferencedTipAgeSeconds: maxReferencedTipAgeSeconds,
		maxApprovers:               maxApprovers,
		tipsMap:                    make(map[string]*Tip),
		Events: Events{
			TipAdded:        events.NewEvent(TipCaller),
			TipRemoved:      events.NewEvent(TipCaller),
			TipSelPerformed: events.NewEvent(WalkerStatsCaller),
		},
	}
}

// AddTip adds the given tailTxHash as a tip.
func (ts *TipSelector) AddTip(tailTxHash hornet.Hash) {
	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	if _, exists := ts.tipsMap[string(tailTxHash)]; exists {
		// tip already exists
		return
	}

	score := ts.calculateScore(tailTxHash)
	if score == ScoreLazy {
		// Do not add tip at all
		// ToDo: should lazy tips remove old tips from the pool?
		return
	}

	tip := &Tip{
		Score:             score,
		Hash:              tailTxHash,
		TimeFirstApprover: time.Time{},
		ApproversCount:    atomic.NewUint32(0),
	}
	ts.tipsMap[string(tailTxHash)] = tip
	ts.scoreSum += int(tip.Score)

	ts.Events.TipAdded.Trigger(tip)

	// search all referenced tails of this Tip and remove them from the tip pool
	approveeTailTxHashes, err := dag.FindAllTails(tailTxHash, true)
	if err != nil {
		return
	}

	for approveeTailTxHash := range approveeTailTxHashes {
		if approveeTip, exists := ts.tipsMap[approveeTailTxHash]; exists {
			// check if the maximum amount of approvers for this tip is reached
			if approveeTip.ApproversCount.Add(1) >= ts.maxApprovers {
				ts.removeTipWithoutLocking(hornet.Hash(approveeTailTxHash))
				continue
			}

			// check if the tip was referenced by another transaction before
			if approveeTip.TimeFirstApprover.IsZero() {
				approveeTip.TimeFirstApprover = time.Now()

				// remove the tip after it reaches its maximum age
				time.AfterFunc(ts.maxReferencedTipAgeSeconds, func() {
					ts.RemoveTip(hornet.Hash(approveeTailTxHash))
				})
			}
		}
	}
}

// removeTipWithoutLocking removes the given tailTxHash from the tipsMap without acquiring the lock.
func (ts *TipSelector) removeTipWithoutLocking(tailTxHash hornet.Hash) {
	if tip, exists := ts.tipsMap[string(tailTxHash)]; exists {
		ts.scoreSum -= int(tip.Score)
		delete(ts.tipsMap, string(tailTxHash))
		ts.Events.TipRemoved.Trigger(tip)
	}
}

// RemoveTip removes the given tailTxHash from the tipsMap.
func (ts *TipSelector) RemoveTip(tailTxHash hornet.Hash) {
	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	ts.removeTipWithoutLocking(tailTxHash)
}

// TipCount returns the amount of current tips.
func (ts *TipSelector) TipCount() int {
	ts.tipsLock.RLock()
	defer ts.tipsLock.RUnlock()

	return len(ts.tipsMap)
}

// selectTip selects a tip.
func (ts *TipSelector) selectTip() (hornet.Hash, error) {

	// record stats
	start := time.Now()
	tipSelStats := &TipSelStats{}

	tipSelStats.Duration = time.Since(start)
	ts.Events.TipSelPerformed.Trigger(tipSelStats)

	if !tangle.IsNodeSyncedWithThreshold() {
		return nil, tangle.ErrNodeNotSynced
	}

	ts.tipsLock.RLock()
	defer ts.tipsLock.RUnlock()

	if ts.scoreSum == 0 {
		// no semi-/non-lazy tips available
		return nil, ErrNoTipsAvailable
	}

	// get a random number between 1 and the score sum
	randScore := utils.RandomInsecure(1, ts.scoreSum)

	// iterate over the tipsMap and subtract each tip's score from randScore
	for _, tip := range ts.tipsMap {
		// subtract the tip's score from randScore
		randScore -= int(tip.Score)

		// if randScore reaches zero or below, we return the given tip
		if randScore <= 0 {
			return tip.Hash, nil
		}
	}

	// no tips
	return nil, ErrNoTipsAvailable
}

// SelectTips selects two tips.
func (ts *TipSelector) SelectTips() (hornet.Hashes, error) {
	tips := hornet.Hashes{}

	trunk, err := ts.selectTip()
	if err != nil {
		return nil, err
	}
	tips = append(tips, trunk)

	// retry the tipselection several times if trunk and branch are equal
	for i := 0; i < 10; i++ {
		branch, err := ts.selectTip()
		if err != nil {
			return nil, err
		}

		if !bytes.Equal(trunk, branch) {
			tips = append(tips, branch)
			return tips, nil
		}
	}

	// no second tip found, use the same again
	tips = append(tips, trunk)
	return tips, nil
}

func (ts *TipSelector) updateOutdatedRootSnapshotIndexes(outdatedTransactions hornet.Hashes) {
	for i := len(outdatedTransactions) - 1; i >= 0; i-- {
		outdatedTxHash := outdatedTransactions[i]

		cachedTx := tangle.GetCachedTransactionOrNil(outdatedTxHash)
		if cachedTx == nil {
			panic(tangle.ErrTransactionNotFound)
		}
		ts.getTransactionRootSnapshotIndexes(cachedTx)
	}
}

// getTransactionRootSnapshotIndexes searches the root snapshot indexes for a given transaction.
func (ts *TipSelector) getTransactionRootSnapshotIndexes(cachedTx *tangle.CachedTransaction) (youngestTxRootSnapshotIndex milestone.Index, oldestTxRootSnapshotIndex milestone.Index) {
	defer cachedTx.Release() // tx -1

	// if the tx already contains recent (calculation index matches LSMI)
	// information about yrtsi and ortsi, return that info
	yrtsi, ortsi, rtsci := cachedTx.GetMetadata().GetRootSnapshotIndexes()
	if rtsci == tangle.GetSolidMilestoneIndex() {
		return yrtsi, ortsi
	}

	snapshotInfo := tangle.GetSnapshotInfo()

	youngestTxRootSnapshotIndex = 0
	oldestTxRootSnapshotIndex = 0

	updateIndexes := func(yrtsi milestone.Index, ortsi milestone.Index) {
		if (youngestTxRootSnapshotIndex == 0) || (youngestTxRootSnapshotIndex < yrtsi) {
			youngestTxRootSnapshotIndex = yrtsi
		}
		if (oldestTxRootSnapshotIndex == 0) || (oldestTxRootSnapshotIndex > ortsi) {
			oldestTxRootSnapshotIndex = ortsi
		}
	}

	// collect all approvees in the cone that are not confirmed,
	// are no solid entry points and have no recent calculation index
	var outdatedTransactions hornet.Hashes

	if err := dag.TraverseApprovees(cachedTx.GetMetadata().GetTxHash(),
		// traversal stops if no more transactions pass the given condition
		func(cachedTx *tangle.CachedTransaction) (bool, error) { // tx +1
			defer cachedTx.Release(true) // tx -1

			// if the tx contains recent (calculation index matches LSMI) information
			// about yrtsi and ortsi (uncofirmed or confirmed), propagate that info
			// no need to check confirmation index, since all confirmed txs get updated with yrtsi and ortsi equal confirmation index at confirmation
			yrtsi, ortsi, rtsci := cachedTx.GetMetadata().GetRootSnapshotIndexes()
			if rtsci == tangle.GetSolidMilestoneIndex() {
				updateIndexes(yrtsi, ortsi)
				return false, nil
			}

			outdatedTransactions = append(outdatedTransactions, cachedTx.GetTransaction().GetTxHash())

			return true, nil
		},
		// consumer
		func(cachedTx *tangle.CachedTransaction) error { // tx +1
			defer cachedTx.Release(true) // tx -1
			return nil
		},
		// called on missing approvees
		func(approveeHash hornet.Hash) error {
			return fmt.Errorf("missing approvee %v", approveeHash.Trytes())
		},
		// called on solid entry points
		func(txHash hornet.Hash) {
			updateIndexes(snapshotInfo.EntryPointIndex, snapshotInfo.EntryPointIndex)
		}, true, false, nil); err != nil {
		panic(err)
	}

	// update the outdated root snapshot indexes of all transactions in the cone in order from oldest txs to latest
	ts.updateOutdatedRootSnapshotIndexes(outdatedTransactions)

	// set the new transaction root snapshot indexes in the metadata of the transaction
	cachedTx.GetMetadata().SetRootSnapshotIndexes(youngestTxRootSnapshotIndex, oldestTxRootSnapshotIndex, tangle.GetSolidMilestoneIndex())

	return youngestTxRootSnapshotIndex, oldestTxRootSnapshotIndex
}

// calculateScore calculates the tip selection score of this transaction
func (ts *TipSelector) calculateScore(txHash hornet.Hash) Score {
	lsmi := tangle.GetSolidMilestoneIndex()

	cachedTx := tangle.GetCachedTransactionOrNil(txHash) // tx +1
	if cachedTx == nil {
		panic(fmt.Sprintf("transaction not found: %v", txHash.Trytes()))
	}
	defer cachedTx.Release()

	ytrsi, ortsi := ts.getTransactionRootSnapshotIndexes(cachedTx.Retain()) // tx +1

	// if the LSMI to YTRSI delta is over MaxDeltaTxYoungestRootSnapshotIndexToLSMI, then the tip is lazy
	if (lsmi - ytrsi) > ts.maxDeltaTxYoungestRootSnapshotIndexToLSMI {
		return ScoreLazy
	}

	// if the OTRSI to LSMI delta is over BelowMaxDepth/below-max-depth, then the tip is lazy
	if (lsmi - ortsi) > ts.belowMaxDepth {
		return ScoreLazy
	}

	// the approvees (trunk and branch) are the transactions this tip approves
	approveeHashes := map[string]struct{}{
		string(cachedTx.GetTransaction().GetTrunkHash()):  {},
		string(cachedTx.GetTransaction().GetBranchHash()): {},
	}

	approveesLazy := 0
	for approveeHash := range approveeHashes {
		approveeScore := ts.calculateScore(hornet.Hash(approveeHash))

		// direct approvee is already lazy, therefore so is this tip
		if approveeScore == ScoreLazy {
			return ScoreLazy
		}

		cachedApproveeTx := tangle.GetCachedTransactionOrNil(hornet.Hash(approveeHash)) // tx +1
		if cachedApproveeTx == nil {
			panic(fmt.Sprintf("transaction not found: %v", hornet.Hash(approveeHash).Trytes()))
		}

		_, approveeORTSI := ts.getTransactionRootSnapshotIndexes(cachedApproveeTx.Retain()) // tx +1
		cachedApproveeTx.Release()

		// if the OTRSI to LSMI delta of the approvee is MaxDeltaTxApproveesOldestRootSnapshotIndexToLSMI, we mark it as such
		if lsmi-approveeORTSI > ts.maxDeltaTxApproveesOldestRootSnapshotIndexToLSMI {
			approveesLazy++
		}
	}

	// if all available approvees' OTRSI violates the LSMI delta in relation to C2 the tip is lazy too
	if len(approveeHashes) == approveesLazy {
		return ScoreLazy
	}

	// if only one of the approvees violated the OTRSI to LMSI delta, the tip is considered semi-lazy
	if approveesLazy == 1 {
		return ScoreSemiLazy
	}

	return ScoreNonLazy
}
