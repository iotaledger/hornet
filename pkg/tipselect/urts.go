package tipselect

import (
	"errors"
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
	// ErrTipLazy is returned when the choosen tip was lazy already.
	ErrTipLazy = errors.New("tip already lazy")
)

// Tip defines a tip.
type Tip struct {
	// Score is the score of the tip.
	Score Score
	// MessageID is the message ID of the tip.
	MessageID *hornet.MessageID
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
	// maxDeltaMsgYoungestConeRootIndexToLSMI is the maximum allowed delta
	// value for the YCRI of a given message in relation to the current LSMI before it gets lazy.
	maxDeltaMsgYoungestConeRootIndexToLSMI milestone.Index
	// maxDeltaMsgOldestConeRootIndexToLSMI is the maximum allowed delta
	// value between OCRI of a given message in relation to the current LSMI before it gets semi-lazy.
	maxDeltaMsgOldestConeRootIndexToLSMI milestone.Index
	// belowMaxDepth is the maximum allowed delta
	// value between OCRI of a given message in relation to the current LSMI before it gets lazy.
	belowMaxDepth milestone.Index
	// retentionRulesTipsLimit is the maximum amount of current tips for which "maxReferencedTipAgeSeconds"
	// and "maxChildren" are checked. if the amount of tips exceeds this limit,
	// referenced tips get removed directly to reduce the amount of tips in the network. (non-lazy pool)
	retentionRulesTipsLimitNonLazy int
	// maxReferencedTipAgeSeconds is the maximum time a tip remains in the tip pool
	// after it was referenced by the first message.
	// this is used to widen the cone of the tangle. (non-lazy pool)
	maxReferencedTipAgeSecondsNonLazy time.Duration
	// maxChildren is the maximum amount of references by other messages
	// before the tip is removed from the tip pool.
	// this is used to widen the cone of the tangle. (non-lazy pool)
	maxChildrenNonLazy uint32
	// spammerTipsThresholdNonLazy is the maximum amount of tips in a tip-pool before the spammer tries to reduce these (0 = always)
	// this is used to support the network if someone attacks the tangle by spamming a lot of tips. (non-lazy pool)
	spammerTipsThresholdNonLazy int
	// retentionRulesTipsLimit is the maximum amount of current tips for which "maxReferencedTipAgeSeconds"
	// and "maxChildren" are checked. if the amount of tips exceeds this limit,
	// referenced tips get removed directly to reduce the amount of tips in the network. (semi-lazy pool)
	retentionRulesTipsLimitSemiLazy int
	// maxReferencedTipAgeSeconds is the maximum time a tip remains in the tip pool
	// after it was referenced by the first message.
	// this is used to widen the cone of the tangle. (semi-lazy pool)
	maxReferencedTipAgeSecondsSemiLazy time.Duration
	// maxChildren is the maximum amount of references by other messages
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
	Events Events
}

// New creates a new tip-selector.
func New(maxDeltaMsgYoungestConeRootIndexToLSMI int,
	maxDeltaMsgOldestConeRootIndexToLSMI int,
	belowMaxDepth int,
	retentionRulesTipsLimitNonLazy int,
	maxReferencedTipAgeSecondsNonLazy time.Duration,
	maxChildrenNonLazy uint32,
	spammerTipsThresholdNonLazy int,
	retentionRulesTipsLimitSemiLazy int,
	maxReferencedTipAgeSecondsSemiLazy time.Duration,
	maxChildrenSemiLazy uint32,
	spammerTipsThresholdSemiLazy int) *TipSelector {

	return &TipSelector{
		maxDeltaMsgYoungestConeRootIndexToLSMI: milestone.Index(maxDeltaMsgYoungestConeRootIndexToLSMI),
		maxDeltaMsgOldestConeRootIndexToLSMI:   milestone.Index(maxDeltaMsgOldestConeRootIndexToLSMI),
		belowMaxDepth:                          milestone.Index(belowMaxDepth),
		retentionRulesTipsLimitNonLazy:         retentionRulesTipsLimitNonLazy,
		maxReferencedTipAgeSecondsNonLazy:      maxReferencedTipAgeSecondsNonLazy,
		maxChildrenNonLazy:                     maxChildrenNonLazy,
		spammerTipsThresholdNonLazy:            spammerTipsThresholdNonLazy,
		retentionRulesTipsLimitSemiLazy:        retentionRulesTipsLimitSemiLazy,
		maxReferencedTipAgeSecondsSemiLazy:     maxReferencedTipAgeSecondsSemiLazy,
		maxChildrenSemiLazy:                    maxChildrenSemiLazy,
		spammerTipsThresholdSemiLazy:           spammerTipsThresholdSemiLazy,
		nonLazyTipsMap:                         make(map[string]*Tip),
		semiLazyTipsMap:                        make(map[string]*Tip),
		Events: Events{
			TipAdded:        events.NewEvent(TipCaller),
			TipRemoved:      events.NewEvent(TipCaller),
			TipSelPerformed: events.NewEvent(WalkerStatsCaller),
		},
	}
}

