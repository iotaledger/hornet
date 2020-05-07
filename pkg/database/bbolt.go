package database

import (
	"sync"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/objectstorage/boltdb"
	"go.etcd.io/bbolt"
)

var (
	boltOnce        sync.Once
	bolt            *bbolt.DB
	boltStorage     objectstorage.Storage
	boltStorageOnce sync.Once
)

func Bolt() *bbolt.DB {
	boltOnce.Do(func() {
		db, err := bbolt.Open("hornet.db", 0666, nil)
		if err != nil {
			panic(err)
		}
		bolt = db
	})
	return bolt
}

func BoltStorage() objectstorage.Storage {
	boltStorageOnce.Do(func() {
		boltStorage = boltdb.New(Bolt())
	})
	return boltStorage
}
