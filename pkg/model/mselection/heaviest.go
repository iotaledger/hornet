package mselection

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/willf/bitset"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/utils"
)

const (
	belowMaxDepth milestone.Index = 15
)

var (
	// ErrNoTipsAvailable is returned when no tips are available in the node.
	ErrNoTipsAvailable = errors.New("no tips available")
)

// HeaviestSelector implements the heaviest branch selection strategy.
type HeaviestSelector struct {
	sync.Mutex

	minHeaviestBranchUnconfirmedTransactionsThreshold int
	maxHeaviestBranchTipsPerCheckpoint                int
	randomTipsPerCheckpoint                           int

	trackedTails map[string]*bundleTail // map of all tracked bundle transaction tails
	tips         *list.List             // list of available tips
	latestTip    *bundleTail
}

type bundleTail struct {
	hash hornet.Hash    // hash of the corresponding tail transaction
	tip  *list.Element  // pointer to the element in the tip list
	refs *bitset.BitSet // BitSet of all the referenced transactions
}

type bundleTailList struct {
	tails  map[string]*bundleTail
	latest *bundleTail
}

// Len returns the length of the inner tails slice.
func (il *bundleTailList) Len() int {
	return len(il.tails)
}

// randomTip selects a random tip item from the bundleTailList.
func (il *bundleTailList) randomTip() (*bundleTail, error) {
	if len(il.tails) == 0 {
		return nil, ErrNoTipsAvailable
	}

	randomTailIndex := utils.RandomInsecure(0, len(il.tails)-1)

	for _, tip := range il.tails {
		randomTailIndex--

		// if randomTailIndex reaches zero or below, we return the given tip
		if randomTailIndex <= 0 {
			return tip, nil
		}
	}

	return nil, ErrNoTipsAvailable
}

// referenceTip removes the tip and set all bits of all referenced
// transactions of the tip in all existing tips to zero.
// this way we can track which parts of the cone would already be referenced by this tip, and
// correctly calculate the weight of the remaining tips.
func (il *bundleTailList) referenceTip(tip *bundleTail) {

	il.removeTip(tip)

	// set all bits of all referenced transactions in all existing tips to zero
	for _, otherTip := range il.tails {
		otherTip.refs.InPlaceDifference(tip.refs)
	}
}

// removeTip removes the tip from the map.
func (il *bundleTailList) removeTip(tip *bundleTail) {
	delete(il.tails, string(tip.hash))
}

// New creates a new HeaviestSelector instance.
func New(minHeaviestBranchUnconfirmedTransactionsThreshold int, maxHeaviestBranchTipsPerCheckpoint int, randomTipsPerCheckpoint int) *HeaviestSelector {
	s := &HeaviestSelector{
		minHeaviestBranchUnconfirmedTransactionsThreshold: minHeaviestBranchUnconfirmedTransactionsThreshold,
		maxHeaviestBranchTipsPerCheckpoint:                maxHeaviestBranchTipsPerCheckpoint,
		randomTipsPerCheckpoint:                           randomTipsPerCheckpoint,
	}
	s.reset()
	return s
}

// reset resets the tracked transactions map and tips list of s.
func (s *HeaviestSelector) reset() {
	s.Lock()
	defer s.Unlock()

	// create an empty map
	s.trackedTails = make(map[trinary.Hash]*bundleTail)

	// create an empty list
	s.tips = list.New()
}

// selectTip selects a tip to be used for the next checkpoint.
// it returns a tip, confirming the most transactions in the future cone,
// and the amount of referenced transactions of this tip, that were not referenced by previously chosen tips.
func (s *HeaviestSelector) selectTip(tipsList *bundleTailList) (*bundleTail, uint, error) {

	if tipsList.Len() == 0 {
		return nil, 0, ErrNoTipsAvailable
	}

	var best = struct {
		tips  []*bundleTail
		count uint
	}{
		tips: []*bundleTail{
			tipsList.latest,
		},
		count: tipsList.latest.refs.Count(),
	}

	// loop through all tips and find the one with the most referenced transactions
	for _, tip := range tipsList.tails {
		c := tip.refs.Count()
		if c > best.count {
			// tip with heavier branch found
			best.tips = []*bundleTail{
				tip,
			}
			best.count = c
		} else if c == best.count {
			// add the tip to the slice of currently best tips
			best.tips = append(best.tips, tip)
		}
	}

	// select a random tip from the provided slice of tips.
	selected := best.tips[utils.RandomInsecure(0, len(best.tips)-1)]

	return selected, best.count, nil
}

