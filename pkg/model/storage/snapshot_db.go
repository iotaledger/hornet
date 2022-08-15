package storage

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/common"
)

func (s *Storage) configureSnapshotStore(snapshotStore kvstore.KVStore) error {
	snapshotStore, err := snapshotStore.WithRealm([]byte{common.StorePrefixSnapshot})
	if err != nil {
		return err
	}

	s.snapshotStore = snapshotStore

	return nil
}

func (s *Storage) configureProtocolStore(protocolStore kvstore.KVStore) error {
	protocolStore, err := protocolStore.WithRealm([]byte{common.StorePrefixProtocol})
	if err != nil {
		return err
	}

	s.protocolStore = protocolStore

	return nil
}

func (s *Storage) storeSnapshotInfo(snapshot *SnapshotInfo) error {

	data, err := snapshot.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to serialize snapshot info")
	}

	if err := s.snapshotStore.Set([]byte("snapshotInfo"), data); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store snapshot info")
	}

	return nil
}

func (s *Storage) readSnapshotInfo() (*SnapshotInfo, error) {
	data, err := s.snapshotStore.Get([]byte("snapshotInfo"))
	if err != nil {
		if !errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.Wrap(NewDatabaseError(err), "failed to retrieve snapshot info")
		}

		//nolint:nilnil // nil, nil is ok in this context, even if it is not go idiomatic
		return nil, nil
	}

	info := &SnapshotInfo{}
	if _, err := info.Deserialize(data, serializer.DeSeriModeNoValidation, nil); err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to deserialize snapshot info")
	}

	return info, nil
}

func (s *Storage) storeSolidEntryPoints(points *SolidEntryPoints) error {
	if points.IsModified() {

		if err := s.snapshotStore.Set([]byte("solidEntryPoints"), points.Bytes()); err != nil {
			return errors.Wrap(NewDatabaseError(err), "failed to store solid entry points")
		}

		points.SetModified(false)
	}

	return nil
}

func (s *Storage) readSolidEntryPoints() (*SolidEntryPoints, error) {
	value, err := s.snapshotStore.Get([]byte("solidEntryPoints"))
	if err != nil {
		if !errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.Wrap(NewDatabaseError(err), "failed to retrieve solid entry points")
		}

		//nolint:nilnil // nil, nil is ok in this context, even if it is not go idiomatic
		return nil, nil
	}

	points, err := SolidEntryPointsFromBytes(value)
	if err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to convert solid entry points")
	}

	return points, nil
}
