package tangle

import (
	"encoding/binary"

	"github.com/gohornet/hornet/packages/model/milestone_index"

	"github.com/iotaledger/iota.go/trinary"
	"github.com/pkg/errors"
	"github.com/gohornet/hornet/packages/database"
)

var milestoneDatabase database.Database

func configureMilestoneDatabase() {
	if db, err := database.Get("ms"); err != nil {
		panic(err)
	} else {
		milestoneDatabase = db
	}
}

func databaseKeyForMilestone(ms *Bundle) []byte {
	return databaseKeyForMilestoneIndex(ms.GetMilestoneIndex())
}

func databaseKeyForMilestoneIndex(milestoneIndex milestone_index.MilestoneIndex) []byte {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(milestoneIndex))
	return bytes
}

func storeMilestoneInDatabase(milestone *Bundle) error {

	// Be sure the bundle is already saved in the db
	if err := StoreBundleInDatabase(milestone); err != nil {
		return err
	}

	if err := milestoneDatabase.Set(database.Entry{
		Key:   databaseKeyForMilestone(milestone),
		Value: trinary.MustTrytesToBytes(milestone.GetMilestoneHash()),
	}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store milestone")
	}

	return nil
}

func StoreMilestonesInDatabase(milestones []*Bundle) error {

	// Create entries for all milestones
	var entries []database.Entry
	for _, milestone := range milestones {
		entry := database.Entry{
			Key:   databaseKeyForMilestone(milestone),
			Value: trinary.MustTrytesToBytes(milestone.GetMilestoneHash()),
		}
		entries = append(entries, entry)
	}

	// Now batch insert all entries
	if err := milestoneDatabase.Apply(entries, []database.Key{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store milestones")
	}

	return nil
}

func DeleteMilestoneInDatabase(milestoneIndex milestone_index.MilestoneIndex) error {
	if err := milestoneDatabase.Delete(databaseKeyForMilestoneIndex(milestoneIndex)); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete milestone")
	}

	return nil
}

func readMilestoneTransactionHashFromDatabase(milestoneIndex milestone_index.MilestoneIndex) (trinary.Hash, error) {

	entry, err := milestoneDatabase.Get(databaseKeyForMilestoneIndex(milestoneIndex))
	if err != nil {
		if err == database.ErrKeyNotFound {
			return "", nil
		} else {
			return "", errors.Wrap(NewDatabaseError(err), "failed to retrieve milestone")
		}
	}

	return trinary.MustBytesToTrytes(entry.Value, 81), nil
}

func databaseContainsMilestone(milestoneIndex milestone_index.MilestoneIndex) (bool, error) {
	if contains, err := milestoneDatabase.Contains(databaseKeyForMilestoneIndex(milestoneIndex)); err != nil {
		return contains, errors.Wrap(NewDatabaseError(err), "failed to check if the milestone exists")
	} else {
		return contains, nil
	}
}
