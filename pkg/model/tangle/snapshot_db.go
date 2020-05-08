package tangle

import (
	"encoding/binary"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

var snapshotDatabase database.Database

func configureSnapshotDatabase() {
	snapshotDatabase = database.DatabaseWithPrefix(DBPrefixSnapshot)
}

func storeSnapshotInfoInDatabase(snapshot *SnapshotInfo) error {

	if err := snapshotDatabase.Set(database.Entry{
		Key:   []byte("snapshotInfo"),
		Value: snapshot.GetBytes(),
	}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store snapshot info")
	}

	return nil
}

func readSnapshotInfoFromDatabase() (*SnapshotInfo, error) {
	entry, err := snapshotDatabase.Get([]byte("snapshotInfo"))
	if err != nil {
		if err == database.ErrKeyNotFound {
			return nil, nil
		} else {
			return nil, errors.Wrap(NewDatabaseError(err), "failed to retrieve snapshot info")
		}
	}

	info, err := SnapshotInfoFromBytes(entry.Value)
	if err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to convert snapshot info")
	}
	return info, nil
}

func storeSolidEntryPointsInDatabase(points *hornet.SolidEntryPoints) error {
	if points.IsModified() {

		if err := snapshotDatabase.Set(database.Entry{
			Key:   []byte("solidEntryPoints"),
			Value: points.GetBytes(),
		}); err != nil {
			return errors.Wrap(NewDatabaseError(err), "failed to store solid entry points")
		}

		points.SetModified(false)
	}

	return nil
}

func readSolidEntryPointsFromDatabase() (*hornet.SolidEntryPoints, error) {
	entry, err := snapshotDatabase.Get([]byte("solidEntryPoints"))
	if err != nil {
		if err == database.ErrKeyNotFound {
			return nil, nil
		} else {
			return nil, errors.Wrap(NewDatabaseError(err), "failed to retrieve solid entry points")
		}
	}

	points, err := hornet.SolidEntryPointsFromBytes(entry.Value)
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
