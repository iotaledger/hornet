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
	"github.com/gohornet/hornet/pkg/metrics"
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
	// The duration of the tip-selection for a single tip.
	Duration time.Duration `json:"duration"`
	// The amount of lazy tips found and removed during the tip-selection.
	LazyTips int `json:"lazy_tips"`
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
	// ScoreLazy is a lazy tip and should not be selected.
	ScoreLazy Score = iota
	// ScoreSemiLazy is a somewhat lazy tip.
	ScoreSemiLazy
	// ScoreNonLazy is a non-lazy tip.
	ScoreNonLazy
)

var (
	// ErrNoTipsAvailable is returned when no tips are available in the node.
	ErrNoTipsAvailable = errors.New("no tips available")
	// ErrTipLazy is returned when the choosen tip was lazy already.
	ErrTipLazy = errors.New("tip already lazy")
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
	// retentionRulesTipsLimit is the maximum amount of current tips for which "maxReferencedTipAgeSeconds"
	// and "maxApprovers" are checked. if the amount of tips exceeds this limit,
	// referenced tips get removed directly to reduce the amount of tips in the network. (non-lazy pool)
	retentionRulesTipsLimitNonLazy int
	// maxReferencedTipAgeSeconds is the maximum time a tip remains in the tip pool
	// after it was referenced by the first transaction.
	// this is used to widen the cone of the tangle. (non-lazy pool)
	maxReferencedTipAgeSecondsNonLazy time.Duration
	// maxApprovers is the maximum amount of references by other transactions
	// before the tip is removed from the tip pool.
	// this is used to widen the cone of the tangle. (non-lazy pool)
	maxApproversNonLazy uint32
	// retentionRulesTipsLimit is the maximum amount of current tips for which "maxReferencedTipAgeSeconds"
	// and "maxApprovers" are checked. if the amount of tips exceeds this limit,
	// referenced tips get removed directly to reduce the amount of tips in the network. (semi-lazy pool)
	retentionRulesTipsLimitSemiLazy int
	// maxReferencedTipAgeSeconds is the maximum time a tip remains in the tip pool
	// after it was referenced by the first transaction.
	// this is used to widen the cone of the tangle. (semi-lazy pool)
	maxReferencedTipAgeSecondsSemiLazy time.Duration
	// maxApprovers is the maximum amount of references by other transactions
	// before the tip is removed from the tip pool.
	// this is used to widen the cone of the tangle. (semi-lazy pool)
	maxApproversSemiLazy uint32
	// nonLazyTipsMap contains only non-lazy tips.
	nonLazyTipsMap map[string]*Tip
	// semiLazyTipsMap contains only semi-lazy tips.
	semiLazyTipsMap map[string]*Tip
	// lock for the tipsMaps
	tipsLock syncutils.Mutex
	// Events are the events that are triggered by the TipSelector.
	Events Events
}

// New creates a new tip-selector.
func New(maxDeltaTxYoungestRootSnapshotIndexToLSMI int,
	maxDeltaTxApproveesOldestRootSnapshotIndexToLSMI int,
	belowMaxDepth int,
	retentionRulesTipsLimitNonLazy int,
	maxReferencedTipAgeSecondsNonLazy time.Duration,
	maxApproversNonLazy uint32,
	retentionRulesTipsLimitSemiLazy int,
	maxReferencedTipAgeSecondsSemiLazy time.Duration,
	maxApproversSemiLazy uint32) *TipSelector {

	return &TipSelector{
		maxDeltaTxYoungestRootSnapshotIndexToLSMI:        milestone.Index(maxDeltaTxYoungestRootSnapshotIndexToLSMI),
		maxDeltaTxApproveesOldestRootSnapshotIndexToLSMI: milestone.Index(maxDeltaTxApproveesOldestRootSnapshotIndexToLSMI),
		belowMaxDepth:                      milestone.Index(belowMaxDepth),
		retentionRulesTipsLimitNonLazy:     retentionRulesTipsLimitNonLazy,
		maxReferencedTipAgeSecondsNonLazy:  maxReferencedTipAgeSecondsNonLazy,
		maxApproversNonLazy:                maxApproversNonLazy,
		retentionRulesTipsLimitSemiLazy:    retentionRulesTipsLimitSemiLazy,
		maxReferencedTipAgeSecondsSemiLazy: maxReferencedTipAgeSecondsSemiLazy,
		maxApproversSemiLazy:               maxApproversSemiLazy,
		nonLazyTipsMap:                     make(map[string]*Tip),
		semiLazyTipsMap:                    make(map[string]*Tip),
		Events: Events{
			TipAdded:        events.NewEvent(TipCaller),
			TipRemoved:      events.NewEvent(TipCaller),
			TipSelPerformed: events.NewEvent(WalkerStatsCaller),
		},
	}
}

