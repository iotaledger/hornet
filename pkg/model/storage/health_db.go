package storage

import (
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/iotaledger/hive.go/kvstore"
)

const (
	DBVersion = 1
)

func (s *Storage) configureHealthStore(store kvstore.KVStore) error {
	s.healthStore = store.WithRealm([]byte{common.StorePrefixHealth})
	return s.setDatabaseVersion()
}

func (s *Storage) MarkDatabaseCorrupted() error {

	if err := s.healthStore.Set([]byte("dbCorrupted"), []byte{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to set database health status")
	}
	return s.healthStore.Flush()
}

func (s *Storage) MarkDatabaseTainted() error {

	if err := s.healthStore.Set([]byte("dbTainted"), []byte{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to set database health status")
	}
	return s.healthStore.Flush()
}

func (s *Storage) MarkDatabaseHealthy() error {

	if err := s.healthStore.Delete([]byte("dbCorrupted")); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to set database health status")
	}

	return nil
}

func (s *Storage) IsDatabaseCorrupted() (bool, error) {

	contains, err := s.healthStore.Has([]byte("dbCorrupted"))
	if err != nil {
		return true, errors.Wrap(NewDatabaseError(err), "failed to read database health status")
	}
	return contains, nil
}

func (s *Storage) IsDatabaseTainted() (bool, error) {

	contains, err := s.healthStore.Has([]byte("dbTainted"))
	if err != nil {
		return true, errors.Wrap(NewDatabaseError(err), "failed to read database health status")
	}
	return contains, nil
}

func (s *Storage) setDatabaseVersion() error {

	_, err := s.healthStore.Get([]byte("dbVersion"))
	if errors.Is(err, kvstore.ErrKeyNotFound) {
		// Only create the entry, if it doesn't exist already (fresh database)
		if err := s.healthStore.Set([]byte("dbVersion"), []byte{DBVersion}); err != nil {
			return errors.Wrap(NewDatabaseError(err), "failed to set database version")
		}
	}
	return nil
}

func (s *Storage) IsCorrectDatabaseVersion() (bool, error) {

	value, err := s.healthStore.Get([]byte("dbVersion"))
	if err != nil {
		return false, errors.Wrap(NewDatabaseError(err), "failed to read database version")
	}

	if len(value) > 0 {
		return value[0] == DBVersion, nil
	}

	return false, nil
}

// UpdateDatabaseVersion tries to migrate the existing data to the new database version.
func (s *Storage) UpdateDatabaseVersion() (bool, error) {

	value, err := s.healthStore.Get([]byte("dbVersion"))
	if err != nil {
		return false, errors.Wrap(NewDatabaseError(err), "failed to read database version")
	}

	if len(value) < 1 {
		return false, nil
	}

	return false, nil
}
