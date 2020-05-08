package database

import (
	"github.com/iotaledger/hive.go/database/boltdb"
	"sync"

	"go.etcd.io/bbolt"
)

var (
	boltInstance *bbolt.DB
	boltOnce     sync.Once
)

func getBoltInstance() *bbolt.DB {
	boltOnce.Do(func() {
		db, err := boltdb.CreateDB(directory, "hornet.db")
		if err != nil {
			panic(err)
		}
		boltInstance = db
	})
	return boltInstance
}
