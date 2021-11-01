package storage

import (
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/iotaledger/hive.go/kvstore"
)

type storeHealthTracker struct {
	store kvstore.KVStore
}

func newStoreHealthTracker(store kvstore.KVStore) *storeHealthTracker {
	s := &storeHealthTracker{
		store: store.WithRealm([]byte{common.StorePrefixHealth}),
	}
	s.setDatabaseVersion(DBVersion)
	return s
}

func (s *storeHealthTracker) markCorrupted() error {

	if err := s.store.Set([]byte("dbCorrupted"), []byte{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to set database healthTrackers status")
	}
	return s.store.Flush()
}

func (s *storeHealthTracker) markTainted() error {

	if err := s.store.Set([]byte("dbTainted"), []byte{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to set database healthTrackers status")
	}
	return s.store.Flush()
}

func (s *storeHealthTracker) markHealthy() error {

	if err := s.store.Delete([]byte("dbCorrupted")); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to set database healthTrackers status")
	}

	return nil
}

func (s *storeHealthTracker) isCorrupted() (bool, error) {

	contains, err := s.store.Has([]byte("dbCorrupted"))
	if err != nil {
		return true, errors.Wrap(NewDatabaseError(err), "failed to read database healthTrackers status")
	}
	return contains, nil
}

func (s *storeHealthTracker) isTainted() (bool, error) {

	contains, err := s.store.Has([]byte("dbTainted"))
	if err != nil {
		return true, errors.Wrap(NewDatabaseError(err), "failed to read database healthTrackers status")
	}
	return contains, nil
}

func (s *storeHealthTracker) setDatabaseVersion(version byte) error {

	_, err := s.store.Get([]byte("dbVersion"))
	if errors.Is(err, kvstore.ErrKeyNotFound) {
		// Only create the entry, if it doesn't exist already (fresh database)
		if err := s.store.Set([]byte("dbVersion"), []byte{version}); err != nil {
			return errors.Wrap(NewDatabaseError(err), "failed to set database version")
		}
	}
	return nil
}

func (s *storeHealthTracker) checkCorrectDatabaseVersion(expectedVersion byte) (bool, error) {

	value, err := s.store.Get([]byte("dbVersion"))
	if err != nil {
		return false, errors.Wrap(NewDatabaseError(err), "failed to read database version")
	}

	if len(value) > 0 {
		return value[0] == expectedVersion, nil
	}

	return false, nil
}

// UpdateDatabaseVersion tries to migrate the existing data to the new database version.
func (s *storeHealthTracker) updateDatabaseVersion() (bool, error) {

	value, err := s.store.Get([]byte("dbVersion"))
	if err != nil {
		return false, errors.Wrap(NewDatabaseError(err), "failed to read database version")
	}

	if len(value) < 1 {
		return false, nil
	}

	return false, nil
}
