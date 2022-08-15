package tipselect

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/atomic"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	iotago "github.com/iotaledger/iota.go/v3"
)

// Score defines the score of a tip.
type Score int

// TipSelectionFunc is a function which performs a tipselection and returns tips.
type TipSelectionFunc = func() (iotago.BlockIDs, error)

// TipSelStats holds the stats for a tipselection run.
type TipSelStats struct {
	// The duration of the tip-selection for a single tip.
	Duration time.Duration `json:"duration"`
}

// TipCaller is used to signal tip events.
func TipCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(*Tip))(params[0].(*Tip))
}

// WalkerStatsCaller is used to signal tip selection events.
func WalkerStatsCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
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
)

// Tip defines a tip.
type Tip struct {
	// Score is the score of the tip.
	Score Score
	// BlockID is the block ID of the tip.
	BlockID iotago.BlockID
	// TimeFirstChild is the timestamp the tip was referenced for the first time by another block.
	TimeFirstChild time.Time
	// ChildrenCount is the amount the tip was referenced by other blocks.
	ChildrenCount *atomic.Uint32
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
	// context that is done when the node is shutting down.
	shutdownCtx context.Context
	// tipScoreCalculator is used to calculate the tip score.
	tipScoreCalculator *tangle.TipScoreCalculator
	// used to determine the sync status of the node
	syncManager *syncmanager.SyncManager
	// serverMetrics is the shared server metrics instance.
	serverMetrics *metrics.ServerMetrics
	// retentionRulesTipsLimitNonLazy is the maximum amount of current tips for which "maxReferencedTipAgeNonLazy"
	// and "maxChildren" are checked. if the amount of tips exceeds this limit,
	// referenced tips get removed directly to reduce the amount of tips in the network. (non-lazy pool)
	retentionRulesTipsLimitNonLazy int
	// maxReferencedTipAgeNonLazy is the maximum time a tip remains in the tip pool
	// after it was referenced by the first block.
	// this is used to widen the cone of the tangle. (non-lazy pool)
	maxReferencedTipAgeNonLazy time.Duration
	// maxChildrenNonLazy is the maximum amount of references by other blocks
	// before the tip is removed from the tip pool.
	// this is used to widen the cone of the tangle. (non-lazy pool)
	maxChildrenNonLazy uint32
	// retentionRulesTipsLimitSemiLazy is the maximum amount of current tips for which "maxReferencedTipAgeSemiLazy"
	// and "maxChildren" are checked. if the amount of tips exceeds this limit,
	// referenced tips get removed directly to reduce the amount of tips in the network. (semi-lazy pool)
	retentionRulesTipsLimitSemiLazy int
	// maxReferencedTipAgeSemiLazy is the maximum time a tip remains in the tip pool
	// after it was referenced by the first block.
	// this is used to widen the cone of the tangle. (semi-lazy pool)
	maxReferencedTipAgeSemiLazy time.Duration
	// maxChildrenSemiLazy is the maximum amount of references by other blocks
	// before the tip is removed from the tip pool.
	// this is used to widen the cone of the tangle. (semi-lazy pool)
	maxChildrenSemiLazy uint32
	// nonLazyTipsMap contains only non-lazy tips.
	nonLazyTipsMap map[iotago.BlockID]*Tip
	// semiLazyTipsMap contains only semi-lazy tips.
	semiLazyTipsMap map[iotago.BlockID]*Tip
	// lock for the tipsMaps
	tipsLock syncutils.Mutex
	// Events are the events that are triggered by the TipSelector.
	Events *Events
}

// New creates a new tip-selector.
func New(
	shutdownCtx context.Context,
	tipScoreCalculator *tangle.TipScoreCalculator,
	syncManager *syncmanager.SyncManager,
	serverMetrics *metrics.ServerMetrics,
	retentionRulesTipsLimitNonLazy int,
	maxReferencedTipAgeNonLazy time.Duration,
	maxChildrenNonLazy uint32,
	retentionRulesTipsLimitSemiLazy int,
	maxReferencedTipAgeSemiLazy time.Duration,
	maxChildrenSemiLazy uint32) *TipSelector {

	return &TipSelector{
		shutdownCtx:                     shutdownCtx,
		tipScoreCalculator:              tipScoreCalculator,
		syncManager:                     syncManager,
		serverMetrics:                   serverMetrics,
		retentionRulesTipsLimitNonLazy:  retentionRulesTipsLimitNonLazy,
		maxReferencedTipAgeNonLazy:      maxReferencedTipAgeNonLazy,
		maxChildrenNonLazy:              maxChildrenNonLazy,
		retentionRulesTipsLimitSemiLazy: retentionRulesTipsLimitSemiLazy,
		maxReferencedTipAgeSemiLazy:     maxReferencedTipAgeSemiLazy,
		maxChildrenSemiLazy:             maxChildrenSemiLazy,
		nonLazyTipsMap:                  make(map[iotago.BlockID]*Tip),
		semiLazyTipsMap:                 make(map[iotago.BlockID]*Tip),
		Events: &Events{
			TipAdded:        events.NewEvent(TipCaller),
			TipRemoved:      events.NewEvent(TipCaller),
			TipSelPerformed: events.NewEvent(WalkerStatsCaller),
		},
	}
}

