package tangle

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/profile"
)

var (
	milestoneStorage *objectstorage.ObjectStorage
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
		Index: milestoneIndexFromDatabaseKey(key),
		Hash:  hornet.Hash(data[:49]),
	}, nil
}

func GetMilestoneStorageSize() int {
	return milestoneStorage.GetSize()
}

func configureMilestoneStorage(store kvstore.KVStore, opts profile.CacheOpts) {

	milestoneStorage = objectstorage.New(
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

	Index milestone.Index
	Hash  hornet.Hash
}

// ObjectStorage interface

func (ms *Milestone) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("Milestone should never be updated: %v (%d)", ms.Hash.Trytes(), ms.Index))
}

func (ms *Milestone) ObjectStorageKey() []byte {
	return databaseKeyForMilestoneIndex(ms.Index)
}

func (ms *Milestone) ObjectStorageValue() (data []byte) {
	/*
		49 byte transaction hash
	*/
	return ms.Hash
}

// Cached Object
type CachedMilestone struct {
	objectstorage.CachedObject
}

func (c *CachedMilestone) GetMilestone() *Milestone {
	return c.Get().(*Milestone)
}

// milestone +1
func GetCachedMilestoneOrNil(milestoneIndex milestone.Index) *CachedMilestone {
	cachedMilestone := milestoneStorage.Load(databaseKeyForMilestoneIndex(milestoneIndex)) // milestone +1
	if !cachedMilestone.Exists() {
		cachedMilestone.Release(true) // milestone -1
		return nil
	}
	return &CachedMilestone{CachedObject: cachedMilestone}
}

// milestone +-0
func ContainsMilestone(milestoneIndex milestone.Index) bool {
	return milestoneStorage.Contains(databaseKeyForMilestoneIndex(milestoneIndex))
}

// SearchLatestMilestoneIndexInStore searches the latest milestone without accessing the cache layer.
func SearchLatestMilestoneIndexInStore() milestone.Index {
	var latestMilestoneIndex milestone.Index

	milestoneStorage.ForEachKeyOnly(func(key []byte) bool {
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
func ForEachMilestoneIndex(consumer MilestoneIndexConsumer, skipCache bool) {
	milestoneStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(milestoneIndexFromDatabaseKey(key))
	}, skipCache)
}

// milestone +1
func StoreMilestone(bndl *Bundle) *CachedMilestone {

	if bndl.IsMilestone() {

		milestone := &Milestone{
			Index: bndl.GetMilestoneIndex(),
			Hash:  bndl.GetMilestoneHash(),
		}

		// milestones should never exist in the database already, even with an unclean database
		return &CachedMilestone{CachedObject: milestoneStorage.Store(milestone)}
	}

	panic("Bundle is not a milestone")
}

// +-0
func DeleteMilestone(milestoneIndex milestone.Index) {
	milestoneStorage.Delete(databaseKeyForMilestoneIndex(milestoneIndex))
}

func ShutdownMilestoneStorage() {
	milestoneStorage.Shutdown()
}

func FlushMilestoneStorage() {
	milestoneStorage.Flush()
}
