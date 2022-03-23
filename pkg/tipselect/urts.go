package tipselect

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/atomic"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/syncutils"
)

// Score defines the score of a tip.
type Score int

// TipSelectionFunc is a function which performs a tipselection and returns tips.
type TipSelectionFunc = func() (hornet.MessageIDs, error)

// TipSelStats holds the stats for a tipselection run.
type TipSelStats struct {
	// The duration of the tip-selection for a single tip.
	Duration time.Duration `json:"duration"`
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
	// ErrTipLazy is returned when the chosen tip was lazy already.
	ErrTipLazy = errors.New("tip already lazy")
)

// Tip defines a tip.
type Tip struct {
	// Score is the score of the tip.
	Score Score
	// MessageID is the message ID of the tip.
	MessageID hornet.MessageID
	// TimeFirstChild is the timestamp the tip was referenced for the first time by another message.
	TimeFirstChild time.Time
	// ChildrenCount is the amount the tip was referenced by other messages.
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
	// after it was referenced by the first message.
	// this is used to widen the cone of the tangle. (non-lazy pool)
	maxReferencedTipAgeNonLazy time.Duration
	// maxChildrenNonLazy is the maximum amount of references by other messages
	// before the tip is removed from the tip pool.
	// this is used to widen the cone of the tangle. (non-lazy pool)
	maxChildrenNonLazy uint32
	// spammerTipsThresholdNonLazy is the maximum amount of tips in a tip-pool before the spammer tries to reduce these (0 = always)
	// this is used to support the network if someone attacks the tangle by spamming a lot of tips. (non-lazy pool)
	spammerTipsThresholdNonLazy int
	// retentionRulesTipsLimitSemiLazy is the maximum amount of current tips for which "maxReferencedTipAgeSemiLazy"
	// and "maxChildren" are checked. if the amount of tips exceeds this limit,
	// referenced tips get removed directly to reduce the amount of tips in the network. (semi-lazy pool)
	retentionRulesTipsLimitSemiLazy int
	// maxReferencedTipAgeSemiLazy is the maximum time a tip remains in the tip pool
	// after it was referenced by the first message.
	// this is used to widen the cone of the tangle. (semi-lazy pool)
	maxReferencedTipAgeSemiLazy time.Duration
	// maxChildrenSemiLazy is the maximum amount of references by other messages
	// before the tip is removed from the tip pool.
	// this is used to widen the cone of the tangle. (semi-lazy pool)
	maxChildrenSemiLazy uint32
	// spammerTipsThresholdSemiLazy is the maximum amount of tips in a tip-pool before the spammer tries to reduce these (0 = disable)
	// this is used to support the network if someone attacks the tangle by spamming a lot of tips. (semi-lazy pool)
	spammerTipsThresholdSemiLazy int
	// nonLazyTipsMap contains only non-lazy tips.
	nonLazyTipsMap map[string]*Tip
	// semiLazyTipsMap contains only semi-lazy tips.
	semiLazyTipsMap map[string]*Tip
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
	spammerTipsThresholdNonLazy int,
	retentionRulesTipsLimitSemiLazy int,
	maxReferencedTipAgeSemiLazy time.Duration,
	maxChildrenSemiLazy uint32,
	spammerTipsThresholdSemiLazy int) *TipSelector {

	return &TipSelector{
		shutdownCtx:                     shutdownCtx,
		tipScoreCalculator:              tipScoreCalculator,
		syncManager:                     syncManager,
		serverMetrics:                   serverMetrics,
		retentionRulesTipsLimitNonLazy:  retentionRulesTipsLimitNonLazy,
		maxReferencedTipAgeNonLazy:      maxReferencedTipAgeNonLazy,
		maxChildrenNonLazy:              maxChildrenNonLazy,
		spammerTipsThresholdNonLazy:     spammerTipsThresholdNonLazy,
		retentionRulesTipsLimitSemiLazy: retentionRulesTipsLimitSemiLazy,
		maxReferencedTipAgeSemiLazy:     maxReferencedTipAgeSemiLazy,
		maxChildrenSemiLazy:             maxChildrenSemiLazy,
		spammerTipsThresholdSemiLazy:    spammerTipsThresholdSemiLazy,
		nonLazyTipsMap:                  make(map[string]*Tip),
		semiLazyTipsMap:                 make(map[string]*Tip),
		Events: &Events{
			TipAdded:        events.NewEvent(TipCaller),
			TipRemoved:      events.NewEvent(TipCaller),
			TipSelPerformed: events.NewEvent(WalkerStatsCaller),
		},
	}
}

