package storage

import (
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/iotaledger/hive.go/kvstore"
)

type StoreHealthTracker struct {
	store kvstore.KVStore
}

func NewStoreHealthTracker(store kvstore.KVStore) *StoreHealthTracker {
	s := &StoreHealthTracker{
		store: store.WithRealm([]byte{common.StorePrefixHealth}),
	}
	s.setDatabaseVersion(DBVersion)
	return s
}

func (s *StoreHealthTracker) MarkCorrupted() error {

	if err := s.store.Set([]byte("dbCorrupted"), []byte{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to set database healthTrackers status")
	}
	return s.store.Flush()
}

func (s *StoreHealthTracker) MarkTainted() error {

	if err := s.store.Set([]byte("dbTainted"), []byte{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to set database healthTrackers status")
	}
	return s.store.Flush()
}

func (s *StoreHealthTracker) MarkHealthy() error {

	if err := s.store.Delete([]byte("dbCorrupted")); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to set database healthTrackers status")
	}

	return nil
}

func (s *StoreHealthTracker) IsCorrupted() (bool, error) {

	contains, err := s.store.Has([]byte("dbCorrupted"))
	if err != nil {
		return true, errors.Wrap(NewDatabaseError(err), "failed to read database healthTrackers status")
	}
	return contains, nil
}

func (s *StoreHealthTracker) IsTainted() (bool, error) {

	contains, err := s.store.Has([]byte("dbTainted"))
	if err != nil {
		return true, errors.Wrap(NewDatabaseError(err), "failed to read database healthTrackers status")
	}
	return contains, nil
}

// DatabaseVersion returns the database version.
func (s *StoreHealthTracker) DatabaseVersion() (int, error) {

	value, err := s.store.Get([]byte("dbVersion"))
	if err != nil {
		return 0, errors.Wrap(NewDatabaseError(err), "failed to read database version")
	}

	if len(value) < 1 {
		return 0, errors.Wrap(NewDatabaseError(err), "failed to read database version")
	}

	return int(value[0]), nil
}

func (s *StoreHealthTracker) setDatabaseVersion(version byte) error {

	_, err := s.store.Get([]byte("dbVersion"))
	if errors.Is(err, kvstore.ErrKeyNotFound) {
		// Only create the entry, if it doesn't exist already (fresh database)
		if err := s.store.Set([]byte("dbVersion"), []byte{version}); err != nil {
			return errors.Wrap(NewDatabaseError(err), "failed to set database version")
		}
	}
	return nil
}

func (s *StoreHealthTracker) CheckCorrectDatabaseVersion() (bool, error) {

	value, err := s.store.Get([]byte("dbVersion"))
	if err != nil {
		return false, errors.Wrap(NewDatabaseError(err), "failed to read database version")
	}

	if len(value) > 0 {
		return value[0] == DBVersion, nil
	}

	return false, nil
}

// UpdateDatabaseVersion tries to migrate the existing data to the new database version.
func (s *StoreHealthTracker) UpdateDatabaseVersion() (bool, error) {

	value, err := s.store.Get([]byte("dbVersion"))
	if err != nil {
		return false, errors.Wrap(NewDatabaseError(err), "failed to read database version")
	}

	if len(value) < 1 {
		return false, nil
	}

	return false, nil
}