// SelectTips tries to collect tips that confirm the most transactions in the future cone.
// best tips are determined by counting the referenced transactions (heaviest branches) and by "removing" the
// transactions of the referenced cone of the already choosen tips in the bitsets of the available tips.
// only tips are considered that were present at the beginning of the SelectTips call,
// to prevent attackers from creating heavier branches while we are searching the best tips.
// "maxHeaviestBranchTipsPerCheckpoint" is the amount of tips that are collected if
// the current best tip is not below "UnconfirmedTransactionsThreshold" before.
// a minimum amount of selected tips can be enforced, even if none of the heaviest branches matches the
// "minHeaviestBranchUnconfirmedTransactionsThreshold" criteria.
// if at least one heaviest branch tip was found, "randomTipsPerCheckpoint" random tips are added
// to add some additional randomness to prevent parasite chain attacks.
// the selection is cancelled after a fixed deadline. in this case, it returns the current collected tips.
func (s *HeaviestSelector) SelectTips(minRequiredTips int) (hornet.Hashes, error) {

	// copy the tips to release the lock to allow faster iteration
	// and to get a frozen view of the tangle, so an attacker can't
	// create heavier branches while we are searching the best tips
	tipsList := s.copyTipItemsToList()

	// tips could be empty after a reset
	if tipsList.Len() == 0 {
		return nil, ErrNoTipsAvailable
	}

	var result hornet.Hashes

	// run the tip selection for at most 0.1s to keep the view on the tangle recent; this should be plenty
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(100*time.Millisecond))
	defer cancel()

	deadlineExceeded := false

	for i := 0; i < s.maxHeaviestBranchTipsPerCheckpoint; i++ {
		// when the context has been cancelled, stop collecting heaviest branch tips
		select {
		case <-ctx.Done():
			deadlineExceeded = true
		default:
		}

		tip, count, err := s.selectTip(tipsList)
		if err != nil {
			break
		}

		if (len(result) > minRequiredTips) && ((count < uint(s.minHeaviestBranchUnconfirmedTransactionsThreshold)) || deadlineExceeded) {
			// minimum amount of tips reached and the heaviest tips do not confirm enough transactions or the deadline was exceeded
			// => no need to collect more
			break
		}

		tipsList.referenceTip(tip)
		result = append(result, tip.hash)
	}

	if len(result) == 0 {
		return nil, ErrNoTipsAvailable
	}

	// also pick random tips if at least one heaviest branch tip was found
	for i := 0; i < s.randomTipsPerCheckpoint; i++ {
		item, err := tipsList.randomTip()
		if err != nil {
			break
		}

		tipsList.referenceTip(item)
		result = append(result, item.hash)
	}

	// reset the whole HeaviestSelector if valid tips were found
	s.reset()

	return result, nil
}

// OnNewSolidBundle adds a new bundle to be processed by s.
// The bundle must be solid and OnNewSolidBundle must be called in the order of solidification.
// We also have to check if the bundle is below max depth.
func (s *HeaviestSelector) OnNewSolidBundle(bndl *tangle.Bundle) (trackedTailsCount int) {
	s.Lock()
	defer s.Unlock()

	// filter duplicate transaction
	if _, contains := s.trackedTails[string(bndl.GetTailHash())]; contains {
		return
	}

	// we ignore bundles that are below max depth.
	if isBelowMaxDepth(bndl.GetTail()) {
		return s.GetTrackedTailsCount()
	}

	trunkItem := s.trackedTails[string(bndl.GetTrunk(true))]
	branchItem := s.trackedTails[string(bndl.GetBranch(true))]

	// compute the referenced transactions
	// all the known approvers in the HeaviestSelector are represented by a unique bit in a bitset.
	// if a new approver is added, we expand the bitset by 1 bit and store the Union of the bitsets
	// of trunk and branch for this approver, to know which parts of the cone are referenced by this approver.
	idx := uint(len(s.trackedTails))
	it := &bundleTail{hash: bndl.GetTailHash(), refs: bitset.New(idx + 1).Set(idx)}
	if trunkItem != nil {
		it.refs.InPlaceUnion(trunkItem.refs)
	}
	if branchItem != nil {
		it.refs.InPlaceUnion(branchItem.refs)
	}
	s.trackedTails[string(it.hash)] = it

	// update tips
	s.removeTip(trunkItem)
	s.removeTip(branchItem)
	s.latestTip = it
	it.tip = s.tips.PushBack(it)

	return s.GetTrackedTailsCount()
}

// removeTip removes the tip item from s.
func (s *HeaviestSelector) removeTip(it *bundleTail) {
	if it == nil || it.tip == nil {
		return
	}
	s.tips.Remove(it.tip)
	it.tip = nil
}

// copyTipItemsToList returns a copy of the items corresponding to tips.
func (s *HeaviestSelector) copyTipItemsToList() *bundleTailList {
	s.Lock()
	defer s.Unlock()

	result := make(map[string]*bundleTail)
	for e := s.tips.Front(); e != nil; e = e.Next() {
		tip := e.Value.(*bundleTail)
		result[string(tip.hash)] = tip
	}
	return &bundleTailList{tails: result, latest: s.latestTip}
}

// GetTrackedTailsCount returns the amount of known bundle tails.
func (s *HeaviestSelector) GetTrackedTailsCount() (trackedTails int) {
	return len(s.trackedTails)
}

// isBelowMaxDepth checks the below max depth criteria for the given tail transaction.
func isBelowMaxDepth(cachedTailTx *tangle.CachedTransaction) bool {
	defer cachedTailTx.Release(true)

	lsmi := tangle.GetSolidMilestoneIndex()

	_, ortsi := dag.GetTransactionRootSnapshotIndexes(cachedTailTx.Retain(), lsmi) // tx +1

	// if the ORTSI to LSMI delta of the tail transaction is equal or greater belowMaxDepth, the tip is invalid.
	// "equal" is important because the next milestone would reference this transaction.
	if lsmi-ortsi >= belowMaxDepth {
		return true
	}

	return false
}
