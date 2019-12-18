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
	"github.com/iotaledger/hive.go/typeutils"

	"github.com/gohornet/hornet/packages/model/hornet"
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

	emptyHash                trinary.Hash = "999999999999999999999999999999999999999999999999999999999999999999999999999999999"
	coordinatorAddress       string
	coordinatorSecurityLevel int
	numberOfKeysInAMilestone uint64
	maxMilestoneIndex        milestone_index.MilestoneIndex
	ErrInvalidMilestone      = errors.New("invalid milestone")
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

func SetLatestMilestone(milestone *Bundle) {
	latestMilestoneLock.Lock()

	index := milestone.GetMilestoneIndex()

	if latestMilestone != nil && latestMilestone.GetMilestoneIndex() >= index {
		latestMilestoneLock.Unlock()
		return
	}
	latestMilestone = milestone
	latestMilestoneLock.Unlock()

	updateNodeSynced(GetSolidMilestoneIndex(), index)
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

		ms, _ := GetMilestone(index)
		if ms != nil {
			return ms
		}
	}
}

func CheckIfMilestone(bundle *Bundle) (result bool, err error) {
	txIndex0 := bundle.GetTail()
	if txIndex0 == nil {
		return false, nil
	}

	if !IsMaybeMilestone(txIndex0) {
		// Transaction is not issued by compass => no milestone
		return false, nil
	}

	// Check the structure of the milestone
	milestoneIndex := getMilestoneIndex(txIndex0)
	if milestoneIndex <= GetSolidMilestoneIndex() {
		// Milestone older than out solid milestone
		return false, errors.Wrapf(ErrInvalidMilestone, "Index (%d) older than solid milestone (%d), Hash: %v", milestoneIndex, GetSolidMilestoneIndex(), txIndex0.GetHash())
	}

	if milestoneIndex >= maxMilestoneIndex {
		return false, errors.Wrapf(ErrInvalidMilestone, "Index (%d) out of range (0...%d), Hash: %v)", milestoneIndex, maxMilestoneIndex, txIndex0.GetHash())
	}

	signatureTxs := make([]*hornet.Transaction, 0, coordinatorSecurityLevel)
	signatureTxs = append(signatureTxs, txIndex0)

	for secLvl := 1; secLvl < coordinatorSecurityLevel; secLvl++ {
		tx, _ := GetTransaction(signatureTxs[secLvl-1].Tx.TrunkTransaction)
		if tx == nil {
			return false, errors.Wrapf(ErrInvalidMilestone, "Bundle too small for valid milestone, Hash: %v", txIndex0.GetHash())
		}

		if !IsMaybeMilestone(tx) {
			// Transaction is not issued by compass => no milestone
			return false, errors.Wrapf(ErrInvalidMilestone, "Transaction was not issued by compass, Hash: %v", txIndex0.GetHash())
		}

		signatureTxs = append(signatureTxs, tx)
	}

	siblingsTx, _ := GetTransaction(signatureTxs[coordinatorSecurityLevel-1].Tx.TrunkTransaction)
	if siblingsTx == nil {
		return false, errors.Wrapf(ErrInvalidMilestone, "Bundle too small for valid milestone, Hash: %v", txIndex0.GetHash())
	}

	if (siblingsTx.Tx.Value != 0) || (siblingsTx.Tx.Address != emptyHash) {
		// Transaction is not issued by compass => no milestone
		return false, errors.Wrapf(ErrInvalidMilestone, "Transaction was not issued by compass, Hash: %v", txIndex0.GetHash())
	}

	for _, signatureTx := range signatureTxs {
		if signatureTx.Tx.BranchTransaction != siblingsTx.Tx.TrunkTransaction {
			return false, errors.Wrapf(ErrInvalidMilestone, "Structure is wrong, Hash: %v", txIndex0.GetHash())
		}
	}

	// Check if milestone was already processed
	msBundle, _ := GetMilestone(milestoneIndex)
	if msBundle != nil {
		return false, errors.Wrapf(ErrInvalidMilestone, "Exists already, Index: %d", milestoneIndex)
	}

	// Verify milestone signature
	valid := validateMilestone(signatureTxs, siblingsTx, milestoneIndex, coordinatorSecurityLevel, numberOfKeysInAMilestone, coordinatorAddress)
	if !valid {
		return false, errors.Wrapf(ErrInvalidMilestone, "Signature was not valid, Hash: %v", txIndex0.GetHash())
	}

	bundle.SetMilestone(true)

	return true, nil
}

