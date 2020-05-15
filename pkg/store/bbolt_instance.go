package store

import (
	boltdb2 "github.com/iotaledger/hive.go/kvstore/bolt"
	"sync"

	"go.etcd.io/bbolt"
)

var (
	boltInstance *bbolt.DB
	boltOnce     sync.Once
)

func getBoltInstance() *bbolt.DB {
	boltOnce.Do(func() {
		db, err := boltdb2.CreateDB(directory, "hornet.db")
		if err != nil {
			panic(err)
		}
		boltInstance = db
	})
	return boltInstance
}
