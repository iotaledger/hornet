package storage

import (
	"sync"

	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"
)

type packageEvents struct {
	PruningStateChanged *events.Event
}

type ReadOption = objectstorage.ReadOption
type IteratorOption = objectstorage.IteratorOption

type Storage struct {
	// database
	store kvstore.KVStore

	// kv storages
	healthStore   kvstore.KVStore
	snapshotStore kvstore.KVStore

	// object storages
	childrenStorage             *objectstorage.ObjectStorage
	indexationStorage           *objectstorage.ObjectStorage
	messagesStorage             *objectstorage.ObjectStorage
	metadataStorage             *objectstorage.ObjectStorage
	milestoneStorage            *objectstorage.ObjectStorage
	unreferencedMessagesStorage *objectstorage.ObjectStorage

	// solid entry points
	solidEntryPoints     *SolidEntryPoints
	solidEntryPointsLock sync.RWMutex

	// snapshot info
	snapshot      *SnapshotInfo
	snapshotMutex syncutils.RWMutex

	// utxo
	utxoManager *utxo.Manager

	// events
	Events *packageEvents
}

func New(store kvstore.KVStore, cachesProfile ...*profile.Caches) (*Storage, error) {

	utxoManager := utxo.New(store)

	s := &Storage{
		store:       store,
		utxoManager: utxoManager,
		Events: &packageEvents{
			PruningStateChanged: events.NewEvent(events.BoolCaller),
		},
	}

	if err := s.configureStorages(s.store, cachesProfile...); err != nil {
		return nil, err
	}

	if err := s.loadSnapshotInfo(); err != nil {
		return nil, err
	}

	if err := s.loadSolidEntryPoints(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Storage) KVStore() kvstore.KVStore {
	return s.store
}

func (s *Storage) UTXOManager() *utxo.Manager {
	return s.utxoManager
}

// profileCachesDisabled returns a Caches profile with caching disabled.
func (s *Storage) profileCachesDisabled() *profile.Caches {
	return &profile.Caches{
		Addresses: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 10,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		Children: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 10,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		Indexations: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 10,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		Milestones: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 10,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		Messages: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 10,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		UnreferencedMessages: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 10,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		IncomingMessagesFilter: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 10,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
	}
}

func (s *Storage) configureStorages(store kvstore.KVStore, cachesProfile ...*profile.Caches) error {

	if err := s.configureHealthStore(store); err != nil {
		return err
	}

	cachesOpts := s.profileCachesDisabled()
	if len(cachesProfile) > 0 {
		cachesOpts = cachesProfile[0]
	}

	if err := s.configureMessageStorage(store, cachesOpts.Messages); err != nil {
		return err
	}

	if err := s.configureChildrenStorage(store, cachesOpts.Children); err != nil {
		return err
	}

	if err := s.configureMilestoneStorage(store, cachesOpts.Milestones); err != nil {
		return err
	}

	if err := s.configureUnreferencedMessageStorage(store, cachesOpts.UnreferencedMessages); err != nil {
		return err
	}

	if err := s.configureIndexationStorage(store, cachesOpts.Indexations); err != nil {
		return err
	}

	s.configureSnapshotStore(store)

	return nil
}

// FlushStorages flushes all storages.
func (s *Storage) FlushStorages() {
	s.FlushMilestoneStorage()
	s.FlushMessagesStorage()
	s.FlushChildrenStorage()
	s.FlushIndexationStorage()
	s.FlushUnreferencedMessagesStorage()
}

// ShutdownStorages shuts down all storages.
func (s *Storage) ShutdownStorages() {

	s.ShutdownMilestoneStorage()
	s.ShutdownMessagesStorage()
	s.ShutdownChildrenStorage()
	s.ShutdownIndexationStorage()
	s.ShutdownUnreferencedMessagesStorage()
}
