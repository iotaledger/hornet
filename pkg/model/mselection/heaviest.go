package mselection

import (
	"bytes"
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

// Errors during milestone selection
var (
	ErrWrongReference = errors.New("reference does not match root")

	// ErrNoTipsAvailable is returned when no tips are available in the node.
	ErrNoTipsAvailable = errors.New("no tips available")
)

const (
	belowMaxDepth milestone.Index = 15
)

// HeaviestSelector implements the heaviest branch selection strategy.
type HeaviestSelector struct {
	sync.Mutex

	approvers map[trinary.Hash]*item
	tips      *list.List
}

type item struct {
	hash  hornet.Hash    // hash of the corresponding transaction
	tip   *list.Element  // pointer to the element in the tip list
	index uint           // index of the transaction in the approvers list
	refs  *bitset.BitSet // BitSet of all the referenced transactions
}

// New creates a new HeaviestSelector instance.
func New() *HeaviestSelector {
	s := &HeaviestSelector{}
	s.Reset()
	return s
}

// Reset resets the approvers and tips list of s.
func (s *HeaviestSelector) Reset() {
	s.Lock()
	defer s.Unlock()

	// create an empty map
	s.approvers = make(map[trinary.Hash]*item)

	// create an empty list
	s.tips = list.New()
}

// ResetCone set all bits of all referenced transactions of the tip in all existing tips to zero.
func (s *HeaviestSelector) ResetCone(tipHash hornet.Hash) error {
	s.Lock()
	defer s.Unlock()

	choosenTip, exists := s.approvers[string(tipHash)]
	if !exists {
		return ErrWrongReference
	}

	// remove the used tip from the tips list
	s.removeTip(choosenTip)

	// set all bits of all referenced transactions in all existing tips to zero
	for e := s.tips.Front(); e != nil; e = e.Next() {
		e.Value.(*item).refs.InPlaceDifference(choosenTip.refs)
	}

	return nil
}

// selectTip selects a tip to be used for the next milestone.
// It returns a tip, confirming the most transactions in the future cone.
// The selection can be cancelled anytime via the provided context. In this case, it returns the current best solution.
// selectTip be called concurrently with other HeaviestSelector methods. However, it only considers the tips
// that were present at the beginning of the selectTip call.
// TODO: add a proper interface for ms selection that is used by the coordinator
func (s *HeaviestSelector) selectTip(ctx context.Context) (hornet.Hash, error) {
	// copy the tips to release the lock at allow faster iteration
	tips := s.tipItems()

	// tips could be empty after a reset
	if len(tips) == 0 {
		return nil, ErrNoTipsAvailable
	}

	lastTip := tips[len(tips)-1]

	var best = struct {
		tips  hornet.Hashes
		count uint
	}{
		tips:  hornet.Hashes{lastTip.hash},
		count: lastTip.refs.Count(),
	}

	// loop through all tips and find the one with the most referenced transactions
	for _, tip := range tips {
		// when the context has been cancelled, return the current best with an error
		select {
		case <-ctx.Done():
			return randomTip(best.tips), ctx.Err()
		default:
		}

		c := tip.refs.Count()
		if c > best.count {
			best.tips = hornet.Hashes{tip.hash}
			best.count = c
		} else if c == best.count {
			best.tips = append(best.tips, tip.hash)
		}
	}

	// TODO: is it really to select a random tip? Maybe prefer the older (or younger) tip instead
	return randomTip(best.tips), nil
}

// SelectTip selects a tip to be used for the next milestone.
func (s *HeaviestSelector) SelectTip() (hornet.Hash, error) {

	if !tangle.IsNodeSynced() {
		return nil, tangle.ErrNodeNotSynced
	}

	// run the tip selection for at most 0.1s to keep the view on the tangle recent; this should be plenty
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(100*time.Millisecond))
	defer cancel()
	return s.selectTip(ctx)
}

