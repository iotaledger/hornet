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

// the default options used for object storage iteration.
var defaultIteratorOptions = []IteratorOption{
	WithIteratorPrefix(kvstore.EmptyPrefix),
	WithIteratorMaxIterations(0),
}

// IteratorOption is a function setting an iterator option.
type IteratorOption func(opts *IteratorOptions)

// IteratorOptions define options for iterations in the object storage.
type IteratorOptions struct {
	// an optional prefix to iterate a subset of elements.
	optionalPrefix []byte
	// used to stop the iteration after a certain amount of iterations.
	maxIterations int
}

func ObjectStorageIteratorOptions(iteratorOptions ...IteratorOption) []objectstorage.IteratorOption {
	opts := IteratorOptions{}
	opts.apply(defaultIteratorOptions...)
	opts.apply(iteratorOptions...)

	return []objectstorage.IteratorOption{
		objectstorage.WithIteratorMaxIterations(opts.maxIterations),
		objectstorage.WithIteratorPrefix(opts.optionalPrefix),
	}
}

// applies the given IteratorOption.
func (o *IteratorOptions) apply(opts ...IteratorOption) {
	for _, opt := range opts {
		opt(o)
	}
}

// WithIteratorPrefix is used to iterate a subset of elements with a defined prefix.
func WithIteratorPrefix(prefix []byte) IteratorOption {
	return func(opts *IteratorOptions) {
		opts.optionalPrefix = prefix
	}
}

// WithIteratorMaxIterations is used to stop the iteration after a certain amount of iterations.
// 0 disables the limit.
func WithIteratorMaxIterations(maxIterations int) IteratorOption {
	return func(opts *IteratorOptions) {
		opts.maxIterations = maxIterations
	}
}

// NonCachedStorage is a Storage without a cache.
type NonCachedStorage struct {
	storage *Storage
}

// Storage is the access layer to the node databases (partially cached).
type Storage struct {

	// databases
	tangleStore kvstore.KVStore
	utxoStore   kvstore.KVStore

	// healthTrackers
	healthTrackers []*StoreHealthTracker

	// kv storages
	snapshotStore kvstore.KVStore

	// object storages
	childrenStorage             *objectstorage.ObjectStorage
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

func New(tangleStore kvstore.KVStore, utxoStore kvstore.KVStore, cachesProfile ...*profile.Caches) (*Storage, error) {

	s := &Storage{
		tangleStore: tangleStore,
		utxoStore:   utxoStore,
		healthTrackers: []*StoreHealthTracker{
			NewStoreHealthTracker(tangleStore),
			NewStoreHealthTracker(utxoStore),
		},
		utxoManager: utxo.New(utxoStore),
		Events: &packageEvents{
			PruningStateChanged: events.NewEvent(events.BoolCaller),
		},
	}

	if err := s.configureStorages(tangleStore, cachesProfile...); err != nil {
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

func (s *Storage) NonCachedStorage() *NonCachedStorage {
	return &NonCachedStorage{storage: s}
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

// profileLeakDetectionEnabled returns a Caches profile with caching disabled and leak detection enabled.
func (s *Storage) profileCacheEnabled() *profile.Caches {
	return &profile.Caches{
		Addresses: &profile.CacheOpts{
			CacheTime:                  "500ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		Children: &profile.CacheOpts{
			CacheTime:                  "500ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		Milestones: &profile.CacheOpts{
			CacheTime:                  "500ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		Messages: &profile.CacheOpts{
			CacheTime:                  "500ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		UnreferencedMessages: &profile.CacheOpts{
			CacheTime:                  "500ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		IncomingMessagesFilter: &profile.CacheOpts{
			CacheTime:                  "500ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
	}
}

// profileLeakDetectionEnabled returns a Caches profile with caching disabled and leak detection enabled.
func (s *Storage) profileLeakDetectionEnabled() *profile.Caches {
	return &profile.Caches{
		Addresses: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               true,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "1s",
			},
		},
		Children: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               true,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "1s",
			},
		},
		Milestones: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               true,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "1s",
			},
		},
		Messages: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               true,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "1s",
			},
		},
		UnreferencedMessages: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               true,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "1s",
			},
		},
		IncomingMessagesFilter: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               true,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "1s",
			},
		},
	}
}

func (s *Storage) configureStorages(tangleStore kvstore.KVStore, cachesProfile ...*profile.Caches) error {

	cachesOpts := s.profileCachesDisabled()
	if len(cachesProfile) > 0 {
		cachesOpts = cachesProfile[0]
	}

	if err := s.configureMessageStorage(tangleStore, cachesOpts.Messages); err != nil {
		return err
	}

	if err := s.configureChildrenStorage(tangleStore, cachesOpts.Children); err != nil {
		return err
	}

	if err := s.configureMilestoneStorage(tangleStore, cachesOpts.Milestones); err != nil {
		return err
	}

	if err := s.configureUnreferencedMessageStorage(tangleStore, cachesOpts.UnreferencedMessages); err != nil {
		return err
	}

	s.configureSnapshotStore(tangleStore)

	return nil
}

func (s *Storage) FlushAndCloseStores() error {

	var flushAndCloseError error
	if err := s.tangleStore.Flush(); err != nil {
		flushAndCloseError = err
	}
	if err := s.utxoStore.Flush(); err != nil {
		flushAndCloseError = err
	}
	if err := s.tangleStore.Close(); err != nil {
		flushAndCloseError = err
	}
	if err := s.utxoStore.Close(); err != nil {
		flushAndCloseError = err
	}
	return flushAndCloseError
}

// FlushStorages flushes all storages.
func (s *Storage) FlushStorages() {
	s.FlushMilestoneStorage()
	s.FlushMessagesStorage()
	s.FlushChildrenStorage()
	s.FlushUnreferencedMessagesStorage()
}

// ShutdownStorages shuts down all storages.
func (s *Storage) ShutdownStorages() {

	s.ShutdownMilestoneStorage()
	s.ShutdownMessagesStorage()
	s.ShutdownChildrenStorage()
	s.ShutdownUnreferencedMessagesStorage()
}