// AddTip adds the given tailTxHash as a tip.
func (ts *TipSelector) AddTip(bndl *tangle.Bundle) {
	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	tailTxHash := bndl.GetTailHash()

	if _, exists := ts.nonLazyTipsMap[string(tailTxHash)]; exists {
		// tip already exists
		return
	}

	if _, exists := ts.semiLazyTipsMap[string(tailTxHash)]; exists {
		// tip already exists
		return
	}

	lsmi := tangle.GetSolidMilestoneIndex()

	score := ts.calculateScore(tailTxHash, lsmi)
	if score == ScoreLazy {
		// do not add lazy tips.
		// lazy tips should also not remove other tips from the pool, otherwise the tip pool will run empty.
		return
	}

	tip := &Tip{
		Score:             score,
		Hash:              tailTxHash,
		TimeFirstApprover: time.Time{},
		ApproversCount:    atomic.NewUint32(0),
	}

	switch tip.Score {
	case ScoreNonLazy:
		ts.nonLazyTipsMap[string(tailTxHash)] = tip
		metrics.SharedServerMetrics.TipsNonLazy.Add(1)
	case ScoreSemiLazy:
		ts.semiLazyTipsMap[string(tailTxHash)] = tip
		metrics.SharedServerMetrics.TipsSemiLazy.Add(1)
	}

	ts.Events.TipAdded.Trigger(tip)

	// the approvees (trunk and branch) are the tail transactions this tip approves
	// remove them from the tip pool
	approveeTailTxHashes := map[string]struct{}{
		string(bndl.GetTrunk(true)):  {},
		string(bndl.GetBranch(true)): {},
	}

	checkTip := func(tipsMap map[string]*Tip, approveeTip *Tip, retentionRulesTipsLimit int, maxApprovers uint32, maxReferencedTipAgeSeconds time.Duration) bool {
		// if the amount of known tips is above the limit, remove the tip directly
		if len(tipsMap) > retentionRulesTipsLimit {
			return ts.removeTipWithoutLocking(tipsMap, hornet.Hash(approveeTip.Hash))
		}

		// check if the maximum amount of approvers for this tip is reached
		if approveeTip.ApproversCount.Add(1) >= maxApprovers {
			return ts.removeTipWithoutLocking(tipsMap, hornet.Hash(approveeTip.Hash))
		}

		if maxReferencedTipAgeSeconds == time.Duration(0) {
			// check for maxReferenceTipAge is disabled
			return false
		}

		// check if the tip was referenced by another transaction before
		if approveeTip.TimeFirstApprover.IsZero() {
			// mark the tip as referenced
			approveeTip.TimeFirstApprover = time.Now()
		}

		return false
	}

	for approveeTailTxHash := range approveeTailTxHashes {
		// we have to separate between the pools, to prevent semi-lazy tips from emptying the non-lazy pool
		switch tip.Score {
		case ScoreNonLazy:
			if approveeTip, exists := ts.nonLazyTipsMap[approveeTailTxHash]; exists {
				if checkTip(ts.nonLazyTipsMap, approveeTip, ts.retentionRulesTipsLimitNonLazy, ts.maxApproversNonLazy, ts.maxReferencedTipAgeSecondsNonLazy) {
					metrics.SharedServerMetrics.TipsNonLazy.Sub(1)
				}
			}
		case ScoreSemiLazy:
			if approveeTip, exists := ts.semiLazyTipsMap[approveeTailTxHash]; exists {
				if checkTip(ts.semiLazyTipsMap, approveeTip, ts.retentionRulesTipsLimitSemiLazy, ts.maxApproversSemiLazy, ts.maxReferencedTipAgeSecondsSemiLazy) {
					metrics.SharedServerMetrics.TipsSemiLazy.Sub(1)
				}
			}
		}
	}
}

