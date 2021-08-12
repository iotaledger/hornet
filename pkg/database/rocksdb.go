package database

import (
	"runtime"

	"github.com/iotaledger/hive.go/kvstore/rocksdb"
)

// NewRocksDB creates a new RocksDB instance.
func NewRocksDB(path string) (*rocksdb.RocksDB, error) {

	opts := []rocksdb.Option{
		rocksdb.IncreaseParallelism(runtime.NumCPU() - 1),
	}

	return rocksdb.CreateDB(path, opts...)
}
