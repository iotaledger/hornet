package syncmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/syncutils"
)

const (
	isNodeAlmostSyncedThreshold = 2
)

type SyncManager struct {
	utxoManager *utxo.Manager
	// belowMaxDepth is the maximum allowed delta
	// value between OCRI of a given message in relation to the current CMI before it gets lazy.
	belowMaxDepth milestone.Index

	// milestones
	confirmedMilestoneIndex milestone.Index
	confirmedMilestoneLock  syncutils.RWMutex
	latestMilestoneIndex    milestone.Index
	latestMilestoneLock     syncutils.RWMutex

	// node synced
	isNodeSynced                    bool
	isNodeAlmostSynced              bool
	isNodeSyncedWithinBelowMaxDepth bool
	waitForNodeSyncedChannelsLock   syncutils.Mutex
	waitForNodeSyncedChannels       []chan struct{}
}

func New(utxoManager *utxo.Manager, belowMaxDepth int) (*SyncManager, error) {
	s := &SyncManager{
		utxoManager:   utxoManager,
		belowMaxDepth: milestone.Index(belowMaxDepth),
	}

	if err := s.loadConfirmedMilestoneFromDatabase(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *SyncManager) loadConfirmedMilestoneFromDatabase() error {

	ledgerMilestoneIndex, err := s.utxoManager.ReadLedgerIndex()
	if err != nil {
		return err
	}

	// set the confirmed milestone index based on the ledger milestone
	return s.SetConfirmedMilestoneIndex(ledgerMilestoneIndex, false)
}

func (s *SyncManager) ResetMilestoneIndexes() {
	s.confirmedMilestoneLock.Lock()
	s.latestMilestoneLock.Lock()
	defer s.confirmedMilestoneLock.Unlock()
	defer s.latestMilestoneLock.Unlock()

	s.confirmedMilestoneIndex = 0
	s.latestMilestoneIndex = 0
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
func (s *SyncManager) IsNodeSyncedWithThreshold(threshold milestone.Index) bool {

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
func (s *SyncManager) updateNodeSynced(confirmedIndex, latestIndex milestone.Index) {
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
	if latestIndex < s.belowMaxDepth {
		s.isNodeSyncedWithinBelowMaxDepth = true
		return
	}
	s.isNodeSyncedWithinBelowMaxDepth = confirmedIndex >= (latestIndex - s.belowMaxDepth)
}

// SetConfirmedMilestoneIndex sets the confirmed milestone index.
func (s *SyncManager) SetConfirmedMilestoneIndex(index milestone.Index, updateSynced ...bool) error {
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
func (s *SyncManager) OverwriteConfirmedMilestoneIndex(index milestone.Index) {
	s.confirmedMilestoneLock.Lock()
	s.confirmedMilestoneIndex = index
	s.confirmedMilestoneLock.Unlock()

	if s.isNodeSynced {
		s.updateNodeSynced(index, s.LatestMilestoneIndex())
	}
}

// ConfirmedMilestoneIndex returns the confirmed milestone index.
func (s *SyncManager) ConfirmedMilestoneIndex() milestone.Index {
	s.confirmedMilestoneLock.RLock()
	defer s.confirmedMilestoneLock.RUnlock()

	return s.confirmedMilestoneIndex
}

// SetLatestMilestoneIndex sets the latest milestone index.
func (s *SyncManager) SetLatestMilestoneIndex(index milestone.Index, updateSynced ...bool) bool {

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
func (s *SyncManager) LatestMilestoneIndex() milestone.Index {
	s.latestMilestoneLock.RLock()
	defer s.latestMilestoneLock.RUnlock()

	return s.latestMilestoneIndex
}