// AddTip adds the given block as a tip.
func (ts *TipSelector) AddTip(blockMeta *storage.BlockMetadata) {
	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	blockID := blockMeta.BlockID()

	if _, exists := ts.nonLazyTipsMap[blockID]; exists {
		// tip already exists
		return
	}

	if _, exists := ts.semiLazyTipsMap[blockID]; exists {
		// tip already exists
		return
	}

	cmi := ts.syncManager.ConfirmedMilestoneIndex()

	score, err := ts.calculateScore(blockID, cmi)
	if err != nil {
		// do not add tips if the calculation failed
		return
	}

	if score == ScoreLazy {
		// do not add lazy tips.
		// lazy tips should also not remove other tips from the pool, otherwise the tip pool will run empty.
		return
	}

	tip := &Tip{
		Score:          score,
		BlockID:        blockID,
		TimeFirstChild: time.Time{},
		ChildrenCount:  atomic.NewUint32(0),
	}

	switch tip.Score {
	case ScoreNonLazy:
		ts.nonLazyTipsMap[blockID] = tip
		ts.serverMetrics.TipsNonLazy.Add(1)
	case ScoreSemiLazy:
		ts.semiLazyTipsMap[blockID] = tip
		ts.serverMetrics.TipsSemiLazy.Add(1)
	}

	ts.Events.TipAdded.Trigger(tip)

	// the parents are the blocks this tip approves
	// remove them from the tip pool
	parentBlockIDs := map[iotago.BlockID]struct{}{}
	for _, parent := range blockMeta.Parents() {
		parentBlockIDs[parent] = struct{}{}
	}

	checkTip := func(tipsMap map[iotago.BlockID]*Tip, parentTip *Tip, retentionRulesTipsLimit int, maxChildren uint32, maxReferencedTipAge time.Duration) bool {
		// if the amount of known tips is above the limit, remove the tip directly
		if len(tipsMap) > retentionRulesTipsLimit {
			return ts.removeTipWithoutLocking(tipsMap, parentTip.BlockID)
		}

		// check if the maximum amount of children for this tip is reached
		if parentTip.ChildrenCount.Add(1) >= maxChildren {
			return ts.removeTipWithoutLocking(tipsMap, parentTip.BlockID)
		}

		if maxReferencedTipAge == time.Duration(0) {
			// check for maxReferenceTipAge is disabled
			return false
		}

		// check if the tip was referenced by another block before
		if parentTip.TimeFirstChild.IsZero() {
			// mark the tip as referenced
			parentTip.TimeFirstChild = time.Now()
		}

		return false
	}

	for parentBlockID := range parentBlockIDs {
		// we have to separate between the pools, to prevent semi-lazy tips from emptying the non-lazy pool
		switch tip.Score {
		case ScoreNonLazy:
			if parentTip, exists := ts.nonLazyTipsMap[parentBlockID]; exists {
				if checkTip(ts.nonLazyTipsMap, parentTip, ts.retentionRulesTipsLimitNonLazy, ts.maxChildrenNonLazy, ts.maxReferencedTipAgeNonLazy) {
					ts.serverMetrics.TipsNonLazy.Sub(1)
				}
			}
		case ScoreSemiLazy:
			if parentTip, exists := ts.semiLazyTipsMap[parentBlockID]; exists {
				if checkTip(ts.semiLazyTipsMap, parentTip, ts.retentionRulesTipsLimitSemiLazy, ts.maxChildrenSemiLazy, ts.maxReferencedTipAgeSemiLazy) {
					ts.serverMetrics.TipsSemiLazy.Sub(1)
				}
			}
		}
	}
}

