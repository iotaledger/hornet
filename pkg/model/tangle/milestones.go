package tangle

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

const (
	isNodeSyncedWithinThreshold = 2
)

type CoordinatorPublicKey struct {
	StartIndex milestone.Index
	EndIndex   milestone.Index
	PublicKey  ed25519.PublicKey
}

var (
	ErrInvalidMilestone = errors.New("invalid milestone")
)

func MilestoneCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMs *CachedMilestone))(params[0].(*CachedMilestone).Retain())
}

func (t *Tangle) ConfigureMilestones(cooKeyManager *keymanager.KeyManager, cooMilestonePublicKeyCount int, cooMilestoneMerkleHashFunc crypto.Hash) {
	t.keyManager = cooKeyManager
	t.milestonePublicKeyCount = cooMilestonePublicKeyCount
	t.coordinatorMilestoneMerkleHashFunc = cooMilestoneMerkleHashFunc
}

func (t *Tangle) KeyManager() *keymanager.KeyManager {
	return t.keyManager
}

func (t *Tangle) GetMilestoneMerkleHashFunc() crypto.Hash {
	return t.coordinatorMilestoneMerkleHashFunc
}

func (t *Tangle) ResetMilestoneIndexes() {
	t.solidMilestoneLock.Lock()
	t.latestMilestoneLock.Lock()
	defer t.solidMilestoneLock.Unlock()
	defer t.latestMilestoneLock.Unlock()

	t.solidMilestoneIndex = 0
	t.latestMilestoneIndex = 0
}

// GetMilestoneOrNil returns the CachedMessage of a milestone index or nil if it doesn't exist.
// message +1
func (t *Tangle) GetMilestoneCachedMessageOrNil(milestoneIndex milestone.Index) *CachedMessage {

	cachedMs := t.GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
	if cachedMs == nil {
		return nil
	}
	defer cachedMs.Release(true) // milestone -1

	return t.GetCachedMessageOrNil(cachedMs.GetMilestone().MessageID)
}

// IsNodeSynced returns whether the node is synced.
func (t *Tangle) IsNodeSynced() bool {
	return t.isNodeSynced
}

// IsNodeSyncedWithThreshold returns whether the node is synced within a certain threshold.
func (t *Tangle) IsNodeSyncedWithThreshold() bool {
	return t.isNodeSyncedThreshold
}

// WaitForNodeSynced waits at most "timeout" duration for the node to become fully sync.
// if it is not at least synced within threshold, it will return false immediately.
// this is used to avoid small glitches of IsNodeSynced when the sync state is important,
// but a new milestone came in lately.
func (t *Tangle) WaitForNodeSynced(timeout time.Duration) bool {

	if !t.isNodeSyncedThreshold {
		// node is not even synced within threshold, and therefore it is unsync
		return false
	}

	if t.isNodeSynced {
		// node is synced, no need to wait
		return true
	}

	// create a channel that gets closed if the node got synced
	t.waitForNodeSyncedChannelsLock.Lock()
	waitForNodeSyncedChan := make(chan struct{})
	t.waitForNodeSyncedChannels = append(t.waitForNodeSyncedChannels, waitForNodeSyncedChan)
	t.waitForNodeSyncedChannelsLock.Unlock()

	// check again after the channel was created
	if t.isNodeSynced {
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

	return t.isNodeSynced
}

// The node is synced if LMI != 0 and LSMI == LMI.
func (t *Tangle) updateNodeSynced(latestSolidIndex, latestIndex milestone.Index) {
	if latestIndex == 0 {
		t.isNodeSynced = false
		t.isNodeSyncedThreshold = false
		return
	}

	t.isNodeSynced = latestSolidIndex == latestIndex
	if t.isNodeSynced {
		// if the node is sync, signal all waiting routines at the end
		defer func() {
			t.waitForNodeSyncedChannelsLock.Lock()
			defer t.waitForNodeSyncedChannelsLock.Unlock()

			// signal all routines that are waiting
			for _, channel := range t.waitForNodeSyncedChannels {
				close(channel)
			}

			// create an empty slice for new signals
			t.waitForNodeSyncedChannels = make([]chan struct{}, 0)
		}()
	}

	// catch overflow
	if latestIndex < isNodeSyncedWithinThreshold {
		t.isNodeSyncedThreshold = true
		return
	}

	t.isNodeSyncedThreshold = latestSolidIndex >= (latestIndex - isNodeSyncedWithinThreshold)
}

// SetSolidMilestoneIndex sets the solid milestone index.
func (t *Tangle) SetSolidMilestoneIndex(index milestone.Index, updateSynced ...bool) {
	t.solidMilestoneLock.Lock()
	if t.solidMilestoneIndex > index {
		panic(fmt.Sprintf("current solid milestone (%d) is newer than (%d)", t.solidMilestoneIndex, index))
	}
	t.solidMilestoneIndex = index
	t.solidMilestoneLock.Unlock()

	if len(updateSynced) > 0 && !updateSynced[0] {
		// always call updateNodeSynced if parameter is not given.
		return
	}

	t.updateNodeSynced(index, t.GetLatestMilestoneIndex())
}

// OverwriteSolidMilestoneIndex is used to set older solid milestones (revalidation).
func (t *Tangle) OverwriteSolidMilestoneIndex(index milestone.Index) {
	t.solidMilestoneLock.Lock()
	t.solidMilestoneIndex = index
	t.solidMilestoneLock.Unlock()

	if t.isNodeSynced {
		t.updateNodeSynced(index, t.GetLatestMilestoneIndex())
	}
}

// GetSolidMilestoneIndex returns the latest solid milestone index.
func (t *Tangle) GetSolidMilestoneIndex() milestone.Index {
	t.solidMilestoneLock.RLock()
	defer t.solidMilestoneLock.RUnlock()

	return t.solidMilestoneIndex
}

// SetLatestMilestoneIndex sets the latest milestone index.
func (t *Tangle) SetLatestMilestoneIndex(index milestone.Index, updateSynced ...bool) bool {

	t.latestMilestoneLock.Lock()

	if t.latestMilestoneIndex >= index {
		// current LMI is bigger than new LMI => abort
		t.latestMilestoneLock.Unlock()
		return false
	}

	t.latestMilestoneIndex = index
	t.latestMilestoneLock.Unlock()

	if len(updateSynced) > 0 && !updateSynced[0] {
		// always call updateNodeSynced if parameter is not given
		return true
	}

	t.updateNodeSynced(t.GetSolidMilestoneIndex(), index)

	return true
}

// GetLatestMilestoneIndex returns the latest milestone index.
func (t *Tangle) GetLatestMilestoneIndex() milestone.Index {
	t.latestMilestoneLock.RLock()
	defer t.latestMilestoneLock.RUnlock()

	return t.latestMilestoneIndex
}

// message +1
func (t *Tangle) FindClosestNextMilestoneOrNil(index milestone.Index) *CachedMilestone {
	lmi := t.GetLatestMilestoneIndex()
	if lmi == 0 {
		// no milestone received yet, check the next 100 milestones as a workaround
		lmi = t.GetSolidMilestoneIndex() + 100
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

		cachedMs := t.GetCachedMilestoneOrNil(index) // milestone +1
		if cachedMs != nil {
			return cachedMs
		}
	}
}
