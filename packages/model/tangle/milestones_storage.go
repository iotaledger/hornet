package tangle

import (
	"encoding/binary"
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/profile"
)

var (
	milestoneStorage *objectstorage.ObjectStorage
)

func databaseKeyForMilestoneIndex(milestoneIndex milestone_index.MilestoneIndex) []byte {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(milestoneIndex))
	return bytes
}

func milestoneIndexFromDatabaseKey(key []byte) milestone_index.MilestoneIndex {
	return milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(key))
}

func milestoneFactory(key []byte) (objectstorage.StorableObject, error) {
	return &Milestone{
		Index: milestoneIndexFromDatabaseKey(key),
	}, nil
}

func GetMilestoneStorageSize() int {
	return milestoneStorage.GetSize()
}

func configureMilestoneStorage() {

	opts := profile.GetProfile().Caches.Milestones

	milestoneStorage = objectstorage.New(
		database.GetHornetBadgerInstance(),
		[]byte{DBPrefixMilestones},
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

	Index milestone_index.MilestoneIndex
	Hash  trinary.Hash
}

// ObjectStorage interface

func (ms *Milestone) Update(other objectstorage.StorableObject) {
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

func (ms *Milestone) UnmarshalObjectStorageValue(data []byte) error {

	ms.Hash = trinary.MustBytesToTrytes(data, 81)
	return nil
}

// Cached Object
type CachedMilestone struct {
	objectstorage.CachedObject
}

func (c *CachedMilestone) GetMilestone() *Milestone {
	return c.Get().(*Milestone)
}

// milestone +1
func GetCachedMilestoneOrNil(milestoneIndex milestone_index.MilestoneIndex) *CachedMilestone {
	cachedMilestone := milestoneStorage.Load(databaseKeyForMilestoneIndex(milestoneIndex)) // milestone +1
	if !cachedMilestone.Exists() {
		cachedMilestone.Release(true) // milestone -1
		return nil
	}
	return &CachedMilestone{CachedObject: cachedMilestone}
}

// milestone +-0
func ContainsMilestone(milestoneIndex milestone_index.MilestoneIndex) bool {
	return milestoneStorage.Contains(databaseKeyForMilestoneIndex(milestoneIndex))
}

// milestone +-0
func SearchLatestMilestoneIndex() milestone_index.MilestoneIndex {
	var latestMilestoneIndex milestone_index.MilestoneIndex

	milestoneStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedObject.Release(true) // milestone -1

		msIndex := milestoneIndexFromDatabaseKey(key)
		if latestMilestoneIndex < msIndex {
			latestMilestoneIndex = msIndex
		}

		return true
	})

	return latestMilestoneIndex
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
func DeleteMilestone(milestoneIndex milestone_index.MilestoneIndex) {
	milestoneStorage.Delete(databaseKeyForMilestoneIndex(milestoneIndex))
}

func ShutdownMilestoneStorage() {
	milestoneStorage.Shutdown()
}
