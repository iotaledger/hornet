package storage

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/pkg/common"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/profile"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	ErrMilestoneNotFound = errors.New("milestone not found")
)

func databaseKeyForMilestoneIndex(milestoneIndex milestone.Index) []byte {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(milestoneIndex))
	return bytes
}

func databaseKeyForMilestone(milestoneID iotago.MilestoneID) []byte {
	bytes := make([]byte, iotago.MilestoneIDLength)
	copy(bytes, milestoneID[:])
	return bytes
}

func milestoneIndexFromDatabaseKey(key []byte) milestone.Index {
	return milestone.Index(binary.LittleEndian.Uint32(key))
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

		bID, err := marshalUtil.ReadBytes(iotago.BlockIDLength)
		if err != nil {
			return nil, err
		}
		blockID := iotago.BlockID{}
		copy(blockID[:], bID)

		return &MilestoneIndex{
			Index:       milestoneIndexFromDatabaseKey(key),
			milestoneID: milestoneID,
			blockID:     blockID,
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

	Index       milestone.Index
	milestoneID iotago.MilestoneID
	blockID     iotago.BlockID
}

func NewMilestoneIndex(milestonePayload *iotago.Milestone, blockID iotago.BlockID, milestoneID ...iotago.MilestoneID) (*MilestoneIndex, error) {

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
		Index:       milestone.Index(milestonePayload.Index),
		milestoneID: msID,
		blockID:     blockID,
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
		32 byte block ID
	*/

	return marshalutil.New(64).
		WriteBytes(ms.milestoneID[:]).
		WriteBytes(ms.blockID[:iotago.BlockIDLength]).
		Bytes()
}

// cachedMilestoneIndex represents a cached milestone index lookup.
type cachedMilestoneIndex struct {
	objectstorage.CachedObject
}

// Retain registers a new consumer for the cached milestone index lookup.
// milestone index +1
func (c *cachedMilestoneIndex) Retain() *cachedMilestoneIndex {
	return &cachedMilestoneIndex{c.CachedObject.Retain()} // milestone index +1
}

// MilestoneIndex retrieves the milestone index, that is cached in this container.
func (c *cachedMilestoneIndex) MilestoneIndex() *MilestoneIndex {
	return c.Get().(*MilestoneIndex)
}

// cachedMilestoneIndexOrNil returns a cached milestone index object.
// milestone index +1
func (s *Storage) cachedMilestoneIndexOrNil(milestoneIndex milestone.Index) *cachedMilestoneIndex {
	cachedMilestoneIdx := s.milestoneIndexStorage.Load(databaseKeyForMilestoneIndex(milestoneIndex)) // milestone index +1
	if !cachedMilestoneIdx.Exists() {
		cachedMilestoneIdx.Release(true) // milestone index -1
		return nil
	}
	return &cachedMilestoneIndex{CachedObject: cachedMilestoneIdx}
}

// ContainsMilestoneIndex returns if the given milestone exists in the cache/persistence layer.
func (s *Storage) ContainsMilestoneIndex(milestoneIndex milestone.Index, readOptions ...ReadOption) bool {
	return s.milestoneIndexStorage.Contains(databaseKeyForMilestoneIndex(milestoneIndex), readOptions...)
}

// MilestoneIndexConsumer consumes the given index during looping through all milestones.
type MilestoneIndexConsumer func(index milestone.Index) bool

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

func (ms *Milestone) Index() milestone.Index {
	return milestone.Index(ms.Milestone().Index)
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
// milestone +1
func (c CachedMilestones) Retain() CachedMilestones {
	cachedResult := make(CachedMilestones, len(c))
	for i, cachedMilestone := range c {
		cachedResult[i] = cachedMilestone.Retain() // milestone +1
	}
	return cachedResult
}

// Release releases the cached milestones, to be picked up by the persistence layer (as soon as all consumers are done).
// milestone -1
func (c CachedMilestones) Release(force ...bool) {
	for _, cachedMilestone := range c {
		cachedMilestone.Release(force...) // milestone -1
	}
}

// Retain registers a new consumer for the cached milestone.
// milestone +1
func (c *CachedMilestone) Retain() *CachedMilestone {
	return &CachedMilestone{c.CachedObject.Retain()} // milestone +1
}

// Milestone retrieves the milestone, that is cached in this container.
func (c *CachedMilestone) Milestone() *Milestone {
	return c.Get().(*Milestone)
}

// MilestoneStorageSize returns the size of the milestone storage.
func (s *Storage) MilestoneStorageSize() int {
	return s.milestoneStorage.GetSize()
}

// CachedMilestoneOrNil returns a cached milestone object.
// milestone +1
func (s *Storage) CachedMilestoneOrNil(milestoneID iotago.MilestoneID) *CachedMilestone {

	cachedMilestone := s.milestoneStorage.Load(databaseKeyForMilestone(milestoneID)) // milestone +1
	if !cachedMilestone.Exists() {
		cachedMilestone.Release(true) // milestone -1
		return nil
	}
	return &CachedMilestone{CachedObject: cachedMilestone}
}

// CachedMilestoneByIndexOrNil returns a cached milestone object.
// milestone +1
func (s *Storage) CachedMilestoneByIndexOrNil(milestoneIndex milestone.Index) *CachedMilestone {
	cachedMilestoneIdx := s.cachedMilestoneIndexOrNil(milestoneIndex) // milestoneIndex +1
	if cachedMilestoneIdx == nil {
		return nil
	}
	defer cachedMilestoneIdx.Release(true) // milestoneIndex -1

	return s.CachedMilestoneOrNil(cachedMilestoneIdx.MilestoneIndex().MilestoneID()) // milestone +1
}

// MilestoneBlockIDByIndex returns the block ID of a milestone.
// Attention: this can be different from node to node, because only the first seen reattachment of milestone payload
// is stored in a node. This information should never be exposed via external API in any way.
func (s *Storage) MilestoneBlockIDByIndex(milestoneIndex milestone.Index) (iotago.BlockID, error) {
	cachedMilestoneIdx := s.cachedMilestoneIndexOrNil(milestoneIndex) // milestoneIndex +1
	if cachedMilestoneIdx == nil {
		return iotago.EmptyBlockID(), ErrMilestoneNotFound
	}
	defer cachedMilestoneIdx.Release(true) // milestoneIndex -1

	return cachedMilestoneIdx.MilestoneIndex().blockID, nil
}

// MilestoneParentsByIndex returns the parents of a milestone.
func (s *Storage) MilestoneParentsByIndex(milestoneIndex milestone.Index) (iotago.BlockIDs, error) {
	cachedMilestone := s.CachedMilestoneByIndexOrNil(milestoneIndex) // milestone +1
	if cachedMilestone == nil {
		return nil, ErrMilestoneNotFound
	}
	defer cachedMilestone.Release(true) // milestone -1

	return cachedMilestone.Milestone().Parents(), nil
}

// MilestoneTimestampByIndex returns the timestamp of a milestone.
func (s *Storage) MilestoneTimestampByIndex(milestoneIndex milestone.Index) (time.Time, error) {
	cachedMilestone := s.CachedMilestoneByIndexOrNil(milestoneIndex) // milestone +1
	if cachedMilestone == nil {
		return time.Time{}, ErrMilestoneNotFound
	}
	defer cachedMilestone.Release(true) // milestone -1

	return cachedMilestone.Milestone().Timestamp(), nil
}

// MilestoneTimestampUnixByIndex returns the unix timestamp of a milestone.
func (s *Storage) MilestoneTimestampUnixByIndex(milestoneIndex milestone.Index) (uint32, error) {
	cachedMilestone := s.CachedMilestoneByIndexOrNil(milestoneIndex) // milestone +1
	if cachedMilestone == nil {
		return 0, ErrMilestoneNotFound
	}
	defer cachedMilestone.Release(true) // milestone -1

	return cachedMilestone.Milestone().TimestampUnix(), nil
}

// SearchLatestMilestoneIndexInStore searches the latest milestone without accessing the cache layer.
func (s *Storage) SearchLatestMilestoneIndexInStore() milestone.Index {
	var latestMilestoneIndex milestone.Index

	s.NonCachedStorage().ForEachMilestoneIndex(func(index milestone.Index) bool {
		if latestMilestoneIndex < index {
			latestMilestoneIndex = index
		}

		return true
	})

	return latestMilestoneIndex
}

// milestone +1
func (s *Storage) StoreMilestoneIfAbsent(milestonePayload *iotago.Milestone, blockID iotago.BlockID) (cachedMilestone *CachedMilestone, newlyAdded bool) {

	// compute the milestone ID
	msID, err := milestonePayload.ID()
	if err != nil {
		return nil, false
	}
	milestoneID := msID

	msIndexLookup, err := NewMilestoneIndex(milestonePayload, blockID, milestoneID)
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
// +-0
func (s *Storage) DeleteMilestone(milestoneIndex milestone.Index) {
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
