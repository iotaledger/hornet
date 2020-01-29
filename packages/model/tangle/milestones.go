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

	"github.com/gohornet/hornet/packages/model/milestone_index"
)

const (
	NodeSyncedThreshold = 2
)

var (
	solidMilestoneIndex milestone_index.MilestoneIndex
	solidMilestoneLock  syncutils.RWMutex
	latestMilestone     *Bundle
	latestMilestoneLock syncutils.RWMutex
	isNodeSynced        bool

	coordinatorAddress       string
	coordinatorSecurityLevel int
	numberOfKeysInAMilestone uint64
	maxMilestoneIndex        milestone_index.MilestoneIndex

	ErrInvalidMilestone = errors.New("invalid milestone")
)

func ConfigureMilestones(cooAddr string, cooSecLvl int, numOfKeysInMS uint64) {
	err := address.ValidAddress(cooAddr)
	if err != nil {
		panic(err)
	}
	coordinatorAddress = cooAddr
	coordinatorSecurityLevel = cooSecLvl
	numberOfKeysInAMilestone = numOfKeysInMS
	maxMilestoneIndex = 1 << numberOfKeysInAMilestone
}

func GetMilestone(milestoneIndex milestone_index.MilestoneIndex) *Bundle {

	cachedMilestone := GetCachedMilestone(milestoneIndex) // cachedMilestone +1
	defer cachedMilestone.Release()                       // cachedMilestone -1

	if !cachedMilestone.Exists() {
		return nil
	}

	ms := cachedMilestone.GetMilestone()

	tx := GetCachedTransaction(ms.Hash) // tx +1
	defer tx.Release()                  // tx -1

	if !tx.Exists() {
		return nil
	}

	bundleBucket, err := GetBundleBucket(tx.GetTransaction().Tx.Bundle)
	if err != nil {
		return nil
	}

	return bundleBucket.GetBundleOfTailTransaction(ms.Hash)
}

func IsNodeSynced() bool {
	return isNodeSynced
}

func updateNodeSynced(latestSolidIndex, latestIndex milestone_index.MilestoneIndex) {
	if latestIndex == 0 {
		isNodeSynced = false
		return
	}

	isNodeSynced = latestSolidIndex >= (latestIndex - NodeSyncedThreshold)
}

func SetSolidMilestone(bundle *Bundle) {
	if bundle.IsSolid() {
		solidMilestoneLock.Lock()
		if bundle.GetMilestoneIndex() < solidMilestoneIndex {
			panic(fmt.Sprintf("Current solid milestone (%d) is newer than (%d)", solidMilestoneIndex, bundle.GetMilestoneIndex()))
		} else {
			solidMilestoneIndex = bundle.GetMilestoneIndex()
		}
		solidMilestoneLock.Unlock()
		updateNodeSynced(bundle.GetMilestoneIndex(), GetLatestMilestoneIndex())
	}
}

func setSolidMilestoneIndex(index milestone_index.MilestoneIndex) {
	solidMilestoneLock.Lock()
	solidMilestoneIndex = index
	solidMilestoneLock.Unlock()
	updateNodeSynced(index, GetLatestMilestoneIndex())
}

func GetSolidMilestoneIndex() milestone_index.MilestoneIndex {
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

func SetLatestMilestone(milestone *Bundle) error {
	latestMilestoneLock.Lock()

	index := milestone.GetMilestoneIndex()

	if latestMilestone != nil && latestMilestone.GetMilestoneIndex() >= index {
		latestMilestoneLock.Unlock()
		return nil
	}

	var err error
	if latestMilestone == nil {
		// Milestone was 0 before, so we have to fix all entries for all first seen tx until now
		err = FixFirstSeenTxHashOperations(index)
	}

	latestMilestone = milestone
	latestMilestoneLock.Unlock()

	updateNodeSynced(GetSolidMilestoneIndex(), index)

	return err
}

func GetLatestMilestone() *Bundle {
	latestMilestoneLock.RLock()
	defer latestMilestoneLock.RUnlock()

	return latestMilestone
}

func GetLatestMilestoneIndex() milestone_index.MilestoneIndex {
	latestMilestoneLock.RLock()
	defer latestMilestoneLock.RUnlock()

	if latestMilestone != nil {
		return latestMilestone.GetMilestoneIndex()
	}
	return 0
}

func FindClosestNextMilestone(index milestone_index.MilestoneIndex) (milestone *Bundle) {
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

		ms := GetMilestone(index)
		if ms != nil {
			return ms
		}
	}
}

