package mselection

import (
	"container/list"
	"context"
	"errors"
	"fmt"
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

	approvers map[trinary.Hash]*item
	tips      *list.List
}

type item struct {
	hash hornet.Hash    // hash of the corresponding transaction
	tip  *list.Element  // pointer to the element in the tip list
	refs *bitset.BitSet // BitSet of all the referenced transactions
}

type selectedTip struct {
	item  *item
	index int
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

// Reset resets the approvers and tips list of s.
func (s *HeaviestSelector) reset() {
	s.Lock()
	defer s.Unlock()

	// create an empty map
	s.approvers = make(map[trinary.Hash]*item)

	// create an empty list
	s.tips = list.New()
}

// selectTip selects a tip to be used for the next milestone.
// It returns a tip, confirming the most transactions in the future cone.
// The selection can be cancelled anytime via the provided context. In this case, it returns the current best solution.
// selectTip be called concurrently with other HeaviestSelector methods. However, it only considers the tips
// that were present at the beginning of the selectTip call.
func (s *HeaviestSelector) selectTip(tips []*item, ctx context.Context) (*selectedTip, error) {

	lastTip := tips[len(tips)-1]

	var best = struct {
		tips  []*selectedTip
		count uint
	}{
		tips: []*selectedTip{
			&selectedTip{
				item:  lastTip,
				index: len(tips) - 1,
			}},
		count: lastTip.refs.Count(),
	}

	// loop through all tips and find the one with the most referenced transactions
	for index, tip := range tips {
		// when the context has been cancelled, return the current best with an error
		select {
		case <-ctx.Done():
			return randomTip(best.tips), ctx.Err()
		default:
		}

		c := tip.refs.Count()
		if c > best.count {
			best.tips = []*selectedTip{{
				item:  tip,
				index: index,
			}}
			best.count = c
		} else if c == best.count {
			best.tips = append(best.tips, &selectedTip{
				item:  tip,
				index: index,
			})
		}
	}

	return randomTip(best.tips), nil
}

// selectTip selects a tip to be used for the next milestone.
// It returns a tip, confirming the most transactions in the future cone.
// The selection can be cancelled anytime via the provided context. In this case, it returns the current best solution.
// selectTip be called concurrently with other HeaviestSelector methods. However, it only considers the tips
// that were present at the beginning of the selectTip call.
func (s *HeaviestSelector) SelectTips(enforceTips bool) (hornet.Hashes, error) {

	// copy the tips to release the lock to allow faster iteration
	// and to get a frozen view of the tangle, so an attacker can't
	// create heavier branches while we are searching the best tips
	tips := s.tipItems()

	// tips could be empty after a reset
	if len(tips) == 0 {
		return nil, ErrNoTipsAvailable
	}

	var result hornet.Hashes

	for i := 0; i < s.maxHeaviestBranchTipsPerCheckpoint; i++ {
		// run the tip selection for at most 0.1s to keep the view on the tangle recent; this should be plenty
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(100*time.Millisecond))
		defer cancel()

		tip, err := s.selectTip(tips, ctx)
		if err != nil {
			break
		}

		if tip.item.refs.Count() < uint(s.minHeaviestBranchUnconfirmedTransactionsThreshold) {
			break
		}

		tips = applyTip(tips, tip)
		result = append(result, tip.item.hash)
	}

	if len(result) != 0 {
		// TODO
		for i := 0; i < s.randomTipsPerCheckpoint; i++ {

		}

		// reset the whole HeaviestSelector if valid tips were found
		s.reset()
	}

	return result, nil
}

// OnNewSolidBundle adds a new bundle to be processed by s.
// The bundle must be solid and OnNewSolidBundle must be called in the order of solidification.
// We also have to check if the bundle is below max depth.
func (s *HeaviestSelector) OnNewSolidBundle(bndl *tangle.Bundle) (tipCount int, approveeCount int) {
	s.Lock()
	defer s.Unlock()

	// filter duplicate transaction
	if _, contains := s.approvers[string(bndl.GetTailHash())]; contains {
		return
	}

	trunkHash := bndl.GetTrunk(true)
	branchHash := bndl.GetBranch(true)

	trunkItem := s.approvers[string(trunkHash)]
	branchItem := s.approvers[string(branchHash)]

	approveeHashes := make(map[string]struct{})
	if trunkItem == nil {
		approveeHashes[string(trunkHash)] = struct{}{}
	}

	if branchItem == nil {
		approveeHashes[string(branchHash)] = struct{}{}
	}

	// we have to check the below max depth criteria for approvees that do not reference our future cone.
	// if all the unknown approvees do not fail the below max depth criteria, the tip is valid
	if len(approveeHashes) > 0 {
		lsmi := tangle.GetSolidMilestoneIndex()

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

			// if the approveeORTSI to LSMI delta of the approvee is equal or greater belowMaxDepth, the tip is invalid.
			// "equal" is important because the next milestone would reference this transaction.
			if lsmi-approveeORTSI >= belowMaxDepth {
				return s.GetStats()
			}
		}
	}

	// compute the referenced transactions
	idx := uint(len(s.approvers))
	it := &item{hash: bndl.GetTailHash(), refs: bitset.New(idx + 1).Set(idx)}
	if trunkItem != nil {
		it.refs.InPlaceUnion(trunkItem.refs)
	}
	if branchItem != nil {
		it.refs.InPlaceUnion(branchItem.refs)
	}
	s.approvers[string(it.hash)] = it

	// update tips
	s.removeTip(trunkItem)
	s.removeTip(branchItem)
	it.tip = s.tips.PushBack(it)

	return s.GetStats()
}

func (s *HeaviestSelector) removeTip(it *item) {
	if it == nil || it.tip == nil {
		return
	}
	s.tips.Remove(it.tip)
	it.tip = nil
}

// tipItems returns a copy of the items corresponding to tips.
func (s *HeaviestSelector) tipItems() []*item {
	s.Lock()
	defer s.Unlock()

	result := make([]*item, 0, s.tips.Len())
	for e := s.tips.Front(); e != nil; e = e.Next() {
		result = append(result, e.Value.(*item))
	}
	return result
}

// GetStats returns the amount of known tips and approvees of s.
func (s *HeaviestSelector) GetStats() (tipCount int, approveeCount int) {
	return s.tips.Len(), len(s.approvers)
}

// randomTip selects a random tip from the provided slice of tips.
func randomTip(tips []*selectedTip) *selectedTip {
	if len(tips) == 0 {
		panic("empty tips")
	}
	return tips[utils.RandomInsecure(0, len(tips)-1)]
}

// applyTip set all bits of all referenced transactions of the tip in all existing tips to zero.
func applyTip(tips []*item, tip *selectedTip) []*item {

	tips = removeTip(tips, tip)

	// set all bits of all referenced transactions in all existing tips to zero
	for _, otherTip := range tips {
		otherTip.refs.InPlaceDifference(tip.item.refs)
	}

	return tips
}

func removeTip(tips []*item, tip *selectedTip) []*item {
	tips[tip.index] = tips[len(tips)-1]
	tips[len(tips)-1] = nil
	return tips[:len(tips)-1]
}
