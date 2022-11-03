package storage

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/objectstorage"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/profile"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	DBVersionTangle byte = 2
	DBVersionUTXO   byte = 2
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
	*ProtocolStorage

	// databases
	tangleStore kvstore.KVStore
	utxoStore   kvstore.KVStore

	// kv storages
	protocolStore kvstore.KVStore
	snapshotStore kvstore.KVStore

	// healthTrackers
	healthTrackers []*kvstore.StoreHealthTracker

	// object storages
	childrenStorage           *objectstorage.ObjectStorage
	blocksStorage             *objectstorage.ObjectStorage
	metadataStorage           *objectstorage.ObjectStorage
	milestoneIndexStorage     *objectstorage.ObjectStorage
	milestoneStorage          *objectstorage.ObjectStorage
	unreferencedBlocksStorage *objectstorage.ObjectStorage

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

	healthTrackerTangle, err := kvstore.NewStoreHealthTracker(tangleStore, []byte{common.StorePrefixHealth}, DBVersionTangle, nil)
	if err != nil {
		return nil, err
	}

	healthTrackerUTXO, err := kvstore.NewStoreHealthTracker(utxoStore, []byte{common.StorePrefixHealth}, DBVersionUTXO, nil)
	if err != nil {
		return nil, err
	}

	s := &Storage{
		ProtocolStorage: nil,
		tangleStore:     tangleStore,
		utxoStore:       utxoStore,
		healthTrackers: []*kvstore.StoreHealthTracker{
			healthTrackerTangle,
			healthTrackerUTXO,
		},
		utxoManager: utxo.New(utxoStore),
		Events: &packageEvents{
			PruningStateChanged: events.NewEvent(events.BoolCaller),
		},
	}

	if err := s.configureStorages(tangleStore, cachesProfile...); err != nil {
		return nil, err
	}

	s.ProtocolStorage = NewProtocolStorage(s.protocolStore)

	if err := s.loadSnapshotInfo(); err != nil {
		return nil, err
	}

	if err := s.loadSolidEntryPoints(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Storage) TangleStore() kvstore.KVStore {
	return s.tangleStore
}

func (s *Storage) UTXOStore() kvstore.KVStore {
	return s.utxoStore
}

func (s *Storage) NonCachedStorage() *NonCachedStorage {
	return &NonCachedStorage{storage: s}
}

func (s *Storage) UTXOManager() *utxo.Manager {
	return s.utxoManager
}

func (s *Storage) SolidEntryPoints() *SolidEntryPoints {
	return s.solidEntryPoints
}

// profileCachesDisabled returns a Caches profile with caching disabled.
//
//lint:ignore U1000 used for easier debugging
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
		Blocks: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 10,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		UnreferencedBlocks: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 10,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		IncomingBlocksFilter: &profile.CacheOpts{
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
//
//lint:ignore U1000 used for easier debugging
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
		Blocks: &profile.CacheOpts{
			CacheTime:                  "500ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		UnreferencedBlocks: &profile.CacheOpts{
			CacheTime:                  "500ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               false,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "0ms",
			},
		},
		IncomingBlocksFilter: &profile.CacheOpts{
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
//
//lint:ignore U1000 used for easier debugging
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
		Blocks: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               true,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "1s",
			},
		},
		UnreferencedBlocks: &profile.CacheOpts{
			CacheTime:                  "0ms",
			ReleaseExecutorWorkerCount: 1,
			LeakDetectionOptions: &profile.LeakDetectionOpts{
				Enabled:               true,
				MaxConsumersPerObject: 10,
				MaxConsumerHoldTime:   "1s",
			},
		},
		IncomingBlocksFilter: &profile.CacheOpts{
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

	if err := s.configureBlockStorage(tangleStore, cachesOpts.Blocks); err != nil {
		return err
	}

	if err := s.configureChildrenStorage(tangleStore, cachesOpts.Children); err != nil {
		return err
	}

	if err := s.configureMilestoneStorage(tangleStore, cachesOpts.Milestones); err != nil {
		return err
	}

	if err := s.configureUnreferencedBlocksStorage(tangleStore, cachesOpts.UnreferencedBlocks); err != nil {
		return err
	}

	if err := s.configureSnapshotStore(tangleStore); err != nil {
		return err
	}

	if err := s.configureProtocolStore(tangleStore); err != nil {
		return err
	}

	return nil
}

func (s *Storage) FlushAndCloseStores() error {

	var flushAndCloseError error
	if err := s.snapshotStore.Flush(); err != nil {
		flushAndCloseError = err
	}
	if err := s.protocolStore.Flush(); err != nil {
		flushAndCloseError = err
	}
	if err := s.tangleStore.Flush(); err != nil {
		flushAndCloseError = err
	}
	if err := s.utxoStore.Flush(); err != nil {
		flushAndCloseError = err
	}

	if err := s.snapshotStore.Close(); err != nil {
		flushAndCloseError = err
	}
	if err := s.protocolStore.Close(); err != nil {
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
	s.FlushBlocksStorage()
	s.FlushChildrenStorage()
	s.FlushUnreferencedBlocksStorage()
}

// ShutdownStorages shuts down all storages.
func (s *Storage) ShutdownStorages() {
	s.ShutdownMilestoneStorage()
	s.ShutdownBlocksStorage()
	s.ShutdownChildrenStorage()
	s.ShutdownUnreferencedBlocksStorage()
}

// Shutdown flushes and closes all object storages,
// and then flushes and closes all stores.
func (s *Storage) Shutdown() error {
	s.FlushStorages()
	s.ShutdownStorages()

	return s.FlushAndCloseStores()
}

func (s *Storage) CurrentProtocolParameters() (*iotago.ProtocolParameters, error) {
	ledgerIndex, err := s.UTXOManager().ReadLedgerIndex()
	if err != nil {
		return nil, fmt.Errorf("loading current protocol parameters failed: %w", err)
	}

	return s.ProtocolParameters(ledgerIndex)
}

// CheckLedgerState checks if the total balance of the ledger fits the token supply in the protocol parameters.
func (s *Storage) CheckLedgerState() error {

	protoParams, err := s.CurrentProtocolParameters()
	if err != nil {
		return err
	}

	if err = s.UTXOManager().CheckLedgerState(protoParams.TokenSupply); err != nil {
		return err
	}

	return nil
}