func CheckIfMilestone(bundle *Bundle) (result bool, err error) {
	txIndex0 := bundle.GetTail() //+1
	if txIndex0 == nil {
		return false, nil
	}

	if !IsMaybeMilestone(txIndex0) {
		txIndex0.Release() //-1
		// Transaction is not issued by compass => no milestone
		return false, nil
	}

	txIndex0Hash := txIndex0.GetTransaction().GetHash()

	// Check the structure of the milestone
	milestoneIndex := getMilestoneIndex(txIndex0)
	if milestoneIndex <= GetSolidMilestoneIndex() {
		// Milestone older than solid milestone
		txIndex0.Release() //-1
		return false, errors.Wrapf(ErrInvalidMilestone, "Index (%d) older than solid milestone (%d), Hash: %v", milestoneIndex, GetSolidMilestoneIndex(), txIndex0Hash)
	}

	if milestoneIndex >= maxMilestoneIndex {
		txIndex0.Release() //-1
		return false, errors.Wrapf(ErrInvalidMilestone, "Index (%d) out of range (0...%d), Hash: %v)", milestoneIndex, maxMilestoneIndex, txIndex0Hash)
	}

	// Check if milestone was already processed
	msBundle := GetMilestone(milestoneIndex)
	if msBundle != nil {
		txIndex0.Release() //-1
		// It could be issued again since several transactions of the same bundle were processed in parallel
		return false, nil
	}

	var signatureTxs CachedTransactions
	defer signatureTxs.Release() //-1
	signatureTxs = append(signatureTxs, txIndex0)

	for secLvl := 1; secLvl < coordinatorSecurityLevel; secLvl++ {
		tx := GetCachedTransaction(signatureTxs[secLvl-1].GetTransaction().Tx.TrunkTransaction) //+1
		if !tx.Exists() {
			tx.Release() //-1
			return false, errors.Wrapf(ErrInvalidMilestone, "Bundle too small for valid milestone, Hash: %v", txIndex0Hash)
		}

		if !IsMaybeMilestone(tx) {
			tx.Release() //-1
			// Transaction is not issued by compass => no milestone
			return false, errors.Wrapf(ErrInvalidMilestone, "Transaction was not issued by compass, Hash: %v", txIndex0Hash)
		}

		signatureTxs = append(signatureTxs, tx)
		// tx will be released with signatureTxs
	}

	siblingsTx := GetCachedTransaction(signatureTxs[coordinatorSecurityLevel-1].GetTransaction().Tx.TrunkTransaction) //+1
	defer siblingsTx.Release()                                                                                        //-1

	if !siblingsTx.Exists() {
		return false, errors.Wrapf(ErrInvalidMilestone, "Bundle too small for valid milestone, Hash: %v", txIndex0Hash)
	}

	if (siblingsTx.GetTransaction().Tx.Value != 0) || (siblingsTx.GetTransaction().Tx.Address != consts.NullHashTrytes) {
		// Transaction is not issued by compass => no milestone
		return false, errors.Wrapf(ErrInvalidMilestone, "Transaction was not issued by compass, Hash: %v", txIndex0Hash)
	}

	for _, signatureTx := range signatureTxs {
		if signatureTx.GetTransaction().Tx.BranchTransaction != siblingsTx.GetTransaction().Tx.TrunkTransaction {
			return false, errors.Wrapf(ErrInvalidMilestone, "Structure is wrong, Hash: %v", txIndex0Hash)
		}
	}

	// Verify milestone signature
	valid := validateMilestone(signatureTxs, siblingsTx, milestoneIndex, coordinatorSecurityLevel, numberOfKeysInAMilestone, coordinatorAddress)
	if !valid {
		return false, errors.Wrapf(ErrInvalidMilestone, "Signature was not valid, Hash: %v", txIndex0Hash)
	}

	bundle.SetMilestone(true)

	return true, nil
}

// Validates if the milestone has the correct signature
func validateMilestone(signatureTxs CachedTransactions, siblingsTx *CachedTransaction, milestoneIndex milestone_index.MilestoneIndex, securityLvl int, numberOfKeysInAMilestone uint64, coordinatorAddress trinary.Hash) (valid bool) {

	signatureTxs.Retain() //+1
	siblingsTx.Retain()   //+1

	normalizedBundleHashFragments := make([]trinary.Trits, securityLvl)

	// milestones sign the normalized hash of the sibling transaction.
	normalizeBundleHash := signing.NormalizedBundleHash(siblingsTx.GetTransaction().GetHash())

	for i := 0; i < int(securityLvl); i++ {
		normalizedBundleHashFragments[i] = normalizeBundleHash[i*consts.KeySegmentsPerFragment : (i+1)*consts.KeySegmentsPerFragment]
	}

	digests := make(trinary.Trits, len(signatureTxs)*consts.HashTrinarySize)
	for i := 0; i < len(signatureTxs); i++ {
		signatureMessageFragmentTrits, err := trinary.TrytesToTrits(signatureTxs[i].GetTransaction().Tx.SignatureMessageFragment)
		if err != nil {
			return false
		}

		digest, err := signing.Digest(normalizedBundleHashFragments[i%consts.MaxSecurityLevel], signatureMessageFragmentTrits, kerl.NewKerl())
		if err != nil {
			return false
		}

		copy(digests[i*consts.HashTrinarySize:], digest)
	}

	signatureTxs.Release() //-1

	addressTrits, err := signing.Address(digests, kerl.NewKerl())
	if err != nil {
		siblingsTx.Release() //-1
		return false
	}

	siblingsTrits, err := transaction.TransactionToTrits(siblingsTx.GetTransaction().Tx)
	if err != nil {
		siblingsTx.Release() //-1
		return false
	}

	siblingsTx.Release() //-1

	// validate Merkle path
	merkleRoot, err := merkle.MerkleRoot(
		addressTrits,
		siblingsTrits,
		numberOfKeysInAMilestone,
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
func IsMaybeMilestone(transaction *CachedTransaction) bool {
	transaction.Retain()        //+1
	defer transaction.Release() //-1
	return (transaction.GetTransaction().Tx.Value == 0) && (transaction.GetTransaction().Tx.Address == coordinatorAddress)
}

// Returns Milestone index of the milestone
func getMilestoneIndex(transaction *CachedTransaction) (milestoneIndex milestone_index.MilestoneIndex) {
	transaction.Retain()        //+1
	defer transaction.Release() //-1
	return milestone_index.MilestoneIndex(trinary.TrytesToInt(transaction.GetTransaction().Tx.ObsoleteTag))
}
