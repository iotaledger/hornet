package tangle

import (
	"encoding/binary"
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/iota.go/trinary"

	hornetDB "github.com/gohornet/hornet/packages/database"
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

func milestoneFactory(key []byte) objectstorage.StorableObject {
	return &Milestone{
		Index: milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(key)),
	}
}

func GetMilestoneStorageSize() int {
	return milestoneStorage.GetSize()
}

func configureMilestoneStorage() {

	opts := profile.GetProfile().Caches.Milestones

	milestoneStorage = objectstorage.New(
		hornetDB.GetHornetBadgerInstance(),
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
	if obj, ok := other.(*Milestone); !ok {
		panic("invalid object passed to Milestone.Update()")
	} else {
		ms.Index = obj.Index
		ms.Hash = obj.Hash
	}
}

func (ms *Milestone) GetStorageKey() []byte {
	return databaseKeyForMilestoneIndex(ms.Index)
}

func (ms *Milestone) MarshalBinary() (data []byte, err error) {
	/*
		49 byte transaction hash
	*/
	value := trinary.MustTrytesToBytes(ms.Hash)[:49]

	return value, nil
}

func (ms *Milestone) UnmarshalBinary(data []byte) error {

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
func GetCachedMilestone(milestoneIndex milestone_index.MilestoneIndex) *CachedMilestone {
	return &CachedMilestone{milestoneStorage.Load(databaseKeyForMilestoneIndex(milestoneIndex))}
}

// milestone +-0
func ContainsMilestone(milestoneIndex milestone_index.MilestoneIndex) bool {
	return milestoneStorage.Contains(databaseKeyForMilestoneIndex(milestoneIndex))
}

// milestone +1
func StoreMilestone(cachedBndl *CachedBundle) *CachedMilestone {
	defer cachedBndl.Release() // bundle -1

	if cachedBndl.GetBundle().IsMilestone() {
		return &CachedMilestone{milestoneStorage.Store(&Milestone{
			Index: cachedBndl.GetBundle().GetMilestoneIndex(),
			Hash:  cachedBndl.GetBundle().GetMilestoneHash()})}
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
