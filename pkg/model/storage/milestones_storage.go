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
	iotago "github.com/iotaledger/iota.go/v2"
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
		Timestamp: time.Unix(int64(binary.LittleEndian.Uint64(data[iotago.MessageIDLength:iotago.MessageIDLength+iotago.UInt64ByteSize])), 0),
	}, nil
}

func (s *Storage) GetMilestoneStorageSize() int {
	return s.milestoneStorage.GetSize()
}

func (s *Storage) configureMilestoneStorage(store kvstore.KVStore, opts *profile.CacheOpts) {

	cacheTime, _ := time.ParseDuration(opts.CacheTime)
	leakDetectionMaxConsumerHoldTime, _ := time.ParseDuration(opts.LeakDetectionOptions.MaxConsumerHoldTime)

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
}

// Storable Object
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

// Cached Object
type CachedMilestone struct {
	objectstorage.CachedObject
}

type CachedMilestones []*CachedMilestone

// milestone +1
func (c CachedMilestones) Retain() CachedMilestones {
	cachedResult := make(CachedMilestones, len(c))
	for i, cachedMs := range c {
		cachedResult[i] = cachedMs.Retain()
	}
	return cachedResult
}

// milestone -1
func (c CachedMilestones) Release(force ...bool) {
	for _, cachedMs := range c {
		cachedMs.Release(force...)
	}
}

// milestone +1
func (c *CachedMilestone) Retain() *CachedMilestone {
	return &CachedMilestone{c.CachedObject.Retain()}
}

func (c *CachedMilestone) GetMilestone() *Milestone {
	return c.Get().(*Milestone)
}

// milestone +1
func (s *Storage) GetCachedMilestoneOrNil(milestoneIndex milestone.Index) *CachedMilestone {
	cachedMilestone := s.milestoneStorage.Load(databaseKeyForMilestoneIndex(milestoneIndex)) // milestone +1
	if !cachedMilestone.Exists() {
		cachedMilestone.Release(true) // milestone -1
		return nil
	}
	return &CachedMilestone{CachedObject: cachedMilestone}
}

// milestone +-0
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
	}, iteratorOptions...)
}

// milestone +1
func (s *Storage) storeMilestoneIfAbsent(index milestone.Index, messageID hornet.MessageID, timestamp time.Time) (cachedMilestone *CachedMilestone, newlyAdded bool) {

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

// +-0
func (s *Storage) DeleteMilestone(milestoneIndex milestone.Index) {
	s.milestoneStorage.Delete(databaseKeyForMilestoneIndex(milestoneIndex))
}

func (s *Storage) ShutdownMilestoneStorage() {
	s.milestoneStorage.Shutdown()
}

func (s *Storage) FlushMilestoneStorage() {
	s.milestoneStorage.Flush()
}
