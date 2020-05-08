package tangle

import (
	"encoding/binary"
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/database"
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

func milestoneFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return &Milestone{
		Index: milestoneIndexFromDatabaseKey(key),
	}, 4, nil
}

func GetMilestoneStorageSize() int {
	return milestoneStorage.GetSize()
}

func configureMilestoneStorage() {

	opts := profile.LoadProfile().Caches.Milestones

	milestoneStorage = objectstorage.New(
		database.StorageWithPrefix(DBPrefixMilestones),
		milestoneFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
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
	Hash  trinary.Hash
}

// ObjectStorage interface

func (ms *Milestone) Update(_ objectstorage.StorableObject) {
	panic("Milestone should never be updated")
}

func (ms *Milestone) ObjectStorageKey() []byte {
	return databaseKeyForMilestoneIndex(ms.Index)
}

func (ms *Milestone) ObjectStorageValue() (data []byte) {
	/*
		49 byte transaction hash
	*/
	value := trinary.MustTrytesToBytes(ms.Hash)[:49]

	return value
}

func (ms *Milestone) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {

	ms.Hash = trinary.MustBytesToTrytes(data, 81)
	return 49, nil
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

// SearchLatestMilestoneIndexInBadger searches the latest milestone without accessing the cache layer.
func SearchLatestMilestoneIndexInBadger() milestone.Index {
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

type MilestoneConsumer func(cachedMs objectstorage.CachedObject)

// MilestoneIndexConsumer consumes the given index during looping though all milestones in the persistence layer.
type MilestoneIndexConsumer func(index milestone.Index)

func ForEachMilestone(consumer MilestoneConsumer) {
	milestoneStorage.ForEach(func(key []byte, cachedMs objectstorage.CachedObject) bool {
		defer cachedMs.Release(true) // tx -1
		consumer(cachedMs.Retain())
		return true
	})
}

// ForEachMilestoneIndex loops though all milestones in the persistence layer.
func ForEachMilestoneIndex(consumer MilestoneIndexConsumer, skipCache bool) {
	milestoneStorage.ForEachKeyOnly(func(key []byte) bool {
		consumer(milestoneIndexFromDatabaseKey(key))
		return true
	}, skipCache)
}

// milestone +1
func StoreMilestone(bndl *Bundle) (bool, *CachedMilestone) {

	newlyAdded := false

	if bndl.IsMilestone() {

		milestone := &Milestone{
			Index: bndl.GetMilestoneIndex(),
			Hash:  bndl.GetMilestoneHash(),
		}

		cachedMilestone := milestoneStorage.ComputeIfAbsent(milestone.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject { // milestone +1
			newlyAdded = true
			milestone.Persist()
			milestone.SetModified()
			return milestone
		})

		return newlyAdded, &CachedMilestone{CachedObject: cachedMilestone}
	}

	panic("Bundle is not a milestone")
}

// +-0
func DeleteMilestone(milestoneIndex milestone.Index) {
	milestoneStorage.Delete(databaseKeyForMilestoneIndex(milestoneIndex))
}

func DeleteMilestoneFromBadger(milestoneIndex milestone.Index) {
	milestoneStorage.DeleteEntryFromBadger(databaseKeyForMilestoneIndex(milestoneIndex))
}

func ShutdownMilestoneStorage() {
	milestoneStorage.Shutdown()
}

func FlushMilestoneStorage() {
	milestoneStorage.Flush()
}