// AddTip adds the given message as a tip.
func (ts *TipSelector) AddTip(messageMeta *storage.MessageMetadata) {
	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	messageID := messageMeta.MessageID()
	messageIDMapKey := messageID.ToMapKey()

	if _, exists := ts.nonLazyTipsMap[messageIDMapKey]; exists {
		// tip already exists
		return
	}

	if _, exists := ts.semiLazyTipsMap[messageIDMapKey]; exists {
		// tip already exists
		return
	}

	cmi := ts.syncManager.ConfirmedMilestoneIndex()

	score, err := ts.calculateScore(messageID, cmi)
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
		MessageID:      messageID,
		TimeFirstChild: time.Time{},
		ChildrenCount:  atomic.NewUint32(0),
	}

	switch tip.Score {
	case ScoreNonLazy:
		ts.nonLazyTipsMap[messageIDMapKey] = tip
		ts.serverMetrics.TipsNonLazy.Add(1)
	case ScoreSemiLazy:
		ts.semiLazyTipsMap[messageIDMapKey] = tip
		ts.serverMetrics.TipsSemiLazy.Add(1)
	}

	ts.Events.TipAdded.Trigger(tip)

	// the parents are the messages this tip approves
	// remove them from the tip pool
	parentMessageIDs := map[string]struct{}{}
	for _, parent := range messageMeta.Parents() {
		parentMessageIDs[parent.ToMapKey()] = struct{}{}
	}

	checkTip := func(tipsMap map[string]*Tip, parentTip *Tip, retentionRulesTipsLimit int, maxChildren uint32, maxReferencedTipAge time.Duration) bool {
		// if the amount of known tips is above the limit, remove the tip directly
		if len(tipsMap) > retentionRulesTipsLimit {
			return ts.removeTipWithoutLocking(tipsMap, parentTip.MessageID)
		}

		// check if the maximum amount of children for this tip is reached
		if parentTip.ChildrenCount.Add(1) >= maxChildren {
			return ts.removeTipWithoutLocking(tipsMap, parentTip.MessageID)
		}

		if maxReferencedTipAge == time.Duration(0) {
			// check for maxReferenceTipAge is disabled
			return false
		}

		// check if the tip was referenced by another message before
		if parentTip.TimeFirstChild.IsZero() {
			// mark the tip as referenced
			parentTip.TimeFirstChild = time.Now()
		}

		return false
	}

	for parentMessageID := range parentMessageIDs {
		// we have to separate between the pools, to prevent semi-lazy tips from emptying the non-lazy pool
		switch tip.Score {
		case ScoreNonLazy:
			if parentTip, exists := ts.nonLazyTipsMap[parentMessageID]; exists {
				if checkTip(ts.nonLazyTipsMap, parentTip, ts.retentionRulesTipsLimitNonLazy, ts.maxChildrenNonLazy, ts.maxReferencedTipAgeNonLazy) {
					ts.serverMetrics.TipsNonLazy.Sub(1)
				}
			}
		case ScoreSemiLazy:
			if parentTip, exists := ts.semiLazyTipsMap[parentMessageID]; exists {
				if checkTip(ts.semiLazyTipsMap, parentTip, ts.retentionRulesTipsLimitSemiLazy, ts.maxChildrenSemiLazy, ts.maxReferencedTipAgeSemiLazy) {
					ts.serverMetrics.TipsSemiLazy.Sub(1)
				}
			}
		}
	}
}

