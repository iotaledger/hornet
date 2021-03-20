package database

import (
	"runtime"

	"github.com/iotaledger/hive.go/kvstore/rocksdb"
)

// NewRocksDB creates a new RocksDB instance.
func NewRocksDB(path string) *rocksdb.RocksDB {

	opts := []rocksdb.Option{
		rocksdb.IncreaseParallelism(runtime.NumCPU() - 1),
	}

	db, err := rocksdb.CreateDB(path, opts...)
	if err != nil {
		panic(err)
	}
	return db
}
