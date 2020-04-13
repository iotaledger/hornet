package tangle

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/kerl"
	"github.com/iotaledger/iota.go/merkle"
	"github.com/iotaledger/iota.go/signing"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

const (
	NodeSyncedThreshold = 2
)

var (
	solidMilestoneIndex   milestone.Index
	solidMilestoneLock    syncutils.RWMutex
	latestMilestone       *Bundle
	latestMilestoneLock   syncutils.RWMutex
	isNodeSynced          bool
	isNodeSyncedThreshold bool

	coordinatorAddress         string
	coordinatorSecurityLevel   int
	coordinatorMerkleTreeDepth uint64
	maxMilestoneIndex          milestone.Index

	ErrInvalidMilestone = errors.New("invalid milestone")
)

func ConfigureMilestones(cooAddr string, cooSecLvl int, cooMerkleTreeDepth uint64) {
	err := address.ValidAddress(cooAddr)
	if err != nil {
		panic(err)
	}
	coordinatorAddress = cooAddr
	coordinatorSecurityLevel = cooSecLvl
	coordinatorMerkleTreeDepth = cooMerkleTreeDepth
	maxMilestoneIndex = 1 << coordinatorMerkleTreeDepth
}

// bundle +1
func GetMilestoneOrNil(milestoneIndex milestone.Index) *CachedBundle {

	cachedMilestone := GetCachedMilestoneOrNil(milestoneIndex) // cachedMilestone +1
	if cachedMilestone == nil {
		return nil
	}
	defer cachedMilestone.Release(true) // cachedMilestone -1

	return GetCachedBundleOrNil(cachedMilestone.GetMilestone().Hash)
}

func IsNodeSynced() bool {
	return isNodeSynced
}

func IsNodeSyncedWithThreshold() bool {
	return isNodeSyncedThreshold
}

func updateNodeSynced(latestSolidIndex, latestIndex milestone.Index) {
	if latestIndex == 0 || latestIndex < GetLatestSeenMilestoneIndexFromSnapshot() {
		isNodeSynced = false
		isNodeSyncedThreshold = false
		return
	}

	isNodeSynced = latestSolidIndex == latestIndex
	isNodeSyncedThreshold = latestSolidIndex >= (latestIndex - NodeSyncedThreshold)
}

func SetSolidMilestone(cachedBndl *CachedBundle) {
	defer cachedBndl.Release() // bundle -1

	if !cachedBndl.GetBundle().IsSolid() {
		panic(fmt.Sprintf("SetSolidMilestone: Milestone was not solid: %d", cachedBndl.GetBundle().GetMilestoneIndex()))
	}

	solidMilestoneLock.Lock()
	if cachedBndl.GetBundle().GetMilestoneIndex() < solidMilestoneIndex {
		panic(fmt.Sprintf("Current solid milestone (%d) is newer than (%d)", solidMilestoneIndex, cachedBndl.GetBundle().GetMilestoneIndex()))
	}
	solidMilestoneIndex = cachedBndl.GetBundle().GetMilestoneIndex()
	solidMilestoneLock.Unlock()

	updateNodeSynced(cachedBndl.GetBundle().GetMilestoneIndex(), GetLatestMilestoneIndex())
}

// SetSolidMilestoneIndex sets the solid milestone index at node startup
// Do not use this function during normal node operation
func SetSolidMilestoneIndex(index milestone.Index) {
	solidMilestoneLock.Lock()
	solidMilestoneIndex = index
	solidMilestoneLock.Unlock()
	updateNodeSynced(index, GetLatestMilestoneIndex())
}

func GetSolidMilestoneIndex() milestone.Index {
	solidMilestoneLock.RLock()
	defer solidMilestoneLock.RUnlock()

	if solidMilestoneIndex != 0 {
		return solidMilestoneIndex
	}

	if snapshot != nil {
		return snapshot.SnapshotIndex
	}

	return 0
}

func SetLatestMilestone(cachedBndl *CachedBundle) error {
	defer cachedBndl.Release() // bundle -1

	latestMilestoneLock.Lock()

	index := cachedBndl.GetBundle().GetMilestoneIndex()

	if latestMilestone != nil && latestMilestone.GetMilestoneIndex() >= index {
		latestMilestoneLock.Unlock()
		return nil
	}

	var err error
	if latestMilestone == nil {
		// Milestone was 0 before, so we have to fix all entries for all first seen tx until now
		FixFirstSeenTxs(index)
	}

	latestMilestone = cachedBndl.GetBundle()
	latestMilestoneLock.Unlock()

	updateNodeSynced(GetSolidMilestoneIndex(), index)

	return err
}