// AddTip adds the given message as a tip.
func (ts *TipSelector) AddTip(messageMeta *tangle.MessageMetadata) {
	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	messageID := messageMeta.GetMessageID()
	messageIDMapKey := messageID.MapKey()

	if _, exists := ts.nonLazyTipsMap[messageIDMapKey]; exists {
		// tip already exists
		return
	}

	if _, exists := ts.semiLazyTipsMap[messageIDMapKey]; exists {
		// tip already exists
		return
	}

	lsmi := tangle.GetSolidMilestoneIndex()

	score := ts.calculateScore(messageID, lsmi)
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
		metrics.SharedServerMetrics.TipsNonLazy.Add(1)
	case ScoreSemiLazy:
		ts.semiLazyTipsMap[messageIDMapKey] = tip
		metrics.SharedServerMetrics.TipsSemiLazy.Add(1)
	}

	ts.Events.TipAdded.Trigger(tip)

	// the parents are the messages this tip approves
	// remove them from the tip pool
	parentMessageIDs := map[string]struct{}{
		messageMeta.GetParent1MessageID().MapKey(): {},
		messageMeta.GetParent2MessageID().MapKey(): {},
	}

	checkTip := func(tipsMap map[string]*Tip, parentTip *Tip, retentionRulesTipsLimit int, maxChildren uint32, maxReferencedTipAgeSeconds time.Duration) bool {
		// if the amount of known tips is above the limit, remove the tip directly
		if len(tipsMap) > retentionRulesTipsLimit {
			return ts.removeTipWithoutLocking(tipsMap, parentTip.MessageID)
		}

		// check if the maximum amount of children for this tip is reached
		if parentTip.ChildrenCount.Add(1) >= maxChildren {
			return ts.removeTipWithoutLocking(tipsMap, parentTip.MessageID)
		}

		if maxReferencedTipAgeSeconds == time.Duration(0) {
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
				if checkTip(ts.nonLazyTipsMap, parentTip, ts.retentionRulesTipsLimitNonLazy, ts.maxChildrenNonLazy, ts.maxReferencedTipAgeSecondsNonLazy) {
					metrics.SharedServerMetrics.TipsNonLazy.Sub(1)
				}
			}
		case ScoreSemiLazy:
			if parentTip, exists := ts.semiLazyTipsMap[parentMessageID]; exists {
				if checkTip(ts.semiLazyTipsMap, parentTip, ts.retentionRulesTipsLimitSemiLazy, ts.maxChildrenSemiLazy, ts.maxReferencedTipAgeSecondsSemiLazy) {
					metrics.SharedServerMetrics.TipsSemiLazy.Sub(1)
				}
			}
		}
	}
}

// removeTipWithoutLocking removes the given message from the tipsMap without acquiring the lock.
func (ts *TipSelector) removeTipWithoutLocking(tipsMap map[string]*Tip, messageID *hornet.MessageID) bool {
	messageIDMapKey := messageID.MapKey()
	if tip, exists := tipsMap[messageIDMapKey]; exists {
		delete(tipsMap, messageIDMapKey)
		ts.Events.TipRemoved.Trigger(tip)
		return true
	}
	return false
}

// randomTipWithoutLocking picks a random tip from the pool and checks it's "own" score again without acquiring the lock.
func (ts *TipSelector) randomTipWithoutLocking(tipsMap map[string]*Tip) (*hornet.MessageID, error) {

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
func (ts *TipSelector) selectTipWithoutLocking(tipsMap map[string]*Tip) (*hornet.MessageID, error) {

	if !tangle.IsNodeSyncedWithThreshold() {
		return nil, tangle.ErrNodeNotSynced
	}

	// record stats
	start := time.Now()

	tipMessageID, err := ts.randomTipWithoutLocking(tipsMap)
	ts.Events.TipSelPerformed.Trigger(&TipSelStats{Duration: time.Since(start)})

	return tipMessageID, err
}

// SelectTips selects two tips.
func (ts *TipSelector) selectTips(tipsMap map[string]*Tip) (hornet.MessageIDs, error) {
	tips := hornet.MessageIDs{}

	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	parent1, err := ts.selectTipWithoutLocking(tipsMap)
	if err != nil {
		return nil, err
	}
	tips = append(tips, parent1)

	// retry the tipselection several times if parent1 and parent2 are equal
	for i := 0; i < 10; i++ {
		parent2, err := ts.selectTipWithoutLocking(tipsMap)
		if err != nil {
			if err == ErrNoTipsAvailable {
				// do not search other tips if there are none
				break
			}
			return nil, err
		}

		if *parent1 != *parent2 {
			tips = append(tips, parent2)
			return tips, nil
		}
	}

	// no second tip found, use the same again
	tips = append(tips, parent1)
	return tips, nil
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
			return false, nil, err
		}

		if *tips[0] != *tips[1] {
			// do not spam if the tip is equal since that would not reduce the semi lazy count
			return false, nil, ErrNoTipsAvailable
		}

		return true, tips, err
	}

	if ts.spammerTipsThresholdNonLazy != 0 && len(ts.nonLazyTipsMap) < ts.spammerTipsThresholdNonLazy {
		// if a threshold was defined and not reached, do not return tips for the spammer
		return false, nil, ErrNoTipsAvailable
	}

	tips, err = ts.SelectNonLazyTips()
	return false, tips, err
}

