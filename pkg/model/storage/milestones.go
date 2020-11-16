package storage

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

func (s *Storage) ConfigureMilestones(cooKeyManager *keymanager.KeyManager, cooMilestonePublicKeyCount int, cooMilestoneMerkleHashFunc crypto.Hash) {
	s.keyManager = cooKeyManager
	s.milestonePublicKeyCount = cooMilestonePublicKeyCount
	s.coordinatorMilestoneMerkleHashFunc = cooMilestoneMerkleHashFunc
}

func (s *Storage) KeyManager() *keymanager.KeyManager {
	return s.keyManager
}

func (s *Storage) GetMilestoneMerkleHashFunc() crypto.Hash {
	return s.coordinatorMilestoneMerkleHashFunc
}

func (s *Storage) ResetMilestoneIndexes() {
	s.solidMilestoneLock.Lock()
	s.latestMilestoneLock.Lock()
	defer s.solidMilestoneLock.Unlock()
	defer s.latestMilestoneLock.Unlock()

	s.solidMilestoneIndex = 0
	s.latestMilestoneIndex = 0
}

// GetMilestoneOrNil returns the CachedMessage of a milestone index or nil if it doesn't exist.
// message +1
func (s *Storage) GetMilestoneCachedMessageOrNil(milestoneIndex milestone.Index) *CachedMessage {

	cachedMs := s.GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
	if cachedMs == nil {
		return nil
	}
	defer cachedMs.Release(true) // milestone -1

	return s.GetCachedMessageOrNil(cachedMs.GetMilestone().MessageID)
}

// IsNodeSynced returns whether the node is synced.
func (s *Storage) IsNodeSynced() bool {
	return s.isNodeSynced
}

// IsNodeSyncedWithThreshold returns whether the node is synced within a certain threshold.
func (s *Storage) IsNodeSyncedWithThreshold() bool {
	return s.isNodeSyncedThreshold
}

// WaitForNodeSynced waits at most "timeout" duration for the node to become fully sync.
// if it is not at least synced within threshold, it will return false immediately.
// this is used to avoid small glitches of IsNodeSynced when the sync state is important,
// but a new milestone came in lately.
func (s *Storage) WaitForNodeSynced(timeout time.Duration) bool {

	if !s.isNodeSyncedThreshold {
		// node is not even synced within threshold, and therefore it is unsync
		return false
	}

	if s.isNodeSynced {
		// node is synced, no need to wait
		return true
	}

	// create a channel that gets closed if the node got synced
	s.waitForNodeSyncedChannelsLock.Lock()
	waitForNodeSyncedChan := make(chan struct{})
	s.waitForNodeSyncedChannels = append(s.waitForNodeSyncedChannels, waitForNodeSyncedChan)
	s.waitForNodeSyncedChannelsLock.Unlock()

	// check again after the channel was created
	if s.isNodeSynced {
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

	return s.isNodeSynced
}

// The node is synced if LMI != 0 and LSMI == LMI.
func (s *Storage) updateNodeSynced(latestSolidIndex, latestIndex milestone.Index) {
	if latestIndex == 0 {
		s.isNodeSynced = false
		s.isNodeSyncedThreshold = false
		return
	}

	s.isNodeSynced = latestSolidIndex == latestIndex
	if s.isNodeSynced {
		// if the node is sync, signal all waiting routines at the end
		defer func() {
			s.waitForNodeSyncedChannelsLock.Lock()
			defer s.waitForNodeSyncedChannelsLock.Unlock()

			// signal all routines that are waiting
			for _, channel := range s.waitForNodeSyncedChannels {
				close(channel)
			}

			// create an empty slice for new signals
			s.waitForNodeSyncedChannels = make([]chan struct{}, 0)
		}()
	}

	// catch overflow
	if latestIndex < isNodeSyncedWithinThreshold {
		s.isNodeSyncedThreshold = true
		return
	}

	s.isNodeSyncedThreshold = latestSolidIndex >= (latestIndex - isNodeSyncedWithinThreshold)
}

// SetSolidMilestoneIndex sets the solid milestone index.
func (s *Storage) SetSolidMilestoneIndex(index milestone.Index, updateSynced ...bool) {
	s.solidMilestoneLock.Lock()
	if s.solidMilestoneIndex > index {
		panic(fmt.Sprintf("current solid milestone (%d) is newer than (%d)", s.solidMilestoneIndex, index))
	}
	s.solidMilestoneIndex = index
	s.solidMilestoneLock.Unlock()

	if len(updateSynced) > 0 && !updateSynced[0] {
		// always call updateNodeSynced if parameter is not given.
		return
	}

	s.updateNodeSynced(index, s.GetLatestMilestoneIndex())
}

// OverwriteSolidMilestoneIndex is used to set older solid milestones (revalidation).
func (s *Storage) OverwriteSolidMilestoneIndex(index milestone.Index) {
	s.solidMilestoneLock.Lock()
	s.solidMilestoneIndex = index
	s.solidMilestoneLock.Unlock()

	if s.isNodeSynced {
		s.updateNodeSynced(index, s.GetLatestMilestoneIndex())
	}
}

// GetSolidMilestoneIndex returns the latest solid milestone index.
func (s *Storage) GetSolidMilestoneIndex() milestone.Index {
	s.solidMilestoneLock.RLock()
	defer s.solidMilestoneLock.RUnlock()

	return s.solidMilestoneIndex
}

// SetLatestMilestoneIndex sets the latest milestone index.
func (s *Storage) SetLatestMilestoneIndex(index milestone.Index, updateSynced ...bool) bool {

	s.latestMilestoneLock.Lock()

	if s.latestMilestoneIndex >= index {
		// current LMI is bigger than new LMI => abort
		s.latestMilestoneLock.Unlock()
		return false
	}

	s.latestMilestoneIndex = index
	s.latestMilestoneLock.Unlock()

	if len(updateSynced) > 0 && !updateSynced[0] {
		// always call updateNodeSynced if parameter is not given
		return true
	}

	s.updateNodeSynced(s.GetSolidMilestoneIndex(), index)

	return true
}

// GetLatestMilestoneIndex returns the latest milestone index.
func (s *Storage) GetLatestMilestoneIndex() milestone.Index {
	s.latestMilestoneLock.RLock()
	defer s.latestMilestoneLock.RUnlock()

	return s.latestMilestoneIndex
}

// message +1
func (s *Storage) FindClosestNextMilestoneOrNil(index milestone.Index) *CachedMilestone {
	lmi := s.GetLatestMilestoneIndex()
	if lmi == 0 {
		// no milestone received yet, check the next 100 milestones as a workaround
		lmi = s.GetSolidMilestoneIndex() + 100
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

		cachedMs := s.GetCachedMilestoneOrNil(index) // milestone +1
		if cachedMs != nil {
			return cachedMs
		}
	}
}
