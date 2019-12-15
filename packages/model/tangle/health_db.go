package tangle

import (
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/typeutils"
)

const (
	DbVersion = 2
)

var (
	healthDatabase database.Database
)

func configureHealthDatabase() {
	if db, err := database.Get(DBPrefixHealth); err != nil {
		panic(err)
	} else {
		healthDatabase = db
	}

	setDatabaseVersion()
}

func MarkDatabaseCorrupted() {

	if err := healthDatabase.Set(
		database.Entry{
			Key: typeutils.StringToBytes("dbCorrupted"),
		}); err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to set database health status"))
	}
}

func MarkDatabaseHealthy() {

	if err := healthDatabase.Delete(typeutils.StringToBytes("dbCorrupted")); err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to set database health status"))
	}
}

func IsDatabaseCorrupted() bool {

	contains, err := healthDatabase.Contains(typeutils.StringToBytes("dbCorrupted"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database health status"))
	}
	return contains
}

func setDatabaseVersion() {
	_, err := healthDatabase.Get(typeutils.StringToBytes("dbVersion"))
	if err == database.ErrKeyNotFound {
		// Only create the entry, if it doesn't exist already (fresh database)
		if err := healthDatabase.Set(
			database.Entry{
				Key:   typeutils.StringToBytes("dbVersion"),
				Value: []byte{DbVersion},
			}); err != nil {
			panic(errors.Wrap(NewDatabaseError(err), "failed to set database version"))
		}
	}
}

func IsCorrectDatabaseVersion() bool {

	entry, err := healthDatabase.Get(typeutils.StringToBytes("dbVersion"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database version"))
	}

	if len(entry.Value) > 0 {
		return entry.Value[0] == DbVersion
	}

	return false
}
