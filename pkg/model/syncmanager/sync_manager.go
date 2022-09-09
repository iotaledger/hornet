package syncmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hornet/v2/pkg/protocol"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	isNodeAlmostSyncedThreshold = 2
)

// MilestoneIndexDelta is a the type used to describe an amount of Milestones that should be used to offset a certain Index.
type MilestoneIndexDelta = uint32

type SyncState struct {
	NodeSynced                    bool
	NodeAlmostSynced              bool
	NodeSyncedWithinBelowMaxDepth bool
	LatestMilestoneIndex          iotago.MilestoneIndex
	ConfirmedMilestoneIndex       iotago.MilestoneIndex
}

type SyncManager struct {
	protocolManager *protocol.Manager

	// milestones
	confirmedMilestoneIndex iotago.MilestoneIndex
	confirmedMilestoneLock  syncutils.RWMutex
	latestMilestoneIndex    iotago.MilestoneIndex
	latestMilestoneLock     syncutils.RWMutex

	// node synced
	isNodeSynced                    bool
	isNodeAlmostSynced              bool
	isNodeSyncedWithinBelowMaxDepth bool
	waitForNodeSyncedChannelsLock   syncutils.Mutex
	waitForNodeSyncedChannels       []chan struct{}
}

func New(ledgerIndex iotago.MilestoneIndex, protocolManager *protocol.Manager) (*SyncManager, error) {
	s := &SyncManager{
		protocolManager: protocolManager,
	}

	// set the confirmed milestone index based on the ledger milestone
	if err := s.SetConfirmedMilestoneIndex(ledgerIndex, false); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *SyncManager) ResetMilestoneIndexes() {
	s.confirmedMilestoneLock.Lock()
	s.latestMilestoneLock.Lock()
	defer s.confirmedMilestoneLock.Unlock()
	defer s.latestMilestoneLock.Unlock()

	s.confirmedMilestoneIndex = 0
	s.latestMilestoneIndex = 0
}

func (s *SyncManager) SyncState() *SyncState {
	s.confirmedMilestoneLock.RLock()
	s.latestMilestoneLock.RLock()
	defer s.confirmedMilestoneLock.RUnlock()
	defer s.latestMilestoneLock.RUnlock()

	return &SyncState{
		NodeSynced:                    s.isNodeSynced,
		NodeAlmostSynced:              s.isNodeAlmostSynced,
		NodeSyncedWithinBelowMaxDepth: s.isNodeSyncedWithinBelowMaxDepth,
		LatestMilestoneIndex:          s.latestMilestoneIndex,
		ConfirmedMilestoneIndex:       s.confirmedMilestoneIndex,
	}
}

// IsNodeSynced returns whether the node is synced.
func (s *SyncManager) IsNodeSynced() bool {
	return s.isNodeSynced
}

// IsNodeAlmostSynced returns whether the node is synced within "isNodeAlmostSyncedThreshold".
func (s *SyncManager) IsNodeAlmostSynced() bool {
	return s.isNodeAlmostSynced
}

// IsNodeSyncedWithinBelowMaxDepth returns whether the node is synced within "belowMaxDepth".
func (s *SyncManager) IsNodeSyncedWithinBelowMaxDepth() bool {
	return s.isNodeSyncedWithinBelowMaxDepth
}

// IsNodeSyncedWithThreshold returns whether the node is synced within a given threshold.
func (s *SyncManager) IsNodeSyncedWithThreshold(threshold MilestoneIndexDelta) bool {

	// catch overflow
	if s.latestMilestoneIndex < threshold {
		return true
	}

	return s.confirmedMilestoneIndex >= (s.latestMilestoneIndex - threshold)
}

// WaitForNodeSynced waits at most "timeout" duration for the node to become fully sync.
// if it is not at least synced within threshold, it will return false immediately.
// this is used to avoid small glitches of IsNodeSynced when the sync state is important,
// but a new milestone came in lately.
func (s *SyncManager) WaitForNodeSynced(timeout time.Duration) bool {

	if !s.isNodeAlmostSynced {
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

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// we wait either until the node got synced or we reached the deadline
	select {
	case <-waitForNodeSyncedChan:
	case <-ctx.Done():
	}

	return s.isNodeSynced
}

// The node is synced if LMI != 0 and CMI == LMI.
func (s *SyncManager) updateNodeSynced(confirmedIndex, latestIndex iotago.MilestoneIndex) {
	if latestIndex == 0 {
		s.isNodeSynced = false
		s.isNodeAlmostSynced = false
		s.isNodeSyncedWithinBelowMaxDepth = false

		return
	}

	s.isNodeSynced = confirmedIndex == latestIndex
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
	if latestIndex < isNodeAlmostSyncedThreshold {
		s.isNodeAlmostSynced = true
		s.isNodeSyncedWithinBelowMaxDepth = true

		return
	}
	s.isNodeAlmostSynced = confirmedIndex >= (latestIndex - isNodeAlmostSyncedThreshold)

	// catch overflow
	if latestIndex < iotago.MilestoneIndex(s.protocolManager.Current().BelowMaxDepth) {
		s.isNodeSyncedWithinBelowMaxDepth = true

		return
	}
	s.isNodeSyncedWithinBelowMaxDepth = confirmedIndex >= (latestIndex - MilestoneIndexDelta(s.protocolManager.Current().BelowMaxDepth))
}

// SetConfirmedMilestoneIndex sets the confirmed milestone index.
func (s *SyncManager) SetConfirmedMilestoneIndex(index iotago.MilestoneIndex, updateSynced ...bool) error {
	s.confirmedMilestoneLock.Lock()
	if s.confirmedMilestoneIndex > index {
		return fmt.Errorf("current confirmed milestone (%d) is newer than (%d)", s.confirmedMilestoneIndex, index)
	}
	s.confirmedMilestoneIndex = index
	s.confirmedMilestoneLock.Unlock()

	if len(updateSynced) > 0 && !updateSynced[0] {
		// always call updateNodeSynced if parameter is not given.
		return nil
	}

	s.updateNodeSynced(index, s.LatestMilestoneIndex())

	return nil
}

// OverwriteConfirmedMilestoneIndex is used to set older confirmed milestones (revalidation).
func (s *SyncManager) OverwriteConfirmedMilestoneIndex(index iotago.MilestoneIndex) {
	s.confirmedMilestoneLock.Lock()
	s.confirmedMilestoneIndex = index
	s.confirmedMilestoneLock.Unlock()

	if s.isNodeSynced {
		s.updateNodeSynced(index, s.LatestMilestoneIndex())
	}
}

// ConfirmedMilestoneIndex returns the confirmed milestone index.
func (s *SyncManager) ConfirmedMilestoneIndex() iotago.MilestoneIndex {
	s.confirmedMilestoneLock.RLock()
	defer s.confirmedMilestoneLock.RUnlock()

	return s.confirmedMilestoneIndex
}

// SetLatestMilestoneIndex sets the latest milestone index.
func (s *SyncManager) SetLatestMilestoneIndex(index iotago.MilestoneIndex, updateSynced ...bool) bool {

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

	s.updateNodeSynced(s.ConfirmedMilestoneIndex(), index)

	return true
}

// LatestMilestoneIndex returns the latest milestone index.
func (s *SyncManager) LatestMilestoneIndex() iotago.MilestoneIndex {
	s.latestMilestoneLock.RLock()
	defer s.latestMilestoneLock.RUnlock()

	return s.latestMilestoneIndex
}
