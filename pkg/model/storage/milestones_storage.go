package storage

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/marshalutil"
	"github.com/iotaledger/hive.go/core/objectstorage"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/profile"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	ErrMilestoneNotFound = errors.New("milestone not found")
)

func databaseKeyForMilestoneIndex(milestoneIndex iotago.MilestoneIndex) []byte {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, milestoneIndex)

	return bytes
}

func databaseKeyForMilestone(milestoneID iotago.MilestoneID) []byte {
	bytes := make([]byte, iotago.MilestoneIDLength)
	copy(bytes, milestoneID[:])

	return bytes
}

func milestoneIndexFromDatabaseKey(key []byte) iotago.MilestoneIndex {
	return binary.LittleEndian.Uint32(key)
}

func milestoneIDFromDatabaseKey(key []byte) iotago.MilestoneID {
	milestoneID := iotago.MilestoneID{}
	copy(milestoneID[:], key)

	return milestoneID
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

	milestoneIndexesStore, err := store.WithRealm([]byte{common.StorePrefixMilestoneIndexes})
	if err != nil {
		return err
	}

	milestoneIndexFactory := func(key []byte, data []byte) (objectstorage.StorableObject, error) {

		marshalUtil := marshalutil.New(data)

		msID, err := marshalUtil.ReadBytes(iotago.MilestoneIDLength)
		if err != nil {
			return nil, err
		}
		milestoneID := iotago.MilestoneID{}
		copy(milestoneID[:], msID)

		return &MilestoneIndex{
			Index:       milestoneIndexFromDatabaseKey(key),
			milestoneID: milestoneID,
		}, nil
	}

	s.milestoneIndexStorage = objectstorage.New(
		milestoneIndexesStore,
		milestoneIndexFactory,
		objectstorage.CacheTime(cacheTime),
		objectstorage.PersistenceEnabled(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.ReleaseExecutorWorkerCount(opts.ReleaseExecutorWorkerCount),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   leakDetectionMaxConsumerHoldTime,
			}),
	)

	milestonesStore, err := store.WithRealm([]byte{common.StorePrefixMilestones})
	if err != nil {
		return err
	}

	milestoneFactory := func(key []byte, data []byte) (objectstorage.StorableObject, error) {
		ms := &Milestone{
			milestoneID: milestoneIDFromDatabaseKey(key[:iotago.MilestoneIDLength]),
			data:        data,
		}

		return ms, nil
	}

	s.milestoneStorage = objectstorage.New(
		milestonesStore,
		milestoneFactory,
		objectstorage.CacheTime(cacheTime),
		objectstorage.PersistenceEnabled(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.ReleaseExecutorWorkerCount(opts.ReleaseExecutorWorkerCount),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   leakDetectionMaxConsumerHoldTime,
			}),
	)

	return nil
}

type MilestoneIndex struct {
	objectstorage.StorableObjectFlags

	Index       iotago.MilestoneIndex
	milestoneID iotago.MilestoneID
}

func NewMilestoneIndex(milestonePayload *iotago.Milestone, milestoneID ...iotago.MilestoneID) (*MilestoneIndex, error) {

	var msID iotago.MilestoneID
	if len(milestoneID) > 0 {
		msID = milestoneID[0]
	} else {
		var err error
		msID, err = milestonePayload.ID()
		if err != nil {
			return nil, err
		}
	}

	msIndex := &MilestoneIndex{
		Index:       milestonePayload.Index,
		milestoneID: msID,
	}

	return msIndex, nil
}

func (ms *MilestoneIndex) MilestoneID() iotago.MilestoneID {
	return ms.milestoneID
}

// ObjectStorage interface

func (ms *MilestoneIndex) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("MilestoneIndex should never be updated: %v (%d)", iotago.EncodeHex(ms.milestoneID[:]), ms.Index))
}

func (ms *MilestoneIndex) ObjectStorageKey() []byte {
	return databaseKeyForMilestoneIndex(ms.Index)
}

func (ms *MilestoneIndex) ObjectStorageValue() (data []byte) {
	/*
		32 byte milestone ID
	*/

	return marshalutil.New(32).
		WriteBytes(ms.milestoneID[:]).
		Bytes()
}

