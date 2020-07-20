package mselection

import (
	"bytes"
	"container/list"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/willf/bitset"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/utils"
)

// Errors during milestone selection
var (
	ErrWrongReference = errors.New("reference does not match root")
)

const capIncreaseFactor = 1.15

// HeaviestSelector implements the heaviest pair selection (HPS) strategy.
type HeaviestSelector struct {
	sync.Mutex

	rootItem  *item
	approvers map[trinary.Hash]*item
	tips      *list.List
}

type item struct {
	hash hornet.Hash    // hash of the corresponding transaction
	tip  *list.Element  // pointer to the element in the tip list
	refs *bitset.BitSet // BitSet of all the referenced transactions
}

// HPS creates a new HeaviestSelector instance.
func HPS(root hornet.Hash) *HeaviestSelector {
	s := &HeaviestSelector{}
	s.SetRoot(root)
	return s
}

// SetRoot sets the root transaction of s; only transactions referencing the root are considered for the tip selection.
func (s *HeaviestSelector) SetRoot(root hornet.Hash) {
	s.Lock()
	defer s.Unlock()

	s.rootItem = &item{hash: root, refs: bitset.New(0)} // the root doesn't reference anything
	// create an empty map only containing the root item
	s.approvers = make(map[trinary.Hash]*item, int(float64(len(s.approvers))*capIncreaseFactor))
	s.approvers[string(root)] = s.rootItem
	// create an empty list only containing the root item
	s.tips = list.New()
	s.rootItem.tip = s.tips.PushBack(s.rootItem)
}

// SelectTips selects two tips to be used for the next milestone.
// It returns a pair of tips, confirming the most transactions in the future cone of the root.
// The selection can be cancelled anytime via the provided context. In this case, it returns the current best solution.
// selectTips be called concurrently with other HeaviestSelector methods. However, it only considers the tips
// that were present at the beginning of the selectTips call.
// TODO: add a proper interface for ms selection that is used by the coordinator
func (s *HeaviestSelector) SelectTips(ctx context.Context) ([]hornet.Hash, error) {
	// copy the tips to release the lock at allow faster iteration
	tips := s.tipItems()
	// tips will never be empty
	lastTip := tips[len(tips)-1]

	var best = struct {
		pairs [][2]hornet.Hash
		count uint
	}{
		pairs: [][2]hornet.Hash{{lastTip.hash, lastTip.hash}},
		count: lastTip.refs.Count(),
	}

	// loop through all tip pairs and find the one with the most referenced transactions
	for i := range tips {
		// when the context has been cancelled, return the current best with an error
		select {
		case <-ctx.Done():
			return randomPair(best.pairs), ctx.Err()
		default:
		}

		for j := range tips {
			if i >= j {
				continue // we do not care about the order in the pair
			}

			c := tips[i].refs.UnionCardinality(tips[j].refs)
			if c > best.count {
				best.pairs = [][2]hornet.Hash{{tips[i].hash, tips[j].hash}}
				best.count = c
			} else if c == best.count {
				best.pairs = append(best.pairs, [2]hornet.Hash{tips[i].hash, tips[j].hash})
			}
		}
	}
	// TODO: is it really to select a random pair? Maybe prefer the older (or younger) pair instead
	return randomPair(best.pairs), nil
}

// SelectTipsWithReference selects two tips to be used for the next milestone.
func (s *HeaviestSelector) SelectTipsWithReference(reference *hornet.Hash) (hornet.Hashes, error) {
	// ToDo: what about locking, so that root doesn't change?

	if reference != nil {
		if !bytes.Equal(s.rootItem.hash, *reference) {
			return nil, ErrWrongReference
		}
	}

	if !tangle.IsNodeSynced() {
		return nil, tangle.ErrNodeNotSynced
	}

	// run the tip selection for at most 0.1s to keep the view on the tangle recent; this should be plenty
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(100*time.Millisecond))
	defer cancel()
	tips, _ := s.SelectTips(ctx)
	return tips, nil
}

// OnNewSolidBundle adds a new transaction tx to be processed by s.
// The bundle must be solid and OnNewSolidBundle must be called in the order of solidification.
func (s *HeaviestSelector) OnNewSolidBundle(bndl *tangle.CachedBundle) {
	defer bndl.Release()

	s.Lock()
	defer s.Unlock()

	// filter duplicate transaction
	if _, contains := s.approvers[string(bndl.GetBundle().GetTailHash())]; contains {
		return
	}

	trunkItem := s.approvers[string(bndl.GetBundle().GetTrunk(true))]
	branchItem := s.approvers[string(bndl.GetBundle().GetBranch(true))]
	// if neither trunk nor branch reference the root, ignore this transaction
	if trunkItem == nil && branchItem == nil {
		return
	}

	// TODO: when len(s.approvers) gets too large trigger a checkpoint to prevent drastic performance hits

	// compute the referenced transactions
	idx := uint(len(s.approvers)) - 1
	it := &item{hash: bndl.GetBundle().GetTailHash(), refs: bitset.New(idx + 1).Set(idx)}
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

// randomPair selects a random pair from the provided slice of pairs.
func randomPair(pairs [][2]hornet.Hash) []hornet.Hash {
	if len(pairs) == 0 {
		panic("empty pairs")
	}
	return pairs[utils.RandomInsecure(0, len(pairs))][:]
}