func GetLatestMilestone() *Bundle {
	latestMilestoneLock.RLock()
	defer latestMilestoneLock.RUnlock()

	return latestMilestone
}

func GetLatestMilestoneIndex() milestone.Index {
	latestMilestoneLock.RLock()
	defer latestMilestoneLock.RUnlock()

	if latestMilestone != nil {
		return latestMilestone.GetMilestoneIndex()
	}
	return 0
}

// bundle +1
func FindClosestNextMilestoneOrNil(index milestone.Index) *CachedBundle {
	lmi := GetLatestMilestoneIndex()
	if lmi == 0 {
		// No milestone received yet, check the next 100 milestones as a workaround
		lmi = GetSolidMilestoneIndex() + 100
	}

	for {
		index++

		if index > lmi {
			return nil
		}

		cachedMs := GetMilestoneOrNil(index) // bundle +1
		if cachedMs != nil {
			return cachedMs
		}
	}
}

func CheckIfMilestone(bndl *Bundle) (result bool, err error) {

	if len(bndl.txs) != (coordinatorSecurityLevel + 1) {
		// Wrong amount of txs in bundle
		return false, nil
	}

	cachedTailTx := bndl.GetTail() // tx +1

	if !IsMaybeMilestone(cachedTailTx.Retain()) { // tx pass +1
		cachedTailTx.Release() // tx -1
		// Transaction is not issued by compass => no milestone
		return false, nil
	}

	tailTxHash := cachedTailTx.GetTransaction().GetHash()

	// Check the structure of the milestone
	milestoneIndex := getMilestoneIndex(cachedTailTx.Retain()) // tx pass +1
	if milestoneIndex <= GetSolidMilestoneIndex() {
		// Milestone older than solid milestone
		cachedTailTx.Release() // tx -1
		return false, nil
	}

	if milestoneIndex >= maxMilestoneIndex {
		cachedTailTx.Release() // tx -1
		return false, nil
	}

	// Check if milestone was already processed
	cachedMs := GetMilestoneOrNil(milestoneIndex) // bundle +1
	if cachedMs != nil {
		cachedTailTx.Release() // tx -1
		cachedMs.Release()     // bundle -1
		// It could be issued again since several transactions of the same bundle were processed in parallel
		return false, nil
	}

	cachedSignatureTxs := CachedTransactions{}
	cachedSignatureTxs = append(cachedSignatureTxs, cachedTailTx)

	for secLvl := 1; secLvl < coordinatorSecurityLevel; secLvl++ {
		cachedTx := GetCachedTransactionOrNil(cachedSignatureTxs[secLvl-1].GetTransaction().Tx.TrunkTransaction) // tx +1
		if cachedTx == nil {
			cachedSignatureTxs.Release() // tx -1
			return false, errors.Wrapf(ErrInvalidMilestone, "Bundle too small for valid milestone, Hash: %v", tailTxHash)
		}

		if !IsMaybeMilestone(cachedTx.Retain()) { // tx pass +1
			cachedTx.Release() // tx -1
			// Transaction is not issued by compass => no milestone
			cachedSignatureTxs.Release() // tx -1
			return false, errors.Wrapf(ErrInvalidMilestone, "Transaction was not issued by compass, Hash: %v", tailTxHash)
		}

		cachedSignatureTxs = append(cachedSignatureTxs, cachedTx)
		// tx will be released with cachedSignatureTxs
	}

	defer cachedSignatureTxs.Release() // tx -1

	cachedSiblingsTx := GetCachedTransactionOrNil(cachedSignatureTxs[coordinatorSecurityLevel-1].GetTransaction().Tx.TrunkTransaction) // tx +1
	if cachedSiblingsTx == nil {
		return false, errors.Wrapf(ErrInvalidMilestone, "Bundle too small for valid milestone, Hash: %v", tailTxHash)
	}
	defer cachedSiblingsTx.Release() // tx -1

	if (cachedSiblingsTx.GetTransaction().Tx.Value != 0) || (cachedSiblingsTx.GetTransaction().Tx.Address != consts.NullHashTrytes) {
		// Transaction is not issued by compass => no milestone
		return false, errors.Wrapf(ErrInvalidMilestone, "Transaction was not issued by compass, Hash: %v", tailTxHash)
	}

	for _, signatureTx := range cachedSignatureTxs {
		if signatureTx.GetTransaction().Tx.BranchTransaction != cachedSiblingsTx.GetTransaction().Tx.TrunkTransaction {
			return false, errors.Wrapf(ErrInvalidMilestone, "Structure is wrong, Hash: %v", tailTxHash)
		}
	}

	// Verify milestone signature
	valid := validateMilestone(cachedSignatureTxs.Retain(), cachedSiblingsTx.Retain(), milestoneIndex, coordinatorSecurityLevel, coordinatorMerkleTreeDepth, coordinatorAddress) // tx pass +2
	if !valid {
		return false, errors.Wrapf(ErrInvalidMilestone, "Signature was not valid, Hash: %v", tailTxHash)
	}

	bndl.setMilestone(true)

	return true, nil
}

