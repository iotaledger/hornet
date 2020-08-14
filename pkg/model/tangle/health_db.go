package tangle

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"
)

const (
	DbVersion = 2
)

var (
	healthStore kvstore.KVStore
)

func configureHealthStore(store kvstore.KVStore) {
	healthStore = store.WithRealm([]byte{StorePrefixHealth})
	setDatabaseVersion()
}

func MarkDatabaseCorrupted() {

	if err := healthStore.Set([]byte("dbCorrupted"), []byte{}); err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to set database health status"))
	}
}

func MarkDatabaseTainted() {

	if err := healthStore.Set([]byte("dbTainted"), []byte{}); err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to set database health status"))
	}
}

func MarkDatabaseHealthy() {

	if err := healthStore.Delete([]byte("dbCorrupted")); err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to set database health status"))
	}
}

func IsDatabaseCorrupted() bool {

	contains, err := healthStore.Has([]byte("dbCorrupted"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database health status"))
	}
	return contains
}

func IsDatabaseTainted() bool {

	contains, err := healthStore.Has([]byte("dbTainted"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database health status"))
	}
	return contains
}

func setDatabaseVersion() {
	_, err := healthStore.Get([]byte("dbVersion"))
	if err == kvstore.ErrKeyNotFound {
		// Only create the entry, if it doesn't exist already (fresh database)
		if err := healthStore.Set([]byte("dbVersion"), []byte{DbVersion}); err != nil {
			panic(errors.Wrap(NewDatabaseError(err), "failed to set database version"))
		}
	}
}

func IsCorrectDatabaseVersion() bool {

	value, err := healthStore.Get([]byte("dbVersion"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database version"))
	}

	if len(value) > 0 {
		return value[0] == DbVersion
	}

	return false
}

// UpdateDatabaseVersion tries to migrate the existing data to the new database version.
func UpdateDatabaseVersion() bool {
	value, err := healthStore.Get([]byte("dbVersion"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database version"))
	}

	if len(value) < 1 {
		return false
	}

	currentDbVersion := int(value[0])

	if currentDbVersion == 1 && DbVersion == 2 {
		// add information about trunk and branch to transaction metadata
		if err := migrateVersionOneToVersionTwo(); err != nil {
			panic(errors.Wrap(NewDatabaseError(err), "failed to migrate database to new version"))
		}
		setDatabaseVersion()
		return true
	}

	return false
}

func migrateVersionOneToVersionTwo() error {
	// this is a soft migration in the metadata storage
	// trunk an branch hashes were added to the metadata
	return nil
}
