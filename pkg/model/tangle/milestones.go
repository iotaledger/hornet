package tangle

import (
	"bytes"
	"context"
	"crypto"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/merkle"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

const (
	isNodeSyncedWithinThreshold = 2
)

var (
	solidMilestoneIndex   milestone.Index
	solidMilestoneLock    syncutils.RWMutex
	latestMilestoneIndex  milestone.Index
	latestMilestoneLock   syncutils.RWMutex
	isNodeSynced          bool
	isNodeSyncedThreshold bool

	waitForNodeSyncedChannelsLock syncutils.Mutex
	waitForNodeSyncedChannels     []chan struct{}

	coordinatorAddress                 hornet.Hash
	coordinatorSecurityLevel           int
	coordinatorMerkleTreeDepth         uint64
	coordinatorMilestoneMerkleHashFunc crypto.Hash
	maxMilestoneIndex                  milestone.Index

	ErrInvalidMilestone = errors.New("invalid milestone")
)

func ConfigureMilestones(cooAddr hornet.Hash, cooSecLvl int, cooMerkleTreeDepth uint64, cooMilestoneMerkleHashFunc crypto.Hash) {
	coordinatorAddress = cooAddr
	coordinatorSecurityLevel = cooSecLvl
	coordinatorMerkleTreeDepth = cooMerkleTreeDepth
	coordinatorMilestoneMerkleHashFunc = cooMilestoneMerkleHashFunc
	maxMilestoneIndex = 1 << coordinatorMerkleTreeDepth
}

func GetMilestoneMerkleHashFunc() crypto.Hash {
	return coordinatorMilestoneMerkleHashFunc
}

func ResetMilestoneIndexes() {
	solidMilestoneLock.Lock()
	latestMilestoneLock.Lock()
	defer solidMilestoneLock.Unlock()
	defer latestMilestoneLock.Unlock()

	solidMilestoneIndex = 0
	latestMilestoneIndex = 0
}

// GetMilestoneOrNil returns the CachedBundle of a milestone index or nil if it doesn't exist.
// bundle +1
func GetMilestoneOrNil(milestoneIndex milestone.Index) *CachedBundle {

	cachedMs := GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
	if cachedMs == nil {
		return nil
	}
	defer cachedMs.Release(true) // milestone -1

	return GetCachedBundleOrNil(cachedMs.GetMilestone().Hash)
}

// IsNodeSynced returns whether the node is synced.
func IsNodeSynced() bool {
	return isNodeSynced
}

// IsNodeSyncedWithThreshold returns whether the node is synced within a certain threshold.
func IsNodeSyncedWithThreshold() bool {
	return isNodeSyncedThreshold
}

// WaitForNodeSynced waits at most "timeout" duration for the node to become fully sync.
// if it is not at least synced within threshold, it will return false immediately.
// this is used to avoid small glitches of IsNodeSynced when the sync state is important,
// but a new milestone came in lately.
func WaitForNodeSynced(timeout time.Duration) bool {

	if !isNodeSyncedThreshold {
		// node is not even synced within threshold, and therefore it is unsync
		return false
	}

	if isNodeSynced {
		// node is synced, no need to wait
		return true
	}

	// create a channel that gets closed if the node got synced
	waitForNodeSyncedChannelsLock.Lock()
	waitForNodeSyncedChan := make(chan struct{})
	waitForNodeSyncedChannels = append(waitForNodeSyncedChannels, waitForNodeSyncedChan)
	waitForNodeSyncedChannelsLock.Unlock()

	// check again after the channel was created
	if isNodeSynced {
		// node is synced, no need to wait
		return true
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(timeout))
	defer cancel()

	// we wait either until the node got synced or we reached the deadline
	select {
	case <-waitForNodeSyncedChan:
	case <-ctx.Done():
	}

	return isNodeSynced
}

// The node is synced if LMI != 0, LMI >= "recentSeenMilestones" from snapshot and LSMI == LMI.
func updateNodeSynced(latestSolidIndex, latestIndex milestone.Index) {
	if latestIndex == 0 || latestIndex < GetLatestSeenMilestoneIndexFromSnapshot() {
		// the node can't be sync if not all "recentSeenMilestones" from the snapshot file have been solidified.
		isNodeSynced = false
		isNodeSyncedThreshold = false
		return
	}

	isNodeSynced = latestSolidIndex == latestIndex
	if isNodeSynced {
		// if the node is sync, signal all waiting routines at the end
		defer func() {
			waitForNodeSyncedChannelsLock.Lock()
			defer waitForNodeSyncedChannelsLock.Unlock()

			// signal all routines that are waiting
			for _, channel := range waitForNodeSyncedChannels {
				close(channel)
			}

			// create an empty slice for new signals
			waitForNodeSyncedChannels = make([]chan struct{}, 0)
		}()
	}

	// catch overflow
	if latestIndex < isNodeSyncedWithinThreshold {
		isNodeSyncedThreshold = true
		return
	}

	isNodeSyncedThreshold = latestSolidIndex >= (latestIndex - isNodeSyncedWithinThreshold)
}

// SetSolidMilestoneIndex sets the solid milestone index.
func SetSolidMilestoneIndex(index milestone.Index, updateSynced ...bool) {
	solidMilestoneLock.Lock()
	if solidMilestoneIndex > index {
		panic(fmt.Sprintf("current solid milestone (%d) is newer than (%d)", solidMilestoneIndex, index))
	}
	solidMilestoneIndex = index
	solidMilestoneLock.Unlock()

	if len(updateSynced) > 0 && !updateSynced[0] {
		// always call updateNodeSynced if parameter is not given.
		return
	}

	updateNodeSynced(index, GetLatestMilestoneIndex())
}

// OverwriteSolidMilestoneIndex is used to set older solid milestones (revalidation).
func OverwriteSolidMilestoneIndex(index milestone.Index) {
	solidMilestoneLock.Lock()
	solidMilestoneIndex = index
	solidMilestoneLock.Unlock()

	if isNodeSynced {
		updateNodeSynced(index, GetLatestMilestoneIndex())
	}
}

// GetSolidMilestoneIndex returns the latest solid milestone index.
func GetSolidMilestoneIndex() milestone.Index {
	solidMilestoneLock.RLock()
	defer solidMilestoneLock.RUnlock()

	return solidMilestoneIndex
}

// SetLatestMilestoneIndex sets the latest milestone index.
func SetLatestMilestoneIndex(index milestone.Index, updateSynced ...bool) bool {

	latestMilestoneLock.Lock()

	if latestMilestoneIndex >= index {
		// current LMI is bigger than new LMI => abort
		latestMilestoneLock.Unlock()
		return false
	}

	latestMilestoneIndex = index
	latestMilestoneLock.Unlock()

	if len(updateSynced) > 0 && !updateSynced[0] {
		// always call updateNodeSynced if parameter is not given
		return true
	}

	updateNodeSynced(GetSolidMilestoneIndex(), index)

	return true
}

// GetLatestMilestoneIndex returns the latest milestone index.
func GetLatestMilestoneIndex() milestone.Index {
	latestMilestoneLock.RLock()
	defer latestMilestoneLock.RUnlock()

	return latestMilestoneIndex
}

// bundle +1
func FindClosestNextMilestoneOrNil(index milestone.Index) *CachedBundle {
	lmi := GetLatestMilestoneIndex()
	if lmi == 0 {
		// no milestone received yet, check the next 100 milestones as a workaround
		lmi = GetSolidMilestoneIndex() + 100
	}

	if index == 4294967295 {
		// prevent overflow (2**32 -1)
		return nil
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
		// wrong amount of txs in bundle
		return false, nil
	}

	cachedTailTx := bndl.GetTail() // tx +1

	if !IsMaybeMilestone(cachedTailTx.Retain()) { // tx pass +1
		cachedTailTx.Release() // tx -1
		// transaction is not issued by compass => no milestone
		return false, nil
	}

	tailTxHash := cachedTailTx.GetTransaction().GetTxHash()

	// check the structure of the milestone
	milestoneIndex := bndl.GetMilestoneIndex()
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
		// it could be issued again since several transactions of the same bundle were processed in parallel
		return false, nil
	}

	cachedSignatureTxs := CachedTransactions{}
	cachedSignatureTxs = append(cachedSignatureTxs, cachedTailTx)

	for secLvl := 1; secLvl < coordinatorSecurityLevel; secLvl++ {
		cachedTx := GetCachedTransactionOrNil(cachedSignatureTxs[secLvl-1].GetTransaction().GetTrunkHash()) // tx +1
		if cachedTx == nil {
			cachedSignatureTxs.Release() // tx -1
			return false, errors.Wrapf(ErrInvalidMilestone, "Bundle too small for valid milestone, Hash: %v", tailTxHash.Trytes())
		}

		if !IsMaybeMilestone(cachedTx.Retain()) { // tx pass +1
			cachedTx.Release() // tx -1
			// transaction is not issued by compass => no milestone
			cachedSignatureTxs.Release() // tx -1
			return false, errors.Wrapf(ErrInvalidMilestone, "Transaction was not issued by compass, Hash: %v", tailTxHash.Trytes())
		}

		cachedSignatureTxs = append(cachedSignatureTxs, cachedTx)
		// tx will be released with cachedSignatureTxs
	}

	defer cachedSignatureTxs.Release() // tx -1

	cachedSiblingsTx := GetCachedTransactionOrNil(cachedSignatureTxs[coordinatorSecurityLevel-1].GetTransaction().GetTrunkHash()) // tx +1
	if cachedSiblingsTx == nil {
		return false, errors.Wrapf(ErrInvalidMilestone, "Bundle too small for valid milestone, Hash: %v", tailTxHash.Trytes())
	}
	defer cachedSiblingsTx.Release() // tx -1

	if !IsMaybeMilestoneTx(cachedSiblingsTx.Retain()) {
		// transaction is not issued by compass => no milestone
		return false, errors.Wrapf(ErrInvalidMilestone, "Transaction was not issued by compass, Hash: %v", tailTxHash.Trytes())
	}

	var fragments []trinary.Trytes
	for _, signatureTx := range cachedSignatureTxs {
		if signatureTx.GetTransaction().Tx.BranchTransaction != cachedSiblingsTx.GetTransaction().Tx.TrunkTransaction {
			return false, errors.Wrapf(ErrInvalidMilestone, "Structure is wrong, Hash: %v", tailTxHash.Trytes())
		}
		fragments = append(fragments, signatureTx.GetTransaction().Tx.SignatureMessageFragment)
	}

	var path []trinary.Trytes
	for i := 0; i < int(coordinatorMerkleTreeDepth); i++ {
		path = append(path, cachedSiblingsTx.GetTransaction().Tx.SignatureMessageFragment[i*consts.HashTrytesSize:(i+1)*consts.HashTrytesSize])
	}

	// verify milestone signature
	if valid, err := merkle.ValidateSignatureFragments(coordinatorAddress.Trytes(), uint32(milestoneIndex), path, fragments, cachedSiblingsTx.GetTransaction().Tx.Hash); !valid {
		if err != nil {
			return false, errors.Wrap(ErrInvalidMilestone, err.Error())
		}
		return false, errors.Wrapf(ErrInvalidMilestone, "Signature was not valid, Hash: %v", tailTxHash.Trytes())
	}

	bndl.setMilestone(true)

	return true, nil
}

// Checks if the the tx could be part of a milestone.
func IsMaybeMilestone(cachedTx *CachedTransaction) bool {
	value := (cachedTx.GetTransaction().Tx.Value == 0) && (bytes.Equal(cachedTx.GetTransaction().GetAddress(), coordinatorAddress))
	cachedTx.Release(true) // tx -1
	return value
}

// Checks if the the tx could be part of a milestone.
func IsMaybeMilestoneTx(cachedTx *CachedTransaction) bool {
	value := (cachedTx.GetTransaction().Tx.Value == 0) && (bytes.Equal(cachedTx.GetTransaction().GetAddress(), coordinatorAddress) || bytes.Equal(cachedTx.GetTransaction().GetAddress(), hornet.NullHashBytes))
	cachedTx.Release(true) // tx -1
	return value
}