// Validates if the milestone has the correct signature
func validateMilestone(cachedSignatureTxs CachedTransactions, cachedSiblingsTx *CachedTransaction, milestoneIndex milestone.Index, securityLvl int, coordinatorMerkleTreeDepth uint64, coordinatorAddress trinary.Hash) (valid bool) {

	defer cachedSignatureTxs.Release() // tx -1
	defer cachedSiblingsTx.Release()   // tx -1

	normalizedBundleHashFragments := make([]trinary.Trits, securityLvl)

	// milestones sign the normalized hash of the sibling transaction.
	normalizeBundleHash := signing.NormalizedBundleHash(cachedSiblingsTx.GetTransaction().GetHash())

	for i := 0; i < securityLvl; i++ {
		normalizedBundleHashFragments[i] = normalizeBundleHash[i*consts.KeySegmentsPerFragment : (i+1)*consts.KeySegmentsPerFragment]
	}

	digests := make(trinary.Trits, len(cachedSignatureTxs)*consts.HashTrinarySize)
	for i := 0; i < len(cachedSignatureTxs); i++ {
		signatureMessageFragmentTrits, err := trinary.TrytesToTrits(cachedSignatureTxs[i].GetTransaction().Tx.SignatureMessageFragment)
		if err != nil {
			return false
		}

		digest, err := signing.Digest(normalizedBundleHashFragments[i%consts.MaxSecurityLevel], signatureMessageFragmentTrits, kerl.NewKerl())
		if err != nil {
			return false
		}

		copy(digests[i*consts.HashTrinarySize:], digest)
	}

	addressTrits, err := signing.Address(digests, kerl.NewKerl())
	if err != nil {
		return false
	}

	siblingsTrits, err := transaction.TransactionToTrits(cachedSiblingsTx.GetTransaction().Tx)
	if err != nil {
		return false
	}

	// validate Merkle path
	merkleRoot, err := merkle.MerkleRoot(
		addressTrits,
		siblingsTrits,
		coordinatorMerkleTreeDepth,
		uint64(milestoneIndex),
		kerl.NewKerl(),
	)
	if err != nil {
		return false
	}

	merkleAddress, err := trinary.TritsToTrytes(merkleRoot)
	if err != nil {
		return false
	}

	return merkleAddress == coordinatorAddress
}

// Checks if the the tx could be part of a milestone
func IsMaybeMilestone(cachedTx *CachedTransaction) bool {
	value := (cachedTx.GetTransaction().Tx.Value == 0) && (cachedTx.GetTransaction().Tx.Address == coordinatorAddress)
	cachedTx.Release(true) // tx -1
	return value
}

// Checks if the the tx could be part of a milestone
func IsMaybeMilestoneTx(cachedTx *CachedTransaction) bool {
	tx := cachedTx.GetTransaction().Tx
	value := (tx.Value == 0) && ((tx.Address == coordinatorAddress) || (tx.Address == consts.NullHashTrytes))
	cachedTx.Release(true) // tx -1
	return value
}

// Returns Milestone index of the milestone
func getMilestoneIndex(cachedTx *CachedTransaction) (milestoneIndex milestone.Index) {
	value := milestone.Index(trinary.TrytesToInt(cachedTx.GetTransaction().Tx.ObsoleteTag))
	cachedTx.Release(true) // tx -1
	return value
}
