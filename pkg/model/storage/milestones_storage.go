package storage

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

func databaseKeyForMilestoneIndex(milestoneIndex milestone.Index) []byte {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(milestoneIndex))
	return bytes
}

func milestoneIndexFromDatabaseKey(key []byte) milestone.Index {
	return milestone.Index(binary.LittleEndian.Uint32(key))
}

func milestoneFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	return &Milestone{
		Index:     milestoneIndexFromDatabaseKey(key),
		MessageID: hornet.MessageIDFromSlice(data[:iotago.MessageIDLength]),
		Timestamp: time.Unix(int64(binary.LittleEndian.Uint64(data[iotago.MessageIDLength:iotago.MessageIDLength+serializer.UInt64ByteSize])), 0),
	}, nil
}

func (s *Storage) MilestoneStorageSize() int {
	return s.milestoneStorage.GetSize()
}

func (s *Storage) configureMilestoneStorage(store kvstore.KVStore, opts *profile.CacheOpts) error {

	cacheTime, err := time.ParseDuration(opts.CacheTime)
	if err != nil {
		return err
	}

	leakDetectionMaxConsumerHoldTime, err := time.ParseDuration(opts.LeakDetectionOptions.MaxConsumerHoldTime)
	if err != nil {
		return err
	}

	s.milestoneStorage = objectstorage.New(
		store.WithRealm([]byte{common.StorePrefixMilestones}),
		milestoneFactory,
		objectstorage.CacheTime(cacheTime),
		objectstorage.PersistenceEnabled(true),
		objectstorage.ReleaseExecutorWorkerCount(opts.ReleaseExecutorWorkerCount),
		objectstorage.StoreOnCreation(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   leakDetectionMaxConsumerHoldTime,
			}),
	)

	return nil
}

type Milestone struct {
	objectstorage.StorableObjectFlags

	Index     milestone.Index
	MessageID hornet.MessageID
	Timestamp time.Time
}

// ObjectStorage interface

func (ms *Milestone) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("Milestone should never be updated: %v (%d)", ms.MessageID.ToHex(), ms.Index))
}

func (ms *Milestone) ObjectStorageKey() []byte {
	return databaseKeyForMilestoneIndex(ms.Index)
}

func (ms *Milestone) ObjectStorageValue() (data []byte) {
	/*
		32 byte message ID
		8  byte timestamp
	*/

	value := make([]byte, 8)
	binary.LittleEndian.PutUint64(value, uint64(ms.Timestamp.Unix()))

	return byteutils.ConcatBytes(ms.MessageID, value)
}

// CachedMilestone represents a cached milestone.
type CachedMilestone struct {
	objectstorage.CachedObject
}

type CachedMilestones []*CachedMilestone

// Retain registers a new consumer for the cached milestones.
// milestone +1
func (c CachedMilestones) Retain() CachedMilestones {
	cachedResult := make(CachedMilestones, len(c))
	for i, cachedMs := range c {
		cachedResult[i] = cachedMs.Retain()
	}
	return cachedResult
}

// Release releases the cached milestones, to be picked up by the persistence layer (as soon as all consumers are done).
// milestone -1
func (c CachedMilestones) Release(force ...bool) {
	for _, cachedMs := range c {
		cachedMs.Release(force...)
	}
}

// Retain registers a new consumer for the cached milestone.
// milestone +1
func (c *CachedMilestone) Retain() *CachedMilestone {
	return &CachedMilestone{c.CachedObject.Retain()}
}

// Milestone retrieves the milestone, that is cached in this container.
func (c *CachedMilestone) Milestone() *Milestone {
	return c.Get().(*Milestone)
}

// CachedMilestoneOrNil returns a cached milestone object.
// milestone +1
func (s *Storage) CachedMilestoneOrNil(milestoneIndex milestone.Index) *CachedMilestone {
	cachedMilestone := s.milestoneStorage.Load(databaseKeyForMilestoneIndex(milestoneIndex)) // milestone +1
	if !cachedMilestone.Exists() {
		cachedMilestone.Release(true) // milestone -1
		return nil
	}
	return &CachedMilestone{CachedObject: cachedMilestone}
}

// ContainsMilestone returns if the given milestone exists in the cache/persistence layer.
func (s *Storage) ContainsMilestone(milestoneIndex milestone.Index, readOptions ...ReadOption) bool {
	return s.milestoneStorage.Contains(databaseKeyForMilestoneIndex(milestoneIndex), readOptions...)
}

// SearchLatestMilestoneIndexInStore searches the latest milestone without accessing the cache layer.
func (s *Storage) SearchLatestMilestoneIndexInStore() milestone.Index {
	var latestMilestoneIndex milestone.Index

	s.milestoneStorage.ForEachKeyOnly(func(key []byte) bool {
		msIndex := milestoneIndexFromDatabaseKey(key)
		if latestMilestoneIndex < msIndex {
			latestMilestoneIndex = msIndex
		}

		return true
	}, objectstorage.WithIteratorSkipCache(true))

	return latestMilestoneIndex
}

// MilestoneIndexConsumer consumes the given index during looping through all milestones.
type MilestoneIndexConsumer func(index milestone.Index) bool

// ForEachMilestoneIndex loops through all milestones.
func (s *Storage) ForEachMilestoneIndex(consumer MilestoneIndexConsumer, iteratorOptions ...IteratorOption) {

	s.milestoneStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(milestoneIndexFromDatabaseKey(key))
	}, ObjectStorageIteratorOptions(iteratorOptions...)...)
}

// ForEachMilestoneIndex loops through all milestones.
func (ns *NonCachedStorage) ForEachMilestoneIndex(consumer MilestoneIndexConsumer, iteratorOptions ...IteratorOption) {

	ns.storage.milestoneStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(milestoneIndexFromDatabaseKey(key))
	}, append(ObjectStorageIteratorOptions(iteratorOptions...), objectstorage.WithIteratorSkipCache(true))...)
}

// milestone +1
func (s *Storage) StoreMilestoneIfAbsent(index milestone.Index, messageID hornet.MessageID, timestamp time.Time) (cachedMilestone *CachedMilestone, newlyAdded bool) {

	cachedMs, newlyAdded := s.milestoneStorage.StoreIfAbsent(&Milestone{
		Index:     index,
		MessageID: messageID,
		Timestamp: timestamp,
	})
	if !newlyAdded {
		return nil, false
	}

	return &CachedMilestone{CachedObject: cachedMs}, newlyAdded
}

// DeleteMilestone deletes the milestone in the cache/persistence layer.
// +-0
func (s *Storage) DeleteMilestone(milestoneIndex milestone.Index) {
	s.milestoneStorage.Delete(databaseKeyForMilestoneIndex(milestoneIndex))
}

// ShutdownMilestoneStorage shuts down milestones storage.
func (s *Storage) ShutdownMilestoneStorage() {
	s.milestoneStorage.Shutdown()
}

// FlushMilestoneStorage flushes the milestones storage.
func (s *Storage) FlushMilestoneStorage() {
	s.milestoneStorage.Flush()
}
