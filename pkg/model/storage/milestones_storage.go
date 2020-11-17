package storage

import (
	"encoding/binary"
	"fmt"
	"time"

	iotago "github.com/iotaledger/iota.go"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/profile"
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
		MessageID: hornet.MessageIDFromBytes(data[:iotago.MessageIDLength]),
		Timestamp: time.Unix(int64(binary.LittleEndian.Uint64(data[iotago.MessageIDLength:iotago.MessageIDLength+iotago.UInt64ByteSize])), 0),
	}, nil
}

func (s *Storage) GetMilestoneStorageSize() int {
	return s.milestoneStorage.GetSize()
}

func (s *Storage) configureMilestoneStorage(store kvstore.KVStore, opts *profile.CacheOpts) {

	s.milestoneStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixMilestones}),
		milestoneFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// Storable Object
type Milestone struct {
	objectstorage.StorableObjectFlags

	Index     milestone.Index
	MessageID *hornet.MessageID
	Timestamp time.Time
}

// ObjectStorage interface

func (ms *Milestone) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("Milestone should never be updated: %v (%d)", ms.MessageID.Hex(), ms.Index))
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

	return byteutils.ConcatBytes(ms.MessageID.Slice(), value)
}

// Cached Object
type CachedMilestone struct {
	objectstorage.CachedObject
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
func (s *Storage) ContainsMilestone(milestoneIndex milestone.Index) bool {
	return s.milestoneStorage.Contains(databaseKeyForMilestoneIndex(milestoneIndex))
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
	}, true)

	return latestMilestoneIndex
}

// MilestoneIndexConsumer consumes the given index during looping through all milestones in the persistence layer.
type MilestoneIndexConsumer func(index milestone.Index) bool

// ForEachMilestoneIndex loops through all milestones in the persistence layer.
func (s *Storage) ForEachMilestoneIndex(consumer MilestoneIndexConsumer, skipCache bool) {
	s.milestoneStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(milestoneIndexFromDatabaseKey(key))
	}, skipCache)
}

// milestone +1
func (s *Storage) storeMilestone(index milestone.Index, messageID *hornet.MessageID, timestamp time.Time) *CachedMilestone {
	milestone := &Milestone{
		Index:       index,
		MessageID:   messageID,
		Timestamp:   timestamp,
	}

	// milestones should never exist in the database already, even with an unclean database
	return &CachedMilestone{CachedObject: s.milestoneStorage.Store(milestone)}
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
