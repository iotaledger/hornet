package storage

import (
	"encoding/binary"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/kvstore"
)

const (
	snapshotMilestoneIndexKey = "snapshotMilestoneIndex"
)

func (s *Storage) configureSnapshotStore(store kvstore.KVStore) {
	s.snapshotStore = store.WithRealm([]byte{common.StorePrefixSnapshot})
}

func (s *Storage) storeSnapshotInfo(snapshot *SnapshotInfo) error {

	if err := s.snapshotStore.Set([]byte("snapshotInfo"), snapshot.GetBytes()); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store snapshot info")
	}

	return nil
}

func (s *Storage) readSnapshotInfo() (*SnapshotInfo, error) {
	value, err := s.snapshotStore.Get([]byte("snapshotInfo"))
	if err != nil {
		if err != kvstore.ErrKeyNotFound {
			return nil, errors.Wrap(NewDatabaseError(err), "failed to retrieve snapshot info")
		}
		return nil, nil
	}

	info, err := SnapshotInfoFromBytes(value)
	if err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to convert snapshot info")
	}
	return info, nil
}

func (s *Storage) storeSolidEntryPoints(points *SolidEntryPoints) error {
	if points.IsModified() {

		if err := s.snapshotStore.Set([]byte("solidEntryPoints"), points.GetBytes()); err != nil {
			return errors.Wrap(NewDatabaseError(err), "failed to store solid entry points")
		}

		points.SetModified(false)
	}

	return nil
}

func (s *Storage) readSolidEntryPoints() (*SolidEntryPoints, error) {
	value, err := s.snapshotStore.Get([]byte("solidEntryPoints"))
	if err != nil {
		if err != kvstore.ErrKeyNotFound {
			return nil, errors.Wrap(NewDatabaseError(err), "failed to retrieve solid entry points")
		}
		return nil, nil
	}

	points, err := SolidEntryPointsFromBytes(value)
	if err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to convert solid entry points")
	}
	return points, nil
}

func bytesFromMilestoneIndex(milestoneIndex milestone.Index) []byte {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(milestoneIndex))
	return bytes
}

func milestoneIndexFromBytes(bytes []byte) milestone.Index {
	return milestone.Index(binary.LittleEndian.Uint32(bytes))
}
