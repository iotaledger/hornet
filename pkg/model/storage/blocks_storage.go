package storage

import (
	"time"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/objectstorage"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/profile"
	iotago "github.com/iotaledger/iota.go/v3"
)

func BlockCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(cachedBlock *CachedBlock))(params[0].(*CachedBlock).Retain()) // block pass +1
}

func BlockMetadataCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(cachedBlockMeta *CachedMetadata))(params[0].(*CachedMetadata).Retain()) // block pass +1
}

func BlockIDCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(blockID iotago.BlockID))(params[0].(iotago.BlockID))
}

func NewBlockCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(cachedBlock *CachedBlock, latestMilestoneIndex iotago.MilestoneIndex, confirmedMilestoneIndex iotago.MilestoneIndex))(params[0].(*CachedBlock).Retain(), params[1].(iotago.MilestoneIndex), params[2].(iotago.MilestoneIndex)) // block pass +1
}

func BlockReferencedCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(cachedBlockMeta *CachedMetadata, msIndex iotago.MilestoneIndex, confTime uint32))(params[0].(*CachedMetadata).Retain(), params[1].(iotago.MilestoneIndex), params[2].(uint32)) // block pass +1
}

// CachedBlock contains two cached objects, one for block and one for metadata.
type CachedBlock struct {
	block    objectstorage.CachedObject
	metadata objectstorage.CachedObject
}

func NewCachedBlock(block objectstorage.CachedObject, metadata objectstorage.CachedObject) *CachedBlock {
	return &CachedBlock{
		block:    block,
		metadata: metadata,
	}
}

// CachedMetadata contains the cached object only for metadata.
type CachedMetadata struct {
	objectstorage.CachedObject
}

func NewCachedMetadata(metadata objectstorage.CachedObject) *CachedMetadata {
	return &CachedMetadata{CachedObject: metadata}
}

type CachedBlocks []*CachedBlock

// Retain registers a new consumer for the cached blocks.
// block +1.
func (cachedBlocks CachedBlocks) Retain() CachedBlocks {
	cachedResult := make(CachedBlocks, len(cachedBlocks))
	for i, cachedBlock := range cachedBlocks {
		cachedResult[i] = cachedBlock.Retain() // block +1
	}

	return cachedResult
}

// Release releases the cached blocks, to be picked up by the persistence layer (as soon as all consumers are done).
// block -1.
func (cachedBlocks CachedBlocks) Release(force ...bool) {
	for _, cachedBlock := range cachedBlocks {
		cachedBlock.Release(force...) // block -1
	}
}

// Block retrieves the block, that is cached in this container.
func (c *CachedBlock) Block() *Block {
	//nolint:forcetypeassert // we will replace that with generics anyway
	return c.block.Get().(*Block)
}

// CachedMetadata returns the underlying cached metadata.
// meta +1.
func (c *CachedBlock) CachedMetadata() *CachedMetadata {
	return &CachedMetadata{c.metadata.Retain()} // meta +1
}

// Metadata retrieves the metadata, that is cached in this container.
func (c *CachedBlock) Metadata() *BlockMetadata {
	//nolint:forcetypeassert // we will replace that with generics anyway
	return c.metadata.Get().(*BlockMetadata)
}

// Metadata retrieves the metadata, that is cached in this container.
func (c *CachedMetadata) Metadata() *BlockMetadata {
	//nolint:forcetypeassert // we will replace that with generics anyway
	return c.Get().(*BlockMetadata)
}

// Retain registers a new consumer for the cached block and metadata.
// block +1.
func (c *CachedBlock) Retain() *CachedBlock {
	return &CachedBlock{
		c.block.Retain(),    // block +1
		c.metadata.Retain(), // meta +1
	}
}

// Retain registers a new consumer for the cached metadata.
// meta +1.
func (c *CachedMetadata) Retain() *CachedMetadata {
	return &CachedMetadata{c.CachedObject.Retain()} // meta +1
}

// Exists returns true if the block in this container does exist
// (could be found in the database and was not marked as deleted).
func (c *CachedBlock) Exists() bool {
	return c.block.Exists()
}

// ConsumeBlockAndMetadata consumes the underlying block and metadata.
// block -1.
// meta -1.
func (c *CachedBlock) ConsumeBlockAndMetadata(consumer func(*Block, *BlockMetadata)) {

	c.block.Consume(func(txObject objectstorage.StorableObject) { // block -1
		c.metadata.Consume(func(metadataObject objectstorage.StorableObject) { // meta -1
			//nolint:forcetypeassert // we will replace that with generics anyway
			consumer(txObject.(*Block), metadataObject.(*BlockMetadata))
		}, true)
	}, true)
}

// ConsumeBlock consumes the underlying block.
// block -1.
// meta -1.
func (c *CachedBlock) ConsumeBlock(consumer func(*Block)) {
	defer c.metadata.Release(true)                              // meta -1
	c.block.Consume(func(object objectstorage.StorableObject) { // block -1
		//nolint:forcetypeassert // we will replace that with generics anyway
		consumer(object.(*Block))
	}, true)
}

