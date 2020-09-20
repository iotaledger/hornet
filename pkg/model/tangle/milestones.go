package tangle

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"fmt"
	"time"

	iotago "github.com/iotaledger/iota.go"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/syncutils"

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

	coordinatorPublicKey               ed25519.PublicKey
	coordinatorMilestoneMerkleHashFunc crypto.Hash

	ErrInvalidMilestone = errors.New("invalid milestone")
)

func MilestoneCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMsg *CachedMilestone))(params[0].(*CachedMilestone).Retain())
}

func ConfigureMilestones(cooPublicKey ed25519.PublicKey, cooMilestoneMerkleHashFunc crypto.Hash) {
	coordinatorPublicKey = cooPublicKey
	coordinatorMilestoneMerkleHashFunc = cooMilestoneMerkleHashFunc
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

// GetMilestoneOrNil returns the CachedMessage of a milestone index or nil if it doesn't exist.
// message +1
func GetMilestoneCachedMessageOrNil(milestoneIndex milestone.Index) *CachedMessage {

	cachedMs := GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
	if cachedMs == nil {
		return nil
	}
	defer cachedMs.Release(true) // milestone -1

	return GetCachedMessageOrNil(cachedMs.GetMilestone().MessageID)
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

// The node is synced if LMI != 0 and LSMI == LMI.
func updateNodeSynced(latestSolidIndex, latestIndex milestone.Index) {
	if latestIndex == 0 {
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

// message +1
func FindClosestNextMilestoneOrNil(index milestone.Index) *CachedMilestone {
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

		cachedMs := GetCachedMilestoneOrNil(index) // milestone +1
		if cachedMs != nil {
			return cachedMs
		}
	}
}

func CheckIfMilestone(msg *Message) (ms *iotago.MilestonePayload, err error) {

	switch ms := msg.GetMessage().Payload.(type) {
	case *iotago.MilestonePayload:
		if err := ms.VerifySignature(msg.GetMessage(), coordinatorPublicKey); err != nil {
			return ms, err
		}
	default:
		return nil, nil
	}

	return nil, nil
}