// removeTipWithoutLocking removes the given message from the tipsMap without acquiring the lock.
func (ts *TipSelector) removeTipWithoutLocking(tipsMap map[string]*Tip, messageID hornet.MessageID) bool {
	messageIDMapKey := messageID.ToMapKey()
	if tip, exists := tipsMap[messageIDMapKey]; exists {
		delete(tipsMap, messageIDMapKey)
		ts.Events.TipRemoved.Trigger(tip)
		return true
	}
	return false
}

// randomTipWithoutLocking picks a random tip from the pool and checks it's "own" score again without acquiring the lock.
func (ts *TipSelector) randomTipWithoutLocking(tipsMap map[string]*Tip) (hornet.MessageID, error) {

	if len(tipsMap) == 0 {
		// no semi-/non-lazy tips available
		return nil, ErrNoTipsAvailable
	}

	// get a random number between 0 and the amount of tips-1
	randTip := utils.RandomInsecure(0, len(tipsMap)-1)

	// iterate over the tipsMap and subtract each tip from randTip
	for _, tip := range tipsMap {
		// subtract the tip from randTip
		randTip--

		// if randTip is below zero, we return the given tip
		if randTip < 0 {
			return tip.MessageID, nil
		}
	}

	// no tips
	return nil, ErrNoTipsAvailable
}

// selectTipWithoutLocking selects a tip.
func (ts *TipSelector) selectTipWithoutLocking(tipsMap map[string]*Tip) (hornet.MessageID, error) {

	if !ts.syncManager.IsNodeAlmostSynced() {
		return nil, common.ErrNodeNotSynced
	}

	// record stats
	start := time.Now()

	tipMessageID, err := ts.randomTipWithoutLocking(tipsMap)
	ts.Events.TipSelPerformed.Trigger(&TipSelStats{Duration: time.Since(start)})

	return tipMessageID, err
}

// SelectTips selects multiple tips.
func (ts *TipSelector) selectTips(tipsMap map[string]*Tip) (hornet.MessageIDs, error) {
	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	tipCount := ts.optimalTipCount()
	maxRetries := (tipCount - 1) * 10

	seen := make(map[string]struct{})
	orderedSlicesWithoutDups := make(serializer.LexicalOrderedByteSlices, tipCount)

	// retry the tipselection several times if parents not unique
	uniqueElements := 0
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

		tipMapKey := tip.ToMapKey()
		if _, has := seen[tipMapKey]; has {
			// ignore duplicates
			continue
		}
		seen[tipMapKey] = struct{}{}
		orderedSlicesWithoutDups[uniqueElements] = tip
		uniqueElements++

		if uniqueElements >= tipCount {
			// collected enough tips
			break
		}
	}
	orderedSlicesWithoutDups = orderedSlicesWithoutDups[:uniqueElements]
	sort.Sort(orderedSlicesWithoutDups)

	result := make(hornet.MessageIDs, len(orderedSlicesWithoutDups))
	for i, v := range orderedSlicesWithoutDups {
		// this is necessary, otherwise we create a pointer to the loop variable
		tmp := v
		result[i] = tmp
	}

	return result, nil
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
func (ts *TipSelector) SelectSemiLazyTips() (hornet.MessageIDs, error) {
	return ts.selectTips(ts.semiLazyTipsMap)
}

// SelectNonLazyTips selects two non-lazy tips.
func (ts *TipSelector) SelectNonLazyTips() (hornet.MessageIDs, error) {
	return ts.selectTips(ts.nonLazyTipsMap)
}

