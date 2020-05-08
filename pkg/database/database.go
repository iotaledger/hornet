package database

import (
	"runtime"

	"github.com/iotaledger/hive.go/database"
	"github.com/iotaledger/hive.go/database/badgerdb"
	"github.com/iotaledger/hive.go/database/boltdb"
	"github.com/iotaledger/hive.go/objectstorage"
	badgerstorage "github.com/iotaledger/hive.go/objectstorage/badger"
	boltstorage "github.com/iotaledger/hive.go/objectstorage/boltdb"

	"github.com/gohornet/hornet/pkg/profile"
)

var (
	directory = "mainnetdb"
	useBolt   bool

	badgerOpts     *profile.BadgerOpts
	ErrKeyNotFound = badgerdb.ErrKeyNotFound
)

type (
	Database  = database.Database
	KeyPrefix = database.KeyPrefix
	Key       = database.Key
	Value     = database.Value
	Entry     = database.Entry
)

func DatabaseWithPrefix(prefix byte) Database {
	if useBolt {
		return boltdb.NewDBWithPrefix([]byte{prefix}, getBoltInstance())
	}
	return badgerdb.NewDatabaseWithPrefix([]byte{prefix}, getBadgerInstance())
}

func StorageWithPrefix(prefix byte) objectstorage.Storage {
	var storage objectstorage.Storage
	if useBolt {
		storage = boltstorage.New(getBoltInstance())
	} else {
		storage = badgerstorage.New(getBadgerInstance())
	}
	return storage.WithRealm([]byte{prefix})
}

// Settings sets DB dir and the badger options
func Settings(dir string, options *profile.BadgerOpts, useBoltDB bool) {
	directory = dir
	badgerOpts = options
	useBolt = useBoltDB

	if useBolt {
		ErrKeyNotFound = boltdb.ErrKeyNotFound
	}
}

// GetDatabaseSize returns the size of the database keys and values.
func GetDatabaseSize() (keys int64, values int64) {

	if useBolt {
		return 0, 0
	}

	return getBadgerInstance().Size()
}

func Cleanup(discardRatio ...float64) error {
	// trigger the go garbage collector to release the used memory
	defer runtime.GC()

	if useBolt {
		return nil
	}
	return cleanupBadgerInstance(discardRatio...)
}

func Close() error {
	if useBolt {
		return getBoltInstance().Close()
	}
	return getBadgerInstance().Close()
}