// removeTipWithoutLocking removes the given block from the tipsMap without acquiring the lock.
func (ts *TipSelector) removeTipWithoutLocking(tipsMap map[iotago.BlockID]*Tip, blockID iotago.BlockID) bool {
	if tip, exists := tipsMap[blockID]; exists {
		delete(tipsMap, blockID)
		ts.Events.TipRemoved.Trigger(tip)

		return true
	}

	return false
}

// randomTipWithoutLocking picks a random tip from the pool and checks it's "own" score again without acquiring the lock.
func (ts *TipSelector) randomTipWithoutLocking(tipsMap map[iotago.BlockID]*Tip) (iotago.BlockID, error) {

	if len(tipsMap) == 0 {
		// no semi-/non-lazy tips available
		return iotago.EmptyBlockID(), ErrNoTipsAvailable
	}

	// get a random number between 0 and the amount of tips-1
	randTip := RandomInsecure(0, len(tipsMap)-1)

	// iterate over the tipsMap and subtract each tip from randTip
	for _, tip := range tipsMap {
		// subtract the tip from randTip
		randTip--

		// if randTip is below zero, we return the given tip
		if randTip < 0 {
			return tip.BlockID, nil
		}
	}

	// no tips
	return iotago.EmptyBlockID(), ErrNoTipsAvailable
}

// selectTipWithoutLocking selects a tip.
func (ts *TipSelector) selectTipWithoutLocking(tipsMap map[iotago.BlockID]*Tip) (iotago.BlockID, error) {

	if !ts.syncManager.IsNodeAlmostSynced() {
		return iotago.EmptyBlockID(), common.ErrNodeNotSynced
	}

	// record stats
	start := time.Now()

	tipBlockID, err := ts.randomTipWithoutLocking(tipsMap)
	ts.Events.TipSelPerformed.Trigger(&TipSelStats{Duration: time.Since(start)})

	return tipBlockID, err
}

// SelectTips selects multiple tips.
func (ts *TipSelector) selectTips(tipsMap map[iotago.BlockID]*Tip) (iotago.BlockIDs, error) {
	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	tipCount := ts.optimalTipCount()
	maxRetries := (tipCount - 1) * 10

	seen := make(map[iotago.BlockID]struct{})
	tips := iotago.BlockIDs{}

	// retry the tipselection several times if parents not unique
	for i := 0; i < maxRetries; i++ {
		tip, err := ts.selectTipWithoutLocking(tipsMap)
		if err != nil {
			if errors.Is(err, ErrNoTipsAvailable) && i != 0 {
				// do not search other tips if there are none
				// in case the first tip selection failed => return the error
				break
			}

			return nil, err
		}

		if _, has := seen[tip]; has {
			// ignore duplicates
			continue
		}
		seen[tip] = struct{}{}
		tips = append(tips, tip)

		if len(tips) >= tipCount {
			// collected enough tips
			break
		}
	}

	return tips.RemoveDupsAndSort(), nil
}

// optimalTipCount returns the optimal number of tips.
func (ts *TipSelector) optimalTipCount() int {
	// hardcoded until next PR
	return 4
}

// TipCount returns the current amount of available tips in the non-lazy and semi-lazy pool.
func (ts *TipSelector) TipCount() (int, int) {
	return len(ts.nonLazyTipsMap), len(ts.semiLazyTipsMap)
}

// SelectSemiLazyTips selects two semi-lazy tips.
func (ts *TipSelector) SelectSemiLazyTips() (iotago.BlockIDs, error) {
	return ts.selectTips(ts.semiLazyTipsMap)
}

// SelectNonLazyTips selects two non-lazy tips.
func (ts *TipSelector) SelectNonLazyTips() (iotago.BlockIDs, error) {
	return ts.selectTips(ts.nonLazyTipsMap)
}

// SelectTipsWithSemiLazyAllowed tries to select semi-lazy tips first,
// but uses non-lazy tips instead if not enough semi-lazy tips are found.
// This functionality may be useful for healthy spammers.
func (ts *TipSelector) SelectTipsWithSemiLazyAllowed() (tips iotago.BlockIDs, err error) {
	if len(ts.semiLazyTipsMap) > 2 {
		// return semi-lazy tips (e.g. for healthy spammers)
		tips, err = ts.SelectSemiLazyTips()
		if err != nil {
			return nil, fmt.Errorf("couldn't select semi-lazy tips: %w", err)
		}

		if len(tips) >= 2 {
			return tips, nil
		}

		// if the amount of tips is less than 2, creating a block with a single parent would
		// not reduce the semi-lazy count. Therefore we ignore the semi-lazy tips, and return
		// not-lazy tips instead.
	}

	tips, err = ts.SelectNonLazyTips()
	if err != nil {
		return tips, fmt.Errorf("couldn't select non-lazy tips: %w", err)
	}

	return tips, nil
}

