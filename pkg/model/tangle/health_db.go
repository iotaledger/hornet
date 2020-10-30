package tangle

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"
)

const (
	DbVersion = 1
)

func (t *Tangle) configureHealthStore(store kvstore.KVStore) {
	t.healthStore = store.WithRealm([]byte{StorePrefixHealth})
	t.setDatabaseVersion()
}

func (t *Tangle) MarkDatabaseCorrupted() {

	if err := t.healthStore.Set([]byte("dbCorrupted"), []byte{}); err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to set database health status"))
	}
}

func (t *Tangle) MarkDatabaseTainted() {

	if err := t.healthStore.Set([]byte("dbTainted"), []byte{}); err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to set database health status"))
	}
}

func (t *Tangle) MarkDatabaseHealthy() {

	if err := t.healthStore.Delete([]byte("dbCorrupted")); err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to set database health status"))
	}
}

func (t *Tangle) IsDatabaseCorrupted() bool {

	contains, err := t.healthStore.Has([]byte("dbCorrupted"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database health status"))
	}
	return contains
}

func (t *Tangle) IsDatabaseTainted() bool {

	contains, err := t.healthStore.Has([]byte("dbTainted"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database health status"))
	}
	return contains
}

func (t *Tangle) setDatabaseVersion() {
	_, err := t.healthStore.Get([]byte("dbVersion"))
	if err == kvstore.ErrKeyNotFound {
		// Only create the entry, if it doesn't exist already (fresh database)
		if err := t.healthStore.Set([]byte("dbVersion"), []byte{DbVersion}); err != nil {
			panic(errors.Wrap(NewDatabaseError(err), "failed to set database version"))
		}
	}
}

func (t *Tangle) IsCorrectDatabaseVersion() bool {

	value, err := t.healthStore.Get([]byte("dbVersion"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database version"))
	}

	if len(value) > 0 {
		return value[0] == DbVersion
	}

	return false
}

// UpdateDatabaseVersion tries to migrate the existing data to the new database version.
func (t *Tangle) UpdateDatabaseVersion() bool {
	value, err := t.healthStore.Get([]byte("dbVersion"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database version"))
	}

	if len(value) < 1 {
		return false
	}

	//currentDbVersion := int(value[0])

	return false
}