// ConsumeMetadata consumes the underlying metadata.
// block -1.
// meta -1.
func (c *CachedBlock) ConsumeMetadata(consumer func(*BlockMetadata)) {
	defer c.block.Release(true)                                    // block -1
	c.metadata.Consume(func(object objectstorage.StorableObject) { // meta -1
		//nolint:forcetypeassert // we will replace that with generics anyway
		consumer(object.(*BlockMetadata))
	}, true)
}

// ConsumeMetadata consumes the metadata.
// meta -1.
func (c *CachedMetadata) ConsumeMetadata(consumer func(*BlockMetadata)) {
	c.Consume(func(object objectstorage.StorableObject) { // meta -1
		//nolint:forcetypeassert // we will replace that with generics anyway
		consumer(object.(*BlockMetadata))
	}, true)
}

// Release releases the cached block and metadata, to be picked up by the persistence layer (as soon as all consumers are done).
// block -1.
func (c *CachedBlock) Release(force ...bool) {
	c.block.Release(force...)    // block -1
	c.metadata.Release(force...) // meta -1
}

func BlockFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	block := &Block{
		data: data,
	}
	copy(block.blockID[:], key[:iotago.BlockIDLength])

	return block, nil
}

func (s *Storage) BlockStorageSize() int {
	return s.blocksStorage.GetSize()
}

func (s *Storage) BlockMetadataStorageSize() int {
	return s.metadataStorage.GetSize()
}

func (s *Storage) configureBlockStorage(store kvstore.KVStore, opts *profile.CacheOpts) error {

	cacheTime, err := time.ParseDuration(opts.CacheTime)
	if err != nil {
		return err
	}

	leakDetectionMaxConsumerHoldTime, err := time.ParseDuration(opts.LeakDetectionOptions.MaxConsumerHoldTime)
	if err != nil {
		return err
	}

	blocksStore, err := store.WithRealm([]byte{common.StorePrefixBlocks})
	if err != nil {
		return err
	}

	s.blocksStorage = objectstorage.New(
		blocksStore,
		BlockFactory,
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

	metadataStore, err := store.WithRealm([]byte{common.StorePrefixBlockMetadata})
	if err != nil {
		return err
	}

	s.metadataStorage = objectstorage.New(
		metadataStore,
		MetadataFactory,
		objectstorage.CacheTime(cacheTime),
		objectstorage.PersistenceEnabled(true),
		objectstorage.StoreOnCreation(false),
		objectstorage.ReleaseExecutorWorkerCount(opts.ReleaseExecutorWorkerCount),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   leakDetectionMaxConsumerHoldTime,
			}),
	)

	return nil
}

// CachedBlockOrNil returns a cached block object.
// block +1.
func (s *Storage) CachedBlockOrNil(blockID iotago.BlockID) *CachedBlock {
	cachedBlock := s.blocksStorage.Load(blockID[:]) // block +1
	if !cachedBlock.Exists() {
		cachedBlock.Release(true) // block -1

		return nil
	}

	cachedBlockMeta := s.metadataStorage.Load(blockID[:]) // meta +1
	if !cachedBlockMeta.Exists() {
		cachedBlock.Release(true)     // block -1
		cachedBlockMeta.Release(true) // meta -1

		return nil
	}

	return &CachedBlock{
		block:    cachedBlock,
		metadata: cachedBlockMeta,
	}
}

// CachedBlock returns a cached block object.
// block +1.
func (s *Storage) CachedBlock(blockID iotago.BlockID) (*CachedBlock, error) {
	return s.CachedBlockOrNil(blockID), nil // block +1
}

// Block returns an iotago block object.
func (s *Storage) Block(blockID iotago.BlockID) (*iotago.Block, error) {
	cachedBlock, err := s.CachedBlock(blockID)
	if err != nil {
		return nil, err
	}

	if cachedBlock == nil {
		//nolint:nilnil // nil, nil is ok in this context, even if it is not go idiomatic
		return nil, nil
	}

	defer cachedBlock.Release(true)

	return cachedBlock.Block().Block(), nil
}

// CachedBlockMetadataOrNil returns a cached metadata object.
// meta +1.
func (s *Storage) CachedBlockMetadataOrNil(blockID iotago.BlockID) *CachedMetadata {
	cachedBlockMeta := s.metadataStorage.Load(blockID[:]) // meta +1
	if !cachedBlockMeta.Exists() {
		cachedBlockMeta.Release(true) // meta -1

		return nil
	}

	return &CachedMetadata{CachedObject: cachedBlockMeta}
}

// CachedBlockMetadata returns a cached metadata object.
// meta +1.
func (s *Storage) CachedBlockMetadata(blockID iotago.BlockID) (*CachedMetadata, error) {
	return s.CachedBlockMetadataOrNil(blockID), nil
}

// StoredMetadataOrNil returns a metadata object without accessing the cache layer.
func (s *Storage) StoredMetadataOrNil(blockID iotago.BlockID) *BlockMetadata {
	storedMeta := s.metadataStorage.LoadObjectFromStore(blockID[:])
	if storedMeta == nil {
		return nil
	}

	//nolint:forcetypeassert // we will replace that with generics anyway
	return storedMeta.(*BlockMetadata)
}

