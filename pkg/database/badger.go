package database

import (
	badgerDB "github.com/dgraph-io/badger/v3"
	"github.com/dgraph-io/badger/v3/options"

	"github.com/iotaledger/hive.go/kvstore/badger"
)

// NewBadgerDB creates a new badger DB instance.
func NewBadgerDB(directory string) *badgerDB.DB {

	opts := badgerDB.DefaultOptions(directory)
	opts.Logger = nil

	opts = opts.WithNumCompactors(2).
		WithNumMemtables(3).
		WithValueLogMaxEntries(10000000).
		WithCompression(options.None)

	db, err := badger.CreateDB(directory, opts)
	if err != nil {
		panic(err)
	}

	return db
}