// cachedMilestoneIndex represents a cached milestone index lookup.
type cachedMilestoneIndex struct {
	objectstorage.CachedObject
}

// Retain registers a new consumer for the cached milestone index lookup.
// milestone index +1.
func (c *cachedMilestoneIndex) Retain() *cachedMilestoneIndex {
	return &cachedMilestoneIndex{c.CachedObject.Retain()} // milestone index +1
}

// MilestoneIndex retrieves the milestone index, that is cached in this container.
func (c *cachedMilestoneIndex) MilestoneIndex() *MilestoneIndex {
	//nolint:forcetypeassert // we will replace that with generics anyway
	return c.Get().(*MilestoneIndex)
}

// cachedMilestoneIndexOrNil returns a cached milestone index object.
// milestone index +1.
func (s *Storage) cachedMilestoneIndexOrNil(milestoneIndex iotago.MilestoneIndex) *cachedMilestoneIndex {
	cachedMilestoneIdx := s.milestoneIndexStorage.Load(databaseKeyForMilestoneIndex(milestoneIndex)) // milestone index +1
	if !cachedMilestoneIdx.Exists() {
		cachedMilestoneIdx.Release(true) // milestone index -1

		return nil
	}

	return &cachedMilestoneIndex{CachedObject: cachedMilestoneIdx}
}

// ContainsMilestoneIndex returns if the given milestone exists in the cache/persistence layer.
func (s *Storage) ContainsMilestoneIndex(milestoneIndex iotago.MilestoneIndex, readOptions ...ReadOption) bool {
	return s.milestoneIndexStorage.Contains(databaseKeyForMilestoneIndex(milestoneIndex), readOptions...)
}

// MilestoneIndexConsumer consumes the given index during looping through all milestones.
// Returning false from this function indicates to abort the iteration.
type MilestoneIndexConsumer func(index iotago.MilestoneIndex) bool

// ForEachMilestoneIndex loops through all milestones.
func (s *Storage) ForEachMilestoneIndex(consumer MilestoneIndexConsumer, iteratorOptions ...IteratorOption) {

	s.milestoneIndexStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(milestoneIndexFromDatabaseKey(key))
	}, ObjectStorageIteratorOptions(iteratorOptions...)...)
}

// ForEachMilestoneIndex loops through all milestones.
func (ns *NonCachedStorage) ForEachMilestoneIndex(consumer MilestoneIndexConsumer, iteratorOptions ...IteratorOption) {

	ns.storage.milestoneIndexStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(milestoneIndexFromDatabaseKey(key))
	}, append(ObjectStorageIteratorOptions(iteratorOptions...), objectstorage.WithIteratorSkipCache(true))...)
}

type Milestone struct {
	objectstorage.StorableObjectFlags

	// Key
	milestoneID iotago.MilestoneID

	// Value
	data []byte

	milestoneOnce sync.Once
	payload       *iotago.Milestone
}

func NewMilestone(milestonePayload *iotago.Milestone, deSeriMode serializer.DeSerializationMode, milestoneID ...iotago.MilestoneID) (*Milestone, error) {

	data, err := milestonePayload.Serialize(deSeriMode, nil)
	if err != nil {
		return nil, err
	}

	var msID iotago.MilestoneID
	if len(milestoneID) > 0 {
		msID = milestoneID[0]
	} else {
		var err error
		msID, err = milestonePayload.ID()
		if err != nil {
			return nil, err
		}
	}

	ms := &Milestone{
		milestoneID: msID,
		data:        data,
	}

	ms.milestoneOnce.Do(func() {
		ms.payload = milestonePayload
	})

	return ms, nil
}

func (ms *Milestone) MilestoneID() iotago.MilestoneID {
	return ms.milestoneID
}

func (ms *Milestone) MilestoneIDHex() string {
	return iotago.EncodeHex(ms.milestoneID[:])
}

func (ms *Milestone) Data() []byte {
	return ms.data
}

func (ms *Milestone) Index() iotago.MilestoneIndex {
	return ms.Milestone().Index
}

func (ms *Milestone) Parents() iotago.BlockIDs {
	return ms.Milestone().Parents
}

func (ms *Milestone) Timestamp() time.Time {
	return time.Unix(int64(ms.TimestampUnix()), 0)
}

