package tangle

import (
	"github.com/pkg/errors"
	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/typeutils"
)

var (
	healthDatabase database.Database
)

func configureHealthDatabase() {
	if db, err := database.Get("health"); err != nil {
		panic(err)
	} else {
		healthDatabase = db
	}
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
