package tangle

import (
	"encoding/binary"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/store"
)

var snapshotStore kvstore.KVStore

func configureSnapshotStore() {
	snapshotStore = store.StoreWithPrefix(StorePrefixSnapshot)
}

func storeSnapshotInfo(snapshot *SnapshotInfo) error {

	if err := snapshotStore.Set([]byte("snapshotInfo"), snapshot.GetBytes()); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store snapshot info")
	}

	return nil
}

func readSnapshotInfo() (*SnapshotInfo, error) {
	value, err := snapshotStore.Get([]byte("snapshotInfo"))
	if err != nil {
		if err == kvstore.ErrKeyNotFound {
			return nil, nil
		} else {
			return nil, errors.Wrap(NewDatabaseError(err), "failed to retrieve snapshot info")
		}
	}

	info, err := SnapshotInfoFromBytes(value)
	if err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to convert snapshot info")
	}
	return info, nil
}

func storeSolidEntryPoints(points *hornet.SolidEntryPoints) error {
	if points.IsModified() {

		if err := snapshotStore.Set([]byte("solidEntryPoints"), points.GetBytes()); err != nil {
			return errors.Wrap(NewDatabaseError(err), "failed to store solid entry points")
		}

		points.SetModified(false)
	}

	return nil
}

func readSolidEntryPoints() (*hornet.SolidEntryPoints, error) {
	value, err := snapshotStore.Get([]byte("solidEntryPoints"))
	if err != nil {
		if err == kvstore.ErrKeyNotFound {
			return nil, nil
		} else {
			return nil, errors.Wrap(NewDatabaseError(err), "failed to retrieve solid entry points")
		}
	}

	points, err := hornet.SolidEntryPointsFromBytes(value)
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