func (ms *Milestone) TimestampUnix() uint32 {
	return ms.Milestone().Timestamp
}

func (ms *Milestone) Milestone() *iotago.Milestone {
	ms.milestoneOnce.Do(func() {
		milestonePayload := &iotago.Milestone{}
		// No need to verify the milestone again here
		if _, err := milestonePayload.Deserialize(ms.data, serializer.DeSeriModeNoValidation, nil); err != nil {
			panic(fmt.Sprintf("failed to deserialize milestone: %v, error: %s", iotago.EncodeHex(ms.milestoneID[:]), err))
		}

		ms.payload = milestonePayload
	})

	return ms.payload
}

// ObjectStorage interface

func (ms *Milestone) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("Milestone should never be updated: %v", iotago.EncodeHex(ms.milestoneID[:])))
}

func (ms *Milestone) ObjectStorageKey() []byte {
	return ms.milestoneID[:]
}

func (ms *Milestone) ObjectStorageValue() []byte {
	return ms.data
}

// CachedMilestone represents a cached milestone.
type CachedMilestone struct {
	objectstorage.CachedObject
}

type CachedMilestones []*CachedMilestone

// Retain registers a new consumer for the cached milestones.
// milestone +1.
func (c CachedMilestones) Retain() CachedMilestones {
	cachedResult := make(CachedMilestones, len(c))
	for i, cachedMilestone := range c {
		cachedResult[i] = cachedMilestone.Retain() // milestone +1
	}

	return cachedResult
}

// Release releases the cached milestones, to be picked up by the persistence layer (as soon as all consumers are done).
// milestone -1.
func (c CachedMilestones) Release(force ...bool) {
	for _, cachedMilestone := range c {
		cachedMilestone.Release(force...) // milestone -1
	}
}

// Retain registers a new consumer for the cached milestone.
// milestone +1.
func (c *CachedMilestone) Retain() *CachedMilestone {
	return &CachedMilestone{c.CachedObject.Retain()} // milestone +1
}

// Milestone retrieves the milestone, that is cached in this container.
func (c *CachedMilestone) Milestone() *Milestone {
	//nolint:forcetypeassert // we will replace that with generics anyway
	return c.Get().(*Milestone)
}

// MilestoneStorageSize returns the size of the milestone storage.
func (s *Storage) MilestoneStorageSize() int {
	return s.milestoneStorage.GetSize()
}

// CachedMilestoneOrNil returns a cached milestone object.
// milestone +1.
func (s *Storage) CachedMilestoneOrNil(milestoneID iotago.MilestoneID) *CachedMilestone {

	cachedMilestone := s.milestoneStorage.Load(databaseKeyForMilestone(milestoneID)) // milestone +1
	if !cachedMilestone.Exists() {
		cachedMilestone.Release(true) // milestone -1

		return nil
	}

	return &CachedMilestone{CachedObject: cachedMilestone}
}

// CachedMilestoneByIndexOrNil returns a cached milestone object.
// milestone +1.
func (s *Storage) CachedMilestoneByIndexOrNil(milestoneIndex iotago.MilestoneIndex) *CachedMilestone {
	cachedMilestoneIdx := s.cachedMilestoneIndexOrNil(milestoneIndex) // milestoneIndex +1
	if cachedMilestoneIdx == nil {
		return nil
	}
	defer cachedMilestoneIdx.Release(true) // milestoneIndex -1

	return s.CachedMilestoneOrNil(cachedMilestoneIdx.MilestoneIndex().MilestoneID()) // milestone +1
}

// MilestoneParentsByIndex returns the parents of a milestone.
func (s *Storage) MilestoneParentsByIndex(milestoneIndex iotago.MilestoneIndex) (iotago.BlockIDs, error) {
	cachedMilestone := s.CachedMilestoneByIndexOrNil(milestoneIndex) // milestone +1
	if cachedMilestone == nil {
		return nil, ErrMilestoneNotFound
	}
	defer cachedMilestone.Release(true) // milestone -1

	return cachedMilestone.Milestone().Parents(), nil
}

