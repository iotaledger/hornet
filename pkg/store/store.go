package store

import (
	"errors"
	"github.com/iotaledger/hive.go/kvstore"
	"runtime"

	boltstorage "github.com/iotaledger/hive.go/kvstore/bolt"
)

var (
	directory           = "mainnetdb"
	ErrNothingToCleanup = errors.New("Nothing to clean up")
)

func StoreWithPrefix(prefix byte) kvstore.KVStore {
	return boltstorage.New(getBoltInstance()).WithRealm([]byte{prefix})
}

// Settings sets DB dir and the badger options
func Settings(dir string) {
	directory = dir
}

// GetSize returns the size of the database keys and values.
func GetSize() (keys int64, values int64) {
	//TODO: check filesystem for size
	return 0, 0
}

func Cleanup(discardRatio ...float64) error {
	// trigger the go garbage collector to release the used memory
	defer runtime.GC()
	return ErrNothingToCleanup
}

func Close() error {
	return getBoltInstance().Close()
}
