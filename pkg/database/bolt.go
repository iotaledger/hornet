package database

import (
	"go.etcd.io/bbolt"

	"github.com/iotaledger/hive.go/kvstore/bolt"
)

// NewBoltDB creates a new bbolt DB instance.
func NewBoltDB(directory string, filename string) *bbolt.DB {
	opts := &bbolt.Options{
		NoSync: true,
	}

	db, err := bolt.CreateDB(directory, filename, opts)
	if err != nil {
		panic(err)
	}

	return db
}
