package storage

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/serializer/v2"
)

var (
	ErrParseSnapshotInfoFailed = errors.New("Parsing of snapshot info failed")
)

type SnapshotInfo struct {
	NetworkID       uint64
	SnapshotIndex   milestone.Index
	EntryPointIndex milestone.Index
	PruningIndex    milestone.Index
	Timestamp       time.Time
}

func (s *Storage) loadSnapshotInfo() error {
	info, err := s.readSnapshotInfo()
	if err != nil {
		return err
	}

	s.snapshot = info
	return nil
}

func (s *Storage) PrintSnapshotInfo() {
	if s.snapshot != nil {
		println(fmt.Sprintf(`SnapshotInfo:
    NetworkID: %d
    SnapshotIndex: %d
    EntryPointIndex: %d
    PruningIndex: %d
    Timestamp: %v`, s.snapshot.NetworkID, s.snapshot.SnapshotIndex, s.snapshot.EntryPointIndex, s.snapshot.PruningIndex, s.snapshot.Timestamp.Truncate(time.Second)))
	}
}

func (i *SnapshotInfo) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) (int, error) {
	var timestamp uint32

	offset, err := serializer.NewDeserializer(data).
		ReadNum(&i.NetworkID, func(err error) error {
			return fmt.Errorf("unable to deserialize network ID: %w", err)
		}).
		ReadNum(&i.SnapshotIndex, func(err error) error {
			return fmt.Errorf("unable to deserialize snapshot index: %w", err)
		}).
		ReadNum(&i.EntryPointIndex, func(err error) error {
			return fmt.Errorf("unable to deserialize entry point index: %w", err)
		}).
		ReadNum(&i.PruningIndex, func(err error) error {
			return fmt.Errorf("unable to deserialize pruning index: %w", err)
		}).
		ReadNum(&timestamp, func(err error) error {
			return fmt.Errorf("unable to deserialize timestamp: %w", err)
		}).
		Done()
	if err != nil {
		return offset, err
	}

	i.Timestamp = time.Unix(int64(timestamp), 0)

	return offset, nil
}

func (i *SnapshotInfo) Serialize(deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) ([]byte, error) {
	return serializer.NewSerializer().
		WriteNum(i.NetworkID, func(err error) error {
			return fmt.Errorf("unable to serialize network ID: %w", err)
		}).
		WriteNum(i.SnapshotIndex, func(err error) error {
			return fmt.Errorf("unable to serialize snapshot index: %w", err)
		}).
		WriteNum(i.EntryPointIndex, func(err error) error {
			return fmt.Errorf("unable to serialize entry point index: %w", err)
		}).
		WriteNum(i.PruningIndex, func(err error) error {
			return fmt.Errorf("unable to serialize pruning index: %w", err)
		}).
		WriteNum(uint32(i.Timestamp.Unix()), func(err error) error {
			return fmt.Errorf("unable to serialize timestamp: %w", err)
		}).
		Serialize()
}

func (s *Storage) SetSnapshotMilestone(networkID uint64, snapshotIndex milestone.Index, entryPointIndex milestone.Index, pruningIndex milestone.Index, timestamp time.Time) error {

	sn := &SnapshotInfo{
		NetworkID:       networkID,
		SnapshotIndex:   snapshotIndex,
		EntryPointIndex: entryPointIndex,
		PruningIndex:    pruningIndex,
		Timestamp:       timestamp,
	}

	return s.SetSnapshotInfo(sn)
}

func (s *Storage) SetSnapshotInfo(sn *SnapshotInfo) error {
	s.snapshotMutex.Lock()
	defer s.snapshotMutex.Unlock()

	err := s.storeSnapshotInfo(sn)
	if err != nil {
		return err
	}
	s.snapshot = sn
	return nil
}

func (s *Storage) SnapshotInfo() *SnapshotInfo {
	s.snapshotMutex.RLock()
	defer s.snapshotMutex.RUnlock()

	return s.snapshot
}
