package storage

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

type SnapshotInfo struct {
	// The index of the genesis milestone of the network.
	genesisMilestoneIndex iotago.MilestoneIndex
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

// GenesisMilestoneIndex returns the index of the genesis milestone of the network.
func (i *SnapshotInfo) GenesisMilestoneIndex() iotago.MilestoneIndex {
	return i.genesisMilestoneIndex
}

// SnapshotIndex returns the index of the snapshot file.
func (i *SnapshotInfo) SnapshotIndex() iotago.MilestoneIndex {
	return i.snapshotIndex
}

// EntryPointIndex returns the index of the milestone of which the SEPs within the database are from.
func (i *SnapshotInfo) EntryPointIndex() iotago.MilestoneIndex {
	return i.entryPointIndex
}

// PruningIndex returns the index of the milestone before which the tangle history is pruned.
// This is not the same as EntryPointIndex, so we can cleanly prune again even if the pruning was aborted last time.
func (i *SnapshotInfo) PruningIndex() iotago.MilestoneIndex {
	return i.pruningIndex
}

// SnapshotTimestamp returns the timestamp of the target milestone of the snapshot.
func (i *SnapshotInfo) SnapshotTimestamp() time.Time {
	return i.snapshotTimestamp
}

func (i *SnapshotInfo) Deserialize(data []byte, _ serializer.DeSerializationMode, _ interface{}) (int, error) {

	var (
		genesisMilestoneIndex iotago.MilestoneIndex
		snapshotIndex         iotago.MilestoneIndex
		entryPointIndex       iotago.MilestoneIndex
		pruningIndex          iotago.MilestoneIndex
		snapshotTimestamp     iotago.MilestoneIndex
	)

	offset, err := serializer.NewDeserializer(data).
		ReadNum(&genesisMilestoneIndex, func(err error) error {
			return fmt.Errorf("unable to deserialize genesis milestone index: %w", err)
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
		ReadNum(&snapshotTimestamp, func(err error) error {
			return fmt.Errorf("unable to deserialize snapshot timestamp: %w", err)
		}).
		Done()
	if err != nil {
		return offset, err
	}

	i.genesisMilestoneIndex = genesisMilestoneIndex
	i.snapshotIndex = snapshotIndex
	i.entryPointIndex = entryPointIndex
	i.pruningIndex = pruningIndex
	i.snapshotTimestamp = time.Unix(int64(snapshotTimestamp), 0)

	return offset, nil
}

func (i *SnapshotInfo) Serialize(_ serializer.DeSerializationMode, _ interface{}) ([]byte, error) {
	return serializer.NewSerializer().
		WriteNum(i.genesisMilestoneIndex, func(err error) error {
			return fmt.Errorf("unable to serialize genesis milestone index: %w", err)
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
    GenesisMilestoneIndex: %d
    SnapshotIndex: %d
    EntryPointIndex: %d
    PruningIndex: %d
    Timestamp: %v`, s.snapshot.genesisMilestoneIndex, s.snapshot.snapshotIndex, s.snapshot.entryPointIndex, s.snapshot.pruningIndex, s.snapshot.snapshotTimestamp.Truncate(time.Second)))
	}
}

func (s *Storage) SetInitialSnapshotInfo(genesisMilestoneIndex iotago.MilestoneIndex, snapshotIndex iotago.MilestoneIndex, entryPointIndex iotago.MilestoneIndex, pruningIndex iotago.MilestoneIndex, snapshotTimestamp time.Time) error {
	s.snapshotMutex.Lock()
	defer s.snapshotMutex.Unlock()

	s.snapshot = &SnapshotInfo{
		genesisMilestoneIndex: genesisMilestoneIndex,
		snapshotIndex:         snapshotIndex,
		entryPointIndex:       entryPointIndex,
		pruningIndex:          pruningIndex,
		snapshotTimestamp:     snapshotTimestamp,
	}

	return s.storeSnapshotInfo(s.snapshot)
}

func (s *Storage) UpdateSnapshotInfo(snapshotIndex iotago.MilestoneIndex, entryPointIndex iotago.MilestoneIndex, pruningIndex iotago.MilestoneIndex, snapshotTimestamp time.Time) error {
	s.snapshotMutex.Lock()
	defer s.snapshotMutex.Unlock()

	s.snapshot.snapshotIndex = snapshotIndex
	s.snapshot.entryPointIndex = entryPointIndex
	s.snapshot.pruningIndex = pruningIndex
	s.snapshot.snapshotTimestamp = snapshotTimestamp

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