// removeTipWithoutLocking removes the given tailTxHash from the tipsMap without acquiring the lock.
func (ts *TipSelector) removeTipWithoutLocking(tipsMap map[string]*Tip, tailTxHash hornet.Hash) bool {
	if tip, exists := tipsMap[string(tailTxHash)]; exists {
		delete(tipsMap, string(tailTxHash))
		ts.Events.TipRemoved.Trigger(tip)
		return true
	}
	return false
}

// randomTipWithoutLocking picks a random tip from the pool and checks it's "own" score again without acquiring the lock.
func (ts *TipSelector) randomTipWithoutLocking(tipsMap map[string]*Tip) (hornet.Hash, error) {

	if len(tipsMap) == 0 {
		// no semi-/non-lazy tips available
		return nil, ErrNoTipsAvailable
	}

	// get a random number between 1 and the amount of tips
	randTip := utils.RandomInsecure(0, len(tipsMap)-1)

	// iterate over the tipsMap and subtract each tip from randTip
	for _, tip := range tipsMap {
		// subtract the tip from randTip
		randTip--

		// if randTip is below zero, we return the given tip
		if randTip < 0 {
			return tip.Hash, nil
		}
	}

	// no tips
	return nil, ErrNoTipsAvailable
}

// selectTipWithoutLocking selects a tip.
func (ts *TipSelector) selectTipWithoutLocking(tipsMap map[string]*Tip) (hornet.Hash, error) {

	if !tangle.IsNodeSyncedWithThreshold() {
		return nil, tangle.ErrNodeNotSynced
	}

	// record stats
	start := time.Now()

	tipHash, err := ts.randomTipWithoutLocking(tipsMap)
	ts.Events.TipSelPerformed.Trigger(&TipSelStats{Duration: time.Since(start)})

	return tipHash, err
}

// SelectTips selects two tips.
func (ts *TipSelector) selectTips(tipsMap map[string]*Tip) (hornet.Hashes, error) {
	tips := hornet.Hashes{}

	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	trunk, err := ts.selectTipWithoutLocking(tipsMap)
	if err != nil {
		return nil, err
	}
	tips = append(tips, trunk)

	// retry the tipselection several times if trunk and branch are equal
	for i := 0; i < 10; i++ {
		branch, err := ts.selectTipWithoutLocking(tipsMap)
		if err != nil {
			if err == ErrNoTipsAvailable {
				// do not search other tips if there are none
				break
			}
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

// SelectSemiLazyTips selects two semi-lazy tips.
func (ts *TipSelector) SelectSemiLazyTips() (hornet.Hashes, error) {
	return ts.selectTips(ts.semiLazyTipsMap)
}

// SelectNonLazyTips selects two non-lazy tips.
func (ts *TipSelector) SelectNonLazyTips() (hornet.Hashes, error) {
	return ts.selectTips(ts.nonLazyTipsMap)
}

// CleanUpReferencedTips checks if tips were referenced before
// and removes them if they reached their maximum age.
func (ts *TipSelector) CleanUpReferencedTips() int {

	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	checkTip := func(tipsMap map[string]*Tip, tip *Tip, maxReferencedTipAgeSeconds time.Duration) bool {
		if tip.TimeFirstApprover.IsZero() {
			// not referenced by another transaction
			return false
		}

		// check if the tip reached its maximum age
		if time.Since(tip.TimeFirstApprover) < maxReferencedTipAgeSeconds {
			return false
		}

		// remove the tip from the pool because it is outdated
		return ts.removeTipWithoutLocking(tipsMap, tip.Hash)
	}

	count := 0
	for _, tip := range ts.nonLazyTipsMap {
		if checkTip(ts.nonLazyTipsMap, tip, ts.maxReferencedTipAgeSecondsNonLazy) {
			metrics.SharedServerMetrics.TipsNonLazy.Sub(1)
			count++
		}
	}
	for _, tip := range ts.semiLazyTipsMap {
		if checkTip(ts.semiLazyTipsMap, tip, ts.maxReferencedTipAgeSecondsSemiLazy) {
			metrics.SharedServerMetrics.TipsSemiLazy.Sub(1)
			count++
		}
	}

	return count
}

// UpdateScores updates the scores of the tips and removes lazy ones.
func (ts *TipSelector) UpdateScores() int {

	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	lsmi := tangle.GetSolidMilestoneIndex()

	count := 0
	for _, tip := range ts.nonLazyTipsMap {
		// check the score of the tip again to avoid old tips
		tip.Score = ts.calculateScore(tip.Hash, lsmi)
		if tip.Score == ScoreLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.nonLazyTipsMap, tip.Hash) {
				count++
				metrics.SharedServerMetrics.TipsNonLazy.Sub(1)
			}
			continue
		}

		if tip.Score == ScoreSemiLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.nonLazyTipsMap, tip.Hash) {
				count++
				metrics.SharedServerMetrics.TipsNonLazy.Sub(1)
			}
			// add the tip to the semi-lazy tips map
			ts.semiLazyTipsMap[string(tip.Hash)] = tip
			ts.Events.TipAdded.Trigger(tip)
			metrics.SharedServerMetrics.TipsSemiLazy.Add(1)
			count--
		}
	}

	for _, tip := range ts.semiLazyTipsMap {
		// check the score of the tip again to avoid old tips
		tip.Score = ts.calculateScore(tip.Hash, lsmi)
		if tip.Score == ScoreLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.semiLazyTipsMap, tip.Hash) {
				count++
				metrics.SharedServerMetrics.TipsSemiLazy.Sub(1)
			}
			continue
		}

		if tip.Score == ScoreNonLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.semiLazyTipsMap, tip.Hash) {
				count++
				metrics.SharedServerMetrics.TipsSemiLazy.Sub(1)
			}
			// add the tip to the non-lazy tips map
			ts.nonLazyTipsMap[string(tip.Hash)] = tip
			ts.Events.TipAdded.Trigger(tip)
			metrics.SharedServerMetrics.TipsNonLazy.Add(1)
			count--
		}
	}

	return count
}