func GetMilestone(milestoneIndex milestone_index.MilestoneIndex) (result *Bundle, err error) {
	if cacheResult := MilestoneCache.ComputeIfAbsent(milestoneIndex, func() interface{} {
		if txHash, dbErr := readMilestoneTransactionHashFromDatabase(milestoneIndex); dbErr != nil {
			err = dbErr
			return nil
		} else if txHash != "" {
			tx, err := GetTransaction(txHash)
			if (tx == nil) || (err != nil) {
				return nil
			}
			bundleBucket, err := GetBundleBucket(tx.Tx.Bundle)
			if err != nil {
				return nil
			}

			return bundleBucket.GetBundleOfTailTransaction(txHash)
		} else {
			return nil
		}
	}); !typeutils.IsInterfaceNil(cacheResult) {
		result = cacheResult.(*Bundle)
	}

	return
}

func ContainsMilestone(milestoneIndex milestone_index.MilestoneIndex) (result bool, err error) {
	if MilestoneCache.Contains(milestoneIndex) {
		result = true
	} else {
		result, err = databaseContainsMilestone(milestoneIndex)
	}
	return
}

func StoreMilestoneInCache(milestone *Bundle) {
	if milestone.IsMilestone() {
		MilestoneCache.Set(milestone.GetMilestoneIndex(), milestone)
	}
}

func StoreMilestoneInDatabase(milestone *Bundle) error {
	if milestone.IsMilestone() {
		return storeMilestoneInDatabase(milestone)
	} else {
		return errors.New("Trying to store an invalid milestone")
	}
}

// Validates if the milestone has the correct signature
func validateMilestone(signatureTxs []*hornet.Transaction, siblingsTx *hornet.Transaction, milestoneIndex milestone_index.MilestoneIndex, securityLvl int, numberOfKeysInAMilestone uint64, coordinatorAddress trinary.Hash) (valid bool) {

	normalizedBundleHashFragments := make([]trinary.Trits, securityLvl)

	// milestones sign the normalized hash of the sibling transaction.
	normalizeBundleHash := signing.NormalizedBundleHash(siblingsTx.GetHash())

	for i := 0; i < int(securityLvl); i++ {
		normalizedBundleHashFragments[i] = normalizeBundleHash[i*consts.KeySegmentsPerFragment : (i+1)*consts.KeySegmentsPerFragment]
	}

	digests := make(trinary.Trits, len(signatureTxs)*consts.HashTrinarySize)
	for i := 0; i < len(signatureTxs); i++ {
		signatureMessageFragmentTrits, err := trinary.TrytesToTrits(signatureTxs[i].Tx.SignatureMessageFragment)
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

	siblingsTrits, err := transaction.TransactionToTrits(siblingsTx.Tx)
	if err != nil {
		return false
	}

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
func IsMaybeMilestone(transaction *hornet.Transaction) bool {
	return (transaction.Tx.Value == 0) && (transaction.Tx.Address == coordinatorAddress)
}

// Returns Milestone index of the milestone
func getMilestoneIndex(transaction *hornet.Transaction) (milestoneIndex milestone_index.MilestoneIndex) {
	return milestone_index.MilestoneIndex(trinary.TrytesToInt(transaction.Tx.ObsoleteTag))
}
