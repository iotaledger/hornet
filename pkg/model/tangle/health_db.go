package tangle

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"

	"github.com/gohornet/hornet/pkg/store"
)

const (
	DbVersion = 9
)

var (
	healthStore kvstore.KVStore
)

func configureHealthStore() {
	healthStore = store.StoreWithPrefix(StorePrefixHealth)
	setDatabaseVersion()
}

func MarkDatabaseCorrupted() {

	if err := healthStore.Set([]byte("dbCorrupted"), []byte{}); err != nil {
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
