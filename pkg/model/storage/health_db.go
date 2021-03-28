package storage

import (
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/iotaledger/hive.go/kvstore"
)

const (
	DbVersion = 1
)

func (s *Storage) configureHealthStore(store kvstore.KVStore) {
	s.healthStore = store.WithRealm([]byte{common.StorePrefixHealth})
	s.setDatabaseVersion()
}

func (s *Storage) MarkDatabaseCorrupted() {

	if err := s.healthStore.Set([]byte("dbCorrupted"), []byte{}); err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to set database health status"))
	}
	s.healthStore.Flush()
}

func (s *Storage) MarkDatabaseTainted() {

	if err := s.healthStore.Set([]byte("dbTainted"), []byte{}); err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to set database health status"))
	}
	s.healthStore.Flush()
}

func (s *Storage) MarkDatabaseHealthy() {

	if err := s.healthStore.Delete([]byte("dbCorrupted")); err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to set database health status"))
	}
}

func (s *Storage) IsDatabaseCorrupted() bool {

	contains, err := s.healthStore.Has([]byte("dbCorrupted"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database health status"))
	}
	return contains
}

func (s *Storage) IsDatabaseTainted() bool {

	contains, err := s.healthStore.Has([]byte("dbTainted"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database health status"))
	}
	return contains
}

func (s *Storage) setDatabaseVersion() {
	_, err := s.healthStore.Get([]byte("dbVersion"))
	if err == kvstore.ErrKeyNotFound {
		// Only create the entry, if it doesn't exist already (fresh database)
		if err := s.healthStore.Set([]byte("dbVersion"), []byte{DbVersion}); err != nil {
			panic(errors.Wrap(NewDatabaseError(err), "failed to set database version"))
		}
	}
}

func (s *Storage) IsCorrectDatabaseVersion() bool {

	value, err := s.healthStore.Get([]byte("dbVersion"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database version"))
	}

	if len(value) > 0 {
		return value[0] == DbVersion
	}

	return false
}

// UpdateDatabaseVersion tries to migrate the existing data to the new database version.
func (s *Storage) UpdateDatabaseVersion() bool {
	value, err := s.healthStore.Get([]byte("dbVersion"))
	if err != nil {
		panic(errors.Wrap(NewDatabaseError(err), "failed to read database version"))
	}

	if len(value) < 1 {
		return false
	}

	//currentDbVersion := int(value[0])

	return false
}