// calculateScore calculates the tip selection score of this transaction
func (ts *TipSelector) calculateScore(txHash hornet.Hash, lsmi milestone.Index) Score {
	cachedTx := tangle.GetCachedTransactionOrNil(txHash) // tx +1
	if cachedTx == nil {
		panic(fmt.Errorf("%w: %v", tangle.ErrTransactionNotFound, txHash.Trytes()))
	}
	defer cachedTx.Release(true)

	ytrsi, ortsi := dag.GetTransactionRootSnapshotIndexes(cachedTx.Retain(), lsmi) // tx +1

	// if the LSMI to YTRSI delta is over MaxDeltaTxYoungestRootSnapshotIndexToLSMI, then the tip is lazy
	if (lsmi - ytrsi) > ts.maxDeltaTxYoungestRootSnapshotIndexToLSMI {
		return ScoreLazy
	}

	// if the OTRSI to LSMI delta is over BelowMaxDepth/below-max-depth, then the tip is lazy
	if (lsmi - ortsi) > ts.belowMaxDepth {
		return ScoreLazy
	}

	cachedBundle := tangle.GetCachedBundleOrNil(txHash) // bundle +1
	if cachedBundle == nil {
		panic(fmt.Errorf("%w: bundle %s of tx %s doesn't exist", tangle.ErrBundleNotFound, cachedTx.GetTransaction().Tx.Bundle, txHash.Trytes()))
	}
	defer cachedBundle.Release(true)

	// the approvees (trunk and branch) are the tail transactions this tip approves
	approveeHashes := map[string]struct{}{
		string(cachedBundle.GetBundle().GetTrunk(true)):  {},
		string(cachedBundle.GetBundle().GetBranch(true)): {},
	}

	for approveeHash := range approveeHashes {
		var approveeORTSI milestone.Index

		if tangle.SolidEntryPointsContain(hornet.Hash(approveeHash)) {
			// if the approvee is an solid entry point, use the EntryPointIndex as ORTSI
			approveeORTSI = tangle.GetSnapshotInfo().EntryPointIndex
		} else {
			cachedApproveeTx := tangle.GetCachedTransactionOrNil(hornet.Hash(approveeHash)) // tx +1
			if cachedApproveeTx == nil {
				panic(fmt.Sprintf("transaction not found: %v", hornet.Hash(approveeHash).Trytes()))
			}

			_, approveeORTSI = dag.GetTransactionRootSnapshotIndexes(cachedApproveeTx.Retain(), lsmi) // tx +1
			cachedApproveeTx.Release(true)
		}

		// if the OTRSI to LSMI delta of the approvee is MaxDeltaTxApproveesOldestRootSnapshotIndexToLSMI, the tip is semi-lazy
		if lsmi-approveeORTSI > ts.maxDeltaTxApproveesOldestRootSnapshotIndexToLSMI {
			return ScoreSemiLazy
		}
	}

	return ScoreNonLazy
}