// CleanUpReferencedTips checks if tips were referenced before
// and removes them if they reached their maximum age.
func (ts *TipSelector) CleanUpReferencedTips() int {

	ts.tipsLock.Lock()
	defer ts.tipsLock.Unlock()

	checkTip := func(tipsMap map[string]*Tip, tip *Tip, maxReferencedTipAgeSeconds time.Duration) bool {
		if tip.TimeFirstChild.IsZero() {
			// not referenced by another message
			return false
		}

		// check if the tip reached its maximum age
		if time.Since(tip.TimeFirstChild) < maxReferencedTipAgeSeconds {
			return false
		}

		// remove the tip from the pool because it is outdated
		return ts.removeTipWithoutLocking(tipsMap, tip.MessageID)
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
		tip.Score = ts.calculateScore(tip.MessageID, lsmi)
		if tip.Score == ScoreLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.nonLazyTipsMap, tip.MessageID) {
				count++
				metrics.SharedServerMetrics.TipsNonLazy.Sub(1)
			}
			continue
		}

		if tip.Score == ScoreSemiLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.nonLazyTipsMap, tip.MessageID) {
				count++
				metrics.SharedServerMetrics.TipsNonLazy.Sub(1)
			}
			// add the tip to the semi-lazy tips map
			ts.semiLazyTipsMap[tip.MessageID.MapKey()] = tip
			ts.Events.TipAdded.Trigger(tip)
			metrics.SharedServerMetrics.TipsSemiLazy.Add(1)
			count--
		}
	}

	for _, tip := range ts.semiLazyTipsMap {
		// check the score of the tip again to avoid old tips
		tip.Score = ts.calculateScore(tip.MessageID, lsmi)
		if tip.Score == ScoreLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.semiLazyTipsMap, tip.MessageID) {
				count++
				metrics.SharedServerMetrics.TipsSemiLazy.Sub(1)
			}
			continue
		}

		if tip.Score == ScoreNonLazy {
			// remove the tip from the pool because it is outdated
			if ts.removeTipWithoutLocking(ts.semiLazyTipsMap, tip.MessageID) {
				count++
				metrics.SharedServerMetrics.TipsSemiLazy.Sub(1)
			}
			// add the tip to the non-lazy tips map
			ts.nonLazyTipsMap[tip.MessageID.MapKey()] = tip
			ts.Events.TipAdded.Trigger(tip)
			metrics.SharedServerMetrics.TipsNonLazy.Add(1)
			count--
		}
	}

	return count
}

// calculateScore calculates the tip selection score of this message
func (ts *TipSelector) calculateScore(messageID *hornet.MessageID, lsmi milestone.Index) Score {
	cachedMsgMeta := tangle.GetCachedMessageMetadataOrNil(messageID) // meta +1
	if cachedMsgMeta == nil {
		// we need to return lazy instead of panic here, because the message could have been pruned already
		// if the node was not sync for a longer time and after the pruning "UpdateScores" is called.
		return ScoreLazy
	}
	defer cachedMsgMeta.Release(true)

	ycri, ocri := dag.GetConeRootIndexes(cachedMsgMeta.Retain(), lsmi) // meta +1

	// if the LSMI to YCRI delta is over maxDeltaMsgYoungestConeRootIndexToLSMI, then the tip is lazy
	if (lsmi - ycri) > ts.maxDeltaMsgYoungestConeRootIndexToLSMI {
		return ScoreLazy
	}

	// if the OCRI to LSMI delta is over BelowMaxDepth/below-max-depth, then the tip is lazy
	if (lsmi - ocri) > ts.belowMaxDepth {
		return ScoreLazy
	}

	// if the OCRI to LSMI delta is over maxDeltaMsgOldestConeRootIndexToLSMI, the tip is semi-lazy
	if (lsmi - ocri) > ts.maxDeltaMsgOldestConeRootIndexToLSMI {
		return ScoreSemiLazy
	}

	return ScoreNonLazy
}
