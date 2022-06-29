package storage

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

type SnapshotInfo struct {
	// The index of the first milestone of the network. Zero if there is none.
	firstMilestoneIndex iotago.MilestoneIndex
	// The index of the snapshot file.
	snapshotIndex iotago.MilestoneIndex
	// The index of the milestone of which the SEPs within the database are from.
	entryPointIndex iotago.MilestoneIndex
	// The index of the milestone before which the tangle history is pruned.
	// This is not the same as EntryPointIndex, so we can cleanly prune again even if the pruning was aborted last time.
	pruningIndex iotago.MilestoneIndex
	// The timestamp of the target milestone of the snapshot.
	snapshotTimestamp time.Time
}

// The index of the first milestone of the network. Zero if there is none.
func (i *SnapshotInfo) FirstMilestoneIndex() iotago.MilestoneIndex {
	return i.firstMilestoneIndex
}

// The index of the snapshot file.
func (i *SnapshotInfo) SnapshotIndex() iotago.MilestoneIndex {
	return i.snapshotIndex
}

// The index of the milestone of which the SEPs within the database are from.
func (i *SnapshotInfo) EntryPointIndex() iotago.MilestoneIndex {
	return i.entryPointIndex
}

// The index of the milestone before which the tangle history is pruned.
// This is not the same as EntryPointIndex, so we can cleanly prune again even if the pruning was aborted last time.
func (i *SnapshotInfo) PruningIndex() iotago.MilestoneIndex {
	return i.pruningIndex
}

// The timestamp of the target milestone of the snapshot.
func (i *SnapshotInfo) SnapshotTimestamp() time.Time {
	return i.snapshotTimestamp
}

func (i *SnapshotInfo) Deserialize(data []byte, _ serializer.DeSerializationMode, _ interface{}) (int, error) {

	var (
		firstMilestoneIndex iotago.MilestoneIndex
		snapshotIndex       iotago.MilestoneIndex
		entryPointIndex     iotago.MilestoneIndex
		pruningIndex        iotago.MilestoneIndex
		timestamp           iotago.MilestoneIndex
	)

	offset, err := serializer.NewDeserializer(data).
		ReadNum(&firstMilestoneIndex, func(err error) error {
			return fmt.Errorf("unable to deserialize first milestone index: %w", err)
		}).
		ReadNum(&snapshotIndex, func(err error) error {
			return fmt.Errorf("unable to deserialize snapshot index: %w", err)
		}).
		ReadNum(&entryPointIndex, func(err error) error {
			return fmt.Errorf("unable to deserialize entry point index: %w", err)
		}).
		ReadNum(&pruningIndex, func(err error) error {
			return fmt.Errorf("unable to deserialize pruning index: %w", err)
		}).
		ReadNum(&timestamp, func(err error) error {
			return fmt.Errorf("unable to deserialize timestamp: %w", err)
		}).
		Done()
	if err != nil {
		return offset, err
	}

	i.firstMilestoneIndex = firstMilestoneIndex
	i.snapshotIndex = snapshotIndex
	i.entryPointIndex = entryPointIndex
	i.pruningIndex = pruningIndex
	i.snapshotTimestamp = time.Unix(int64(timestamp), 0)

	return offset, nil
}

func (i *SnapshotInfo) Serialize(_ serializer.DeSerializationMode, _ interface{}) ([]byte, error) {
	return serializer.NewSerializer().
		WriteNum(i.firstMilestoneIndex, func(err error) error {
			return fmt.Errorf("unable to serialize first milestone index: %w", err)
		}).
		WriteNum(i.snapshotIndex, func(err error) error {
			return fmt.Errorf("unable to serialize snapshot index: %w", err)
		}).
		WriteNum(i.entryPointIndex, func(err error) error {
			return fmt.Errorf("unable to serialize entry point index: %w", err)
		}).
		WriteNum(i.pruningIndex, func(err error) error {
			return fmt.Errorf("unable to serialize pruning index: %w", err)
		}).
		WriteNum(uint32(i.snapshotTimestamp.Unix()), func(err error) error {
			return fmt.Errorf("unable to serialize timestamp: %w", err)
		}).
		Serialize()
}

func (s *Storage) loadSnapshotInfo() error {
	s.snapshotMutex.Lock()
	defer s.snapshotMutex.Unlock()

	info, err := s.readSnapshotInfo()
	if err != nil {
		return err
	}

	s.snapshot = info
	return nil
}

func (s *Storage) PrintSnapshotInfo() {
	s.snapshotMutex.RLock()
	defer s.snapshotMutex.RUnlock()

	if s.snapshot != nil {
		println(fmt.Sprintf(`SnapshotInfo:
    FirstMilestoneIndex: %d
    SnapshotIndex: %d
    EntryPointIndex: %d
    PruningIndex: %d
    Timestamp: %v`, s.snapshot.firstMilestoneIndex, s.snapshot.snapshotIndex, s.snapshot.entryPointIndex, s.snapshot.pruningIndex, s.snapshot.snapshotTimestamp.Truncate(time.Second)))
	}
}

func (s *Storage) SetInitialSnapshotInfo(firstMilestoneIndex iotago.MilestoneIndex, snapshotIndex iotago.MilestoneIndex, entryPointIndex iotago.MilestoneIndex, pruningIndex iotago.MilestoneIndex, timestamp time.Time) error {
	s.snapshotMutex.Lock()
	defer s.snapshotMutex.Unlock()

	s.snapshot = &SnapshotInfo{
		firstMilestoneIndex: firstMilestoneIndex,
		snapshotIndex:       snapshotIndex,
		entryPointIndex:     entryPointIndex,
		pruningIndex:        pruningIndex,
		snapshotTimestamp:   timestamp,
	}

	return s.storeSnapshotInfo(s.snapshot)
}

func (s *Storage) UpdateSnapshotInfo(snapshotIndex iotago.MilestoneIndex, entryPointIndex iotago.MilestoneIndex, pruningIndex iotago.MilestoneIndex, timestamp time.Time) error {
	s.snapshotMutex.Lock()
	defer s.snapshotMutex.Unlock()

	s.snapshot.snapshotIndex = snapshotIndex
	s.snapshot.entryPointIndex = entryPointIndex
	s.snapshot.pruningIndex = pruningIndex
	s.snapshot.snapshotTimestamp = timestamp

	return s.storeSnapshotInfo(s.snapshot)
}

func (s *Storage) SetSnapshotIndex(snapshotIndex iotago.MilestoneIndex, snapshotTimestamp time.Time) error {
	s.snapshotMutex.Lock()
	defer s.snapshotMutex.Unlock()

	s.snapshot.snapshotIndex = snapshotIndex
	s.snapshot.snapshotTimestamp = snapshotTimestamp

	return s.storeSnapshotInfo(s.snapshot)
}

func (s *Storage) SetEntryPointIndex(entryPointIndex iotago.MilestoneIndex) error {
	s.snapshotMutex.Lock()
	defer s.snapshotMutex.Unlock()

	s.snapshot.entryPointIndex = entryPointIndex

	return s.storeSnapshotInfo(s.snapshot)
}

func (s *Storage) SetPruningIndex(pruningIndex iotago.MilestoneIndex) error {
	s.snapshotMutex.Lock()
	defer s.snapshotMutex.Unlock()

	s.snapshot.pruningIndex = pruningIndex

	return s.storeSnapshotInfo(s.snapshot)
}

func (s *Storage) SnapshotInfo() *SnapshotInfo {
	s.snapshotMutex.RLock()
	defer s.snapshotMutex.RUnlock()

	return s.snapshot
}