// OnNewSolidBundle adds a new bundle to be processed by s.
// The bundle must be solid and OnNewSolidBundle must be called in the order of solidification.
// We also have to check if the bundle is below max depth.
func (s *HeaviestSelector) OnNewSolidBundle(bndl *tangle.Bundle) {
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

				_, approveeORTSI = s.getTransactionRootSnapshotIndexes(cachedApproveeTx.Retain(), lsmi) // tx +1
				cachedApproveeTx.Release(true)
			}

			// if the confirmationIdx to LSMI delta of the approvee is equal or greater belowMaxDepth, the tip is invalid.
			// "equal" is important because the next milestone would reference this transaction.
			if lsmi-approveeORTSI >= belowMaxDepth {
				return
			}
		}
	}

	// TODO: when len(s.approvers) gets too large trigger a checkpoint to prevent drastic performance hits

	// compute the referenced transactions
	idx := uint(len(s.approvers))
	it := &item{hash: bndl.GetTailHash(), index: idx, refs: bitset.New(idx + 1).Set(idx)}
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

func (s *HeaviestSelector) updateOutdatedRootSnapshotIndexes(outdatedTransactions hornet.Hashes, lsmi milestone.Index) {
	for i := len(outdatedTransactions) - 1; i >= 0; i-- {
		outdatedTxHash := outdatedTransactions[i]

		cachedTx := tangle.GetCachedTransactionOrNil(outdatedTxHash)
		if cachedTx == nil {
			panic(tangle.ErrTransactionNotFound)
		}
		s.getTransactionRootSnapshotIndexes(cachedTx, lsmi)
	}
}

// getTransactionRootSnapshotIndexes searches the root snapshot indexes for a given transaction.
func (s *HeaviestSelector) getTransactionRootSnapshotIndexes(cachedTx *tangle.CachedTransaction, lsmi milestone.Index) (youngestTxRootSnapshotIndex milestone.Index, oldestTxRootSnapshotIndex milestone.Index) {
	defer cachedTx.Release(true) // tx -1

	// if the tx already contains recent (calculation index matches LSMI)
	// information about yrtsi and ortsi, return that info
	yrtsi, ortsi, rtsci := cachedTx.GetMetadata().GetRootSnapshotIndexes()
	if rtsci == lsmi {
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

	startTxHash := cachedTx.GetMetadata().GetTxHash()

	// traverse the approvees of this transaction to calculate the root snapshot indexes for this transaction.
	// this walk will also collect all outdated transactions in the same cone, to update them afterwards.
	if err := dag.TraverseApprovees(cachedTx.GetMetadata().GetTxHash(),
		// traversal stops if no more transactions pass the given condition
		func(cachedTx *tangle.CachedTransaction) (bool, error) { // tx +1
			defer cachedTx.Release(true) // tx -1

			// first check if the tx was confirmed => update yrtsi and ortsi with the confirmation index
			if confirmed, at := cachedTx.GetMetadata().GetConfirmed(); confirmed {
				updateIndexes(at, at)
				return false, nil
			}

			if bytes.Equal(startTxHash, cachedTx.GetTransaction().GetTxHash()) {
				// skip the start transaction, so it doesn't get added to the outdatedTransactions
				return true, nil
			}

			// if the tx was not confirmed yet, but already contains recent (calculation index matches LSMI) information
			// about yrtsi and ortsi, propagate that info
			yrtsi, ortsi, rtsci := cachedTx.GetMetadata().GetRootSnapshotIndexes()
			if rtsci == lsmi {
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
	s.updateOutdatedRootSnapshotIndexes(outdatedTransactions, lsmi)

	// set the new transaction root snapshot indexes in the metadata of the transaction
	cachedTx.GetMetadata().SetRootSnapshotIndexes(youngestTxRootSnapshotIndex, oldestTxRootSnapshotIndex, lsmi)

	return youngestTxRootSnapshotIndex, oldestTxRootSnapshotIndex
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

// randomTip selects a random tip from the provided slice of tips.
func randomTip(tips hornet.Hashes) hornet.Hash {
	if len(tips) == 0 {
		panic("empty tips")
	}
	return tips[utils.RandomInsecure(0, len(tips)-1)]
}