// CleanUpReferencedTips checks if tips were referenced before
// and removes them if they reached their maximum age.
func (ts *TipSelector) CleanUpReferencedTips() int {

	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	checkTip := func(tipsMap map[iotago.BlockID]*Tip, tip *Tip, maxReferencedTipAge time.Duration) bool {
		if tip.TimeFirstChild.IsZero() {
			// not referenced by another block
			return false
		}

		// check if the tip reached its maximum age
		if time.Since(tip.TimeFirstChild) < maxReferencedTipAge {
			return false
		}

		// remove the tip from the pool because it is outdated
		return ts.removeTipWithoutLocking(tipsMap, tip.BlockID)
	}

	count := 0
	for _, tip := range ts.nonLazyTipsMap {
		if checkTip(ts.nonLazyTipsMap, tip, ts.maxReferencedTipAgeNonLazy) {
			ts.serverMetrics.TipsNonLazy.Sub(1)
			count++
		}
	}
	for _, tip := range ts.semiLazyTipsMap {
		if checkTip(ts.semiLazyTipsMap, tip, ts.maxReferencedTipAgeSemiLazy) {
			ts.serverMetrics.TipsSemiLazy.Sub(1)
			count++
		}
	}

	return count
}

// UpdateScores updates the scores of the tips and removes lazy ones.
func (ts *TipSelector) UpdateScores() (int, error) {

	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	cmi := ts.syncManager.ConfirmedMilestoneIndex()

	count := 0
	for _, tip := range ts.nonLazyTipsMap {
		// check the score of the tip again to avoid old tips
		score, err := ts.calculateScore(tip.BlockID, cmi)
		if err != nil {
			// do not continue if calculation of the tip score failed
			return count, err
		}
		tip.Score = score

		if tip.Score == ScoreLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.nonLazyTipsMap, tip.BlockID) {
				count++
				ts.serverMetrics.TipsNonLazy.Sub(1)
			}

			continue
		}

		if tip.Score == ScoreSemiLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.nonLazyTipsMap, tip.BlockID) {
				count++
				ts.serverMetrics.TipsNonLazy.Sub(1)
			}
			// add the tip to the semi-lazy tips map
			ts.semiLazyTipsMap[tip.BlockID] = tip
			ts.Events.TipAdded.Trigger(tip)
			ts.serverMetrics.TipsSemiLazy.Add(1)
			count--
		}
	}

	for _, tip := range ts.semiLazyTipsMap {
		// check the score of the tip again to avoid old tips
		score, err := ts.calculateScore(tip.BlockID, cmi)
		if err != nil {
			// do not continue if calculation of the tip score failed
			return count, err
		}
		tip.Score = score

		if tip.Score == ScoreLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.semiLazyTipsMap, tip.BlockID) {
				count++
				ts.serverMetrics.TipsSemiLazy.Sub(1)
			}

			continue
		}

		if tip.Score == ScoreNonLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.semiLazyTipsMap, tip.BlockID) {
				count++
				ts.serverMetrics.TipsSemiLazy.Sub(1)
			}
			// add the tip to the non-lazy tips map
			ts.nonLazyTipsMap[tip.BlockID] = tip
			ts.Events.TipAdded.Trigger(tip)
			ts.serverMetrics.TipsNonLazy.Add(1)
			count--
		}
	}

	return count, nil
}

// calculateScore calculates the tip selection score of this block.
func (ts *TipSelector) calculateScore(blockID iotago.BlockID, cmi iotago.MilestoneIndex) (Score, error) {

	tipScore, err := ts.tipScoreCalculator.TipScore(ts.shutdownCtx, blockID, cmi)
	if err != nil {
		return ScoreLazy, err
	}

	switch tipScore {
	case tangle.TipScoreNotFound:
		// we need to return lazy instead of panic here, because the block could have been pruned already
		// if the node was not sync for a longer time and after the pruning "UpdateScores" is called.
		return ScoreLazy, nil
	case tangle.TipScoreYCRIThresholdReached:
		return ScoreLazy, nil
	case tangle.TipScoreBelowMaxDepth:
		return ScoreLazy, nil
	case tangle.TipScoreOCRIThresholdReached:
		return ScoreSemiLazy, nil
	case tangle.TipScoreHealthy:
		return ScoreNonLazy, nil
	default:
		return ScoreLazy, nil
	}
}
