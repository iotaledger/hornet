package tangle

import (
	"github.com/iotaledger/iota.go/trinary"
	"github.com/pkg/errors"
	"github.com/gohornet/hornet/packages/database"
)

var approversDatabase database.Database

func configureApproversDatabase() {
	if db, err := database.Get("approvers"); err != nil {
		panic(err)
	} else {
		approversDatabase = db
	}
}

func storeApproversInDatabase(approvers []*Approvers) error {

	var entries []database.Entry
	for _, app := range approvers {
		for _, approverHash := range app.GetHashes() {
			entry := database.Entry{
				Key:   databaseKeyForHashPrefixedHash(app.hash, approverHash),
				Value: []byte{},
				Meta:  0,
			}
			entries = append(entries, entry)
		}
	}

	// Now batch insert all entries
	if err := approversDatabase.Apply(entries, []database.Key{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store approvers")
	}

	return nil
}

func readApproversForTransactionFromDatabase(txHash trinary.Hash) (*Approvers, error) {
	app := NewApprovers(txHash)
	err := approversDatabase.ForEachPrefixKeyOnly(databaseKeyForHashPrefix(txHash), func(entry database.KeyOnlyEntry) (stop bool) {
		app.Add(trinary.MustBytesToTrytes(entry.Key, 81))
		return false
	})

	if err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to read approvers of transaction from database")
	} else {
		return app, nil
	}
}

func DeleteApproversInDatabase(approvers []*Approvers) error {

	var deletions []database.Key
	for _, app := range approvers {
		for _, approverHash := range app.GetHashes() {
			deletions = append(deletions, databaseKeyForHashPrefixedHash(app.hash, approverHash))
		}
	}

	// Now batch delete all entries
	if err := approversDatabase.Apply([]database.Entry{}, deletions); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete approvers")
	}

	return nil
}