// ContainsBlock returns if the given block exists in the cache/persistence layer.
func (s *Storage) ContainsBlock(blockID iotago.BlockID, readOptions ...ReadOption) bool {
	return s.blocksStorage.Contains(blockID[:], readOptions...)
}

// BlockExistsInStore returns if the given block exists in the persistence layer.
func (s *Storage) BlockExistsInStore(blockID iotago.BlockID) bool {
	return s.blocksStorage.ObjectExistsInStore(blockID[:])
}

// BlockMetadataExistsInStore returns if the given block metadata exists in the persistence layer.
func (s *Storage) BlockMetadataExistsInStore(blockID iotago.BlockID) bool {
	return s.metadataStorage.ObjectExistsInStore(blockID[:])
}

// StoreBlockIfAbsent returns a cached object and stores the block in the persistence layer if it was absent.
// block +1.
func (s *Storage) StoreBlockIfAbsent(block *Block) (cachedBlock *CachedBlock, newlyAdded bool) {

	// Store block + metadata atomically in the same callback
	var cachedBlockMeta objectstorage.CachedObject

	cachedBlockData := s.blocksStorage.ComputeIfAbsent(block.ObjectStorageKey(), func(_ []byte) objectstorage.StorableObject { // block +1
		newlyAdded = true

		metadata := &BlockMetadata{
			blockID: block.BlockID(),
			parents: block.Parents(),
		}

		cachedBlockMeta = s.metadataStorage.Store(metadata) // meta +1

		block.Persist(true)
		block.SetModified(true)

		return block
	})

	// if we didn't create a new entry - retrieve the corresponding metadata (it should always exist since it gets created atomically)
	if !newlyAdded {
		cachedBlockMeta = s.metadataStorage.Load(block.blockID[:]) // meta +1
	}

	return &CachedBlock{block: cachedBlockData, metadata: cachedBlockMeta}, newlyAdded
}

// BlockIDConsumer consumes the given block ID during looping through all blocks.
// Returning false from this function indicates to abort the iteration.
type BlockIDConsumer func(blockID iotago.BlockID) bool

// ForEachBlockID loops over all block IDs.
func (s *Storage) ForEachBlockID(consumer BlockIDConsumer, iteratorOptions ...IteratorOption) {

	s.blocksStorage.ForEachKeyOnly(func(key []byte) bool {
		blockID := iotago.BlockID{}
		copy(blockID[:], key)

		return consumer(blockID)
	}, ObjectStorageIteratorOptions(iteratorOptions...)...)
}

// ForEachBlockID loops over all block IDs.
func (ns *NonCachedStorage) ForEachBlockID(consumer BlockIDConsumer, iteratorOptions ...IteratorOption) {

	ns.storage.blocksStorage.ForEachKeyOnly(func(key []byte) bool {
		blockID := iotago.BlockID{}
		copy(blockID[:], key)

		return consumer(blockID)
	}, append(ObjectStorageIteratorOptions(iteratorOptions...), objectstorage.WithIteratorSkipCache(true))...)
}

// ForEachBlockMetadataBlockID loops over all block metadata block IDs.
func (s *Storage) ForEachBlockMetadataBlockID(consumer BlockIDConsumer, iteratorOptions ...IteratorOption) {

	s.metadataStorage.ForEachKeyOnly(func(key []byte) bool {
		blockID := iotago.BlockID{}
		copy(blockID[:], key)

		return consumer(blockID)
	}, ObjectStorageIteratorOptions(iteratorOptions...)...)
}

// ForEachBlockMetadataBlockID loops over all block metadata block IDs.
func (ns *NonCachedStorage) ForEachBlockMetadataBlockID(consumer BlockIDConsumer, iteratorOptions ...IteratorOption) {

	ns.storage.metadataStorage.ForEachKeyOnly(func(key []byte) bool {
		blockID := iotago.BlockID{}
		copy(blockID[:], key)

		return consumer(blockID)
	}, append(ObjectStorageIteratorOptions(iteratorOptions...), objectstorage.WithIteratorSkipCache(true))...)
}

// DeleteBlock deletes the block and metadata in the cache/persistence layer.
func (s *Storage) DeleteBlock(blockID iotago.BlockID) {
	// meta has to be deleted before the block, otherwise we could run into a data race in the object storage
	s.metadataStorage.Delete(blockID[:])
	s.blocksStorage.Delete(blockID[:])
}

// DeleteBlockMetadata deletes the metadata in the cache/persistence layer.
func (s *Storage) DeleteBlockMetadata(blockID iotago.BlockID) {
	s.metadataStorage.Delete(blockID[:])
}

// ShutdownBlocksStorage shuts down the blocks storage.
func (s *Storage) ShutdownBlocksStorage() {
	s.blocksStorage.Shutdown()
	s.metadataStorage.Shutdown()
}

// FlushBlocksStorage flushes the blocks storage.
func (s *Storage) FlushBlocksStorage() {
	s.blocksStorage.Flush()
	s.metadataStorage.Flush()
}
