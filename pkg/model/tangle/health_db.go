package tangle

import (
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/database"
)

const (
	DbVersion = 9
)

var (
	healthDatabase database.Database
)

func configureHealthDatabase() {
	healthDatabase = database.DatabaseWithPrefix(DBPrefixHealth)
	setDatabaseVersion()
}

func MarkDatabaseCorrupted() {

	if err := healthDatabase.Set(
		database.Entry{
			Key: []byte("dbCorrupted"),
		}); err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to set database health status"))
	}
}

func MarkDatabaseHealthy() {

	if err := healthDatabase.Delete([]byte("dbCorrupted")); err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to set database health status"))
	}
}

func IsDatabaseCorrupted() bool {

	contains, err := healthDatabase.Contains([]byte("dbCorrupted"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database health status"))
	}
	return contains
}

func setDatabaseVersion() {
	_, err := healthDatabase.Get([]byte("dbVersion"))
	if err == database.ErrKeyNotFound {
		// Only create the entry, if it doesn't exist already (fresh database)
		if err := healthDatabase.Set(
			database.Entry{
				Key:   []byte("dbVersion"),
				Value: []byte{DbVersion},
			}); err != nil {
			panic(errors.Wrap(NewDatabaseError(err), "failed to set database version"))
		}
	}
}

func IsCorrectDatabaseVersion() bool {

	entry, err := healthDatabase.Get([]byte("dbVersion"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database version"))
	}

	if len(entry.Value) > 0 {
		return entry.Value[0] == DbVersion
	}

	return false
}
