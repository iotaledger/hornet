package database

import (
	"go.etcd.io/bbolt"

	"github.com/iotaledger/hive.go/kvstore/bolt"
)

// NewBoltDB creates a new bbolt DB instance.
func NewBoltDB(directory string, filename string) (*bbolt.DB, error) {
	opts := &bbolt.Options{
		NoSync: true,
	}

	return bolt.CreateDB(directory, filename, opts)
}
