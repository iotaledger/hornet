package tangle

import (
	"encoding/binary"
	"fmt"
	
	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/kvstore"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

var (
	snapshotStore       kvstore.KVStore
	snapshotLedgerStore kvstore.KVStore

	snapshotMilestoneIndexKey = "snapshotMilestoneIndex"
)

func configureSnapshotStore(store kvstore.KVStore) {
	snapshotStore = store.WithRealm([]byte{StorePrefixSnapshot})
	snapshotLedgerStore = store.WithRealm([]byte{StorePrefixSnapshotLedger})
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

// StoreSnapshotBalancesInDatabase deletes all old entries and stores the ledger state of the snapshot index
func StoreSnapshotBalancesInDatabase(balances map[trinary.Hash]uint64, index milestone.Index) error {

	// Delete all old entries
	if err := snapshotLedgerStore.Clear(); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete old snapshot balances")
	}

	// Delete index
	if err := snapshotStore.Delete([]byte(snapshotMilestoneIndexKey)); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete old snapshot index")
	}

	batch := snapshotLedgerStore.Batched()

	for address, balance := range balances {
		key := trinary.MustTrytesToBytes(address)[:49]
		if balance != 0 {
			batch.Set(key, bytesFromBalance(balance))
		}
	}

	// Now batch insert all entries
	if err := batch.Commit(); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store snapshot ledger state")
	}

	if err := snapshotStore.Set([]byte(snapshotMilestoneIndexKey), bytesFromMilestoneIndex(index)); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store new snapshot index")
	}

	return nil
}

// GetAllSnapshotBalances returns all balances for the snapshot milestone.
func GetAllSnapshotBalances(abortSignal <-chan struct{}) (map[trinary.Hash]uint64, milestone.Index, error) {

	balances := make(map[trinary.Hash]uint64)

	value, err := snapshotLedgerStore.Get([]byte(snapshotMilestoneIndexKey))
	if err != nil {
		return nil, 0, errors.Wrap(NewDatabaseError(err), "failed to retrieve snapshot milestone index")
	}

	snapshotMilestoneIndex := milestoneIndexFromBytes(value)

	err = snapshotLedgerStore.Iterate([]kvstore.KeyPrefix{}, func(key kvstore.Key, value kvstore.Value) bool {
		select {
		case <-abortSignal:
			return false
		default:
		}
		// Remove prefix from key
		address := trinary.MustBytesToTrytes(key, 81)
		balances[address] = balanceFromBytes(value)
		return true
	})

	if err != nil {
		return nil, 0, err
	}

	var total uint64
	for _, value := range balances {
		total += value
	}

	if total != consts.TotalSupply {
		panic(fmt.Sprintf("GetAllSnapshotBalances() Total does not match supply: %d != %d", total, consts.TotalSupply))
	}

	return balances, snapshotMilestoneIndex, err
}