func (ts *TipSelector) SelectSpammerTips() (isSemiLazy bool, tips hornet.MessageIDs, err error) {
	if ts.spammerTipsThresholdSemiLazy != 0 && len(ts.semiLazyTipsMap) > ts.spammerTipsThresholdSemiLazy {
		// threshold was defined and reached, return semi-lazy tips for the spammer
		tips, err = ts.SelectSemiLazyTips()
		if err != nil {
			return false, nil, fmt.Errorf("couldn't select semi-lazy tips: %w", err)
		}

		if len(tips) < 2 {
			// do not spam if the amount of tips are less than 2 since that would not reduce the semi lazy count
			return false, nil, fmt.Errorf("%w: semi lazy tips are equal", ErrNoTipsAvailable)
		}

		return true, tips, nil
	}

	if ts.spammerTipsThresholdNonLazy != 0 && len(ts.nonLazyTipsMap) < ts.spammerTipsThresholdNonLazy {
		// if a threshold was defined and not reached, do not return tips for the spammer
		return false, nil, fmt.Errorf("%w: non-lazy threshold not reached", ErrNoTipsAvailable)
	}

	tips, err = ts.SelectNonLazyTips()
	if err != nil {
		return false, tips, fmt.Errorf("couldn't select non-lazy tips: %w", err)
	}
	return false, tips, nil
}

// CleanUpReferencedTips checks if tips were referenced before
// and removes them if they reached their maximum age.
func (ts *TipSelector) CleanUpReferencedTips() int {

	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	checkTip := func(tipsMap map[string]*Tip, tip *Tip, maxReferencedTipAge time.Duration) bool {
		if tip.TimeFirstChild.IsZero() {
			// not referenced by another message
			return false
		}

		// check if the tip reached its maximum age
		if time.Since(tip.TimeFirstChild) < maxReferencedTipAge {
			return false
		}

		// remove the tip from the pool because it is outdated
		return ts.removeTipWithoutLocking(tipsMap, tip.MessageID)
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
		score, err := ts.calculateScore(tip.MessageID, cmi)
		if err != nil {
			// do not continue if calculation of the tip score failed
			return count, err
		}
		tip.Score = score

		if tip.Score == ScoreLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.nonLazyTipsMap, tip.MessageID) {
				count++
				ts.serverMetrics.TipsNonLazy.Sub(1)
			}
			continue
		}

		if tip.Score == ScoreSemiLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.nonLazyTipsMap, tip.MessageID) {
				count++
				ts.serverMetrics.TipsNonLazy.Sub(1)
			}
			// add the tip to the semi-lazy tips map
			ts.semiLazyTipsMap[tip.MessageID.ToMapKey()] = tip
			ts.Events.TipAdded.Trigger(tip)
			ts.serverMetrics.TipsSemiLazy.Add(1)
			count--
		}
	}

	for _, tip := range ts.semiLazyTipsMap {
		// check the score of the tip again to avoid old tips
		score, err := ts.calculateScore(tip.MessageID, cmi)
		if err != nil {
			// do not continue if calculation of the tip score failed
			return count, err
		}
		tip.Score = score

		if tip.Score == ScoreLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.semiLazyTipsMap, tip.MessageID) {
				count++
				ts.serverMetrics.TipsSemiLazy.Sub(1)
			}
			continue
		}

		if tip.Score == ScoreNonLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.semiLazyTipsMap, tip.MessageID) {
				count++
				ts.serverMetrics.TipsSemiLazy.Sub(1)
			}
			// add the tip to the non-lazy tips map
			ts.nonLazyTipsMap[tip.MessageID.ToMapKey()] = tip
			ts.Events.TipAdded.Trigger(tip)
			ts.serverMetrics.TipsNonLazy.Add(1)
			count--
		}
	}

	return count, nil
}

// calculateScore calculates the tip selection score of this message
func (ts *TipSelector) calculateScore(messageID hornet.MessageID, cmi milestone.Index) (Score, error) {

	tipScore, err := ts.tipScoreCalculator.TipScore(ts.shutdownCtx, messageID, cmi)
	if err != nil {
		return ScoreLazy, err
	}

	switch tipScore {
	case tangle.TipScoreNotFound:
		// we need to return lazy instead of panic here, because the message could have been pruned already
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