// MilestoneTimestampByIndex returns the timestamp of a milestone.
func (s *Storage) MilestoneTimestampByIndex(milestoneIndex iotago.MilestoneIndex) (time.Time, error) {
	cachedMilestone := s.CachedMilestoneByIndexOrNil(milestoneIndex) // milestone +1
	if cachedMilestone == nil {
		return time.Time{}, ErrMilestoneNotFound
	}
	defer cachedMilestone.Release(true) // milestone -1

	return cachedMilestone.Milestone().Timestamp(), nil
}

// MilestoneTimestampUnixByIndex returns the unix timestamp of a milestone.
func (s *Storage) MilestoneTimestampUnixByIndex(milestoneIndex iotago.MilestoneIndex) (uint32, error) {
	cachedMilestone := s.CachedMilestoneByIndexOrNil(milestoneIndex) // milestone +1
	if cachedMilestone == nil {
		return 0, ErrMilestoneNotFound
	}
	defer cachedMilestone.Release(true) // milestone -1

	return cachedMilestone.Milestone().TimestampUnix(), nil
}

// SearchLatestMilestoneIndexInStore searches the latest milestone without accessing the cache layer.
func (s *Storage) SearchLatestMilestoneIndexInStore() iotago.MilestoneIndex {
	var latestMilestoneIndex iotago.MilestoneIndex

	s.NonCachedStorage().ForEachMilestoneIndex(func(index iotago.MilestoneIndex) bool {
		if latestMilestoneIndex < index {
			latestMilestoneIndex = index
		}

		return true
	})

	return latestMilestoneIndex
}

// StoreMilestoneIfAbsent stores a milestone if it is not known yet.
// milestone +1.
func (s *Storage) StoreMilestoneIfAbsent(milestonePayload *iotago.Milestone) (cachedMilestone *CachedMilestone, newlyAdded bool) {

	// compute the milestone ID
	msID, err := milestonePayload.ID()
	if err != nil {
		return nil, false
	}
	milestoneID := msID

	msIndexLookup, err := NewMilestoneIndex(milestonePayload, milestoneID)
	if err != nil {
		return nil, false
	}

	var innerErr error

	// Store milestone + milestoneIndex atomically in the same callback
	cachedMilestoneIdx := s.milestoneIndexStorage.ComputeIfAbsent(msIndexLookup.ObjectStorageKey(), func(_ []byte) objectstorage.StorableObject { // milestoneIndex +1
		newlyAdded = true

		ms, err := NewMilestone(milestonePayload, serializer.DeSeriModePerformValidation, milestoneID)
		if err != nil {
			innerErr = err

			return nil
		}

		cachedMilestone = &CachedMilestone{CachedObject: s.milestoneStorage.Store(ms)} // milestone +1

		msIndexLookup.Persist(true)
		msIndexLookup.SetModified(true)

		return msIndexLookup
	})
	defer cachedMilestoneIdx.Release(true) // milestoneIndex -1

	if innerErr != nil {
		return
	}

	// if we didn't create a new entry - retrieve the corresponding milestone (it should always exist since it gets created atomically)
	if !newlyAdded {
		cachedMilestone = &CachedMilestone{CachedObject: s.milestoneStorage.Load(databaseKeyForMilestone(milestoneID))} // milestone +1
	}

	return cachedMilestone, newlyAdded
}

// DeleteMilestone deletes the milestone in the cache/persistence layer.
func (s *Storage) DeleteMilestone(milestoneIndex iotago.MilestoneIndex) {
	cachedMilestoneIdx := s.cachedMilestoneIndexOrNil(milestoneIndex) // milestone index +1
	if cachedMilestoneIdx == nil {
		return
	}
	defer cachedMilestoneIdx.Release(true) // milestone index -1
	s.milestoneStorage.Delete(databaseKeyForMilestone(cachedMilestoneIdx.MilestoneIndex().MilestoneID()))
	s.milestoneIndexStorage.Delete(databaseKeyForMilestoneIndex(milestoneIndex))
}

// ShutdownMilestoneStorage shuts down milestones storage.
func (s *Storage) ShutdownMilestoneStorage() {
	s.milestoneIndexStorage.Shutdown()
	s.milestoneStorage.Shutdown()
}

// FlushMilestoneStorage flushes the milestones storage.
func (s *Storage) FlushMilestoneStorage() {
	s.milestoneIndexStorage.Flush()
	s.milestoneStorage.Flush()
}
