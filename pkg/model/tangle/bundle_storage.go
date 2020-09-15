package tangle

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/profile"
)

var (
	bundleStorage *objectstorage.ObjectStorage
)

func databaseKeyForBundle(tailTxHash hornet.Hash) []byte {
	return tailTxHash
}

func bundleFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	bndl := &Bundle{
		tailTx: key[:49],
		txs:    make(map[string]struct{}),
	}

	if err := bndl.UnmarshalObjectStorageValue(data); err != nil {
		return nil, err
	}

	return bndl, nil
}

func GetBundleStorageSize() int {
	return bundleStorage.GetSize()
}

func configureBundleStorage(store kvstore.KVStore, opts profile.CacheOpts) {

	bundleStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixBundles}),
		bundleFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.StoreOnCreation(false),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// ObjectStorage interface
func (bundle *Bundle) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("Bundle should never be updated: %v, TxHash: %v", bundle.hash.Trytes(), bundle.tailTx.Trytes()))
}

func (bundle *Bundle) ObjectStorageKey() []byte {
	return databaseKeyForBundle(bundle.tailTx)
}

func (bundle *Bundle) ObjectStorageValue() (data []byte) {

	/*
		 1 byte  	   				metadata
		 8 bytes uint64 			lastIndex
		 8 bytes uint64 			txCount
		 8 bytes uint64 			ledgerChangesCount
		49 bytes					bundleHash
		49 bytes					headTx
		49 bytes                 	txHashes		(x txCount)
		49 bytes + 8 bytes uint64 	ledgerChanges	(x ledgerChangesCount)
	*/

	txCount := len(bundle.txs)
	ledgerChangesCount := len(bundle.ledgerChanges)

	value := make([]byte, 172+txCount*49+57*ledgerChangesCount)

	value[0] = byte(bundle.metadata)
	binary.LittleEndian.PutUint64(value[1:], bundle.lastIndex)
	binary.LittleEndian.PutUint64(value[9:], uint64(txCount))
	binary.LittleEndian.PutUint64(value[17:], uint64(ledgerChangesCount))
	copy(value[25:74], bundle.hash)
	copy(value[74:123], bundle.headTx)

	offset := 123
	for txHash := range bundle.txs {
		copy(value[offset:offset+49], hornet.Hash(txHash))
		offset += 49
	}

	for addr, change := range bundle.ledgerChanges {
		copy(value[offset:offset+49], hornet.Hash(addr))
		offset += 49
		binary.LittleEndian.PutUint64(value[offset:], uint64(change))
		offset += 8
	}

	return value
}

func (bundle *Bundle) UnmarshalObjectStorageValue(data []byte) (err error) {

	/*
		 1 byte  	   				metadata
		 8 bytes uint64 			lastIndex
		 8 bytes uint64 			txCount
		 8 bytes uint64 			ledgerChangesCount
		49 bytes					bundleHash
		49 bytes					headTx
		49 bytes                 	txHashes		(x txCount)
		49 bytes + 8 bytes uint64 	ledgerChanges	(x ledgerChangesCount)
	*/

	bundle.metadata = bitmask.BitMask(data[0])
	bundle.lastIndex = binary.LittleEndian.Uint64(data[1:9])
	txCount := int(binary.LittleEndian.Uint64(data[9:17]))
	ledgerChangesCount := int(binary.LittleEndian.Uint64(data[17:25]))
	bundle.hash = data[25:74]
	bundle.headTx = data[74:123]

	offset := 123
	for i := 0; i < txCount; i++ {
		bundle.txs[string(data[offset:offset+49])] = struct{}{}
		offset += 49
	}

	if ledgerChangesCount > 0 {
		bundle.ledgerChanges = make(map[string]int64, ledgerChangesCount)
	}

	for i := 0; i < ledgerChangesCount; i++ {
		address := data[offset : offset+49]
		offset += 49
		balance := int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
		offset += 8
		bundle.ledgerChanges[string(address)] = balance
	}

	return nil
}

// Cached Object
type CachedBundle struct {
	objectstorage.CachedObject
}

type CachedBundles []*CachedBundle

func (cachedBundles CachedBundles) Retain() CachedBundles {
	cachedResult := CachedBundles{}
	for _, cachedBundle := range cachedBundles {
		cachedResult = append(cachedResult, cachedBundle.Retain())
	}
	return cachedResult
}

func (cachedBundles CachedBundles) Release(force ...bool) {
	for _, cachedBundle := range cachedBundles {
		cachedBundle.Release(force...)
	}
}

func (c *CachedBundle) Retain() *CachedBundle {
	return &CachedBundle{c.CachedObject.Retain()}
}

func (c *CachedBundle) ConsumeBundle(consumer func(*Bundle)) {

	c.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Bundle))
	}, true)
}

func (c *CachedBundle) GetBundle() *Bundle {
	return c.Get().(*Bundle)
}

// bundle +1
func GetCachedBundleOrNil(tailTxHash hornet.Hash) *CachedBundle {
	cachedBundle := bundleStorage.Load(databaseKeyForBundle(tailTxHash)) // bundle +1
	if !cachedBundle.Exists() {
		cachedBundle.Release(true) // bundle -1
		return nil
	}
	return &CachedBundle{CachedObject: cachedBundle}
}

// GetStoredBundleOrNil returns a bundle object without accessing the cache layer.
func GetStoredBundleOrNil(tailTxHash hornet.Hash) *Bundle {
	storedBundle := bundleStorage.LoadObjectFromStore(tailTxHash)
	if storedBundle == nil {
		return nil
	}
	return storedBundle.(*Bundle)
}

// BundleHashConsumer consumes the given tailTxHash during looping through all bundles in the persistence layer.
type BundleHashConsumer func(txHash hornet.Hash) bool

// ForEachBundleHash loops over all bundle hashes.
func ForEachBundleHash(consumer BundleHashConsumer, skipCache bool) {
	bundleStorage.ForEachKeyOnly(func(tailTxHash []byte) bool {
		return consumer(tailTxHash)
	}, skipCache)
}

// bundle +-0
func ContainsBundle(tailTxHash hornet.Hash) bool {
	return bundleStorage.Contains(databaseKeyForBundle(tailTxHash))
}

// bundle +-0
func DeleteBundle(tailTxHash hornet.Hash) {
	bundleStorage.Delete(databaseKeyForBundle(tailTxHash))
}

func ShutdownBundleStorage() {
	bundleStorage.Shutdown()
}

func FlushBundleStorage() {
	bundleStorage.Flush()
}

////////////////////////////////////////////////////////////////////////////////////

// GetBundles returns all existing bundle instances for that bundle hash
// bundle +1
func GetBundles(bundleHash hornet.Hash, forceRelease bool, maxFind ...int) CachedBundles {

	var cachedBndls CachedBundles

	for _, txTailHash := range GetBundleTailTransactionHashes(bundleHash, forceRelease, maxFind...) {
		cachedBndl := GetCachedBundleOrNil(txTailHash) // bundle +1
		if cachedBndl == nil {
			continue
		}

		cachedBndls = append(cachedBndls, cachedBndl)
	}

	if len(cachedBndls) == 0 {
		return nil
	}

	return cachedBndls
}

// GetBundlesOfTransactionOrNil gets all bundle instances in which this transaction is present.
// A transaction can be in multiple bundle instances simultaneously
// due to the nature of reattached transactions being able to form infinite amount of bundles
// which attach to the same underlying bundle transaction. For example it is possible to reattach
// a bundle's tail transaction directly "on top" of the origin one.
// bundle +1
func GetBundlesOfTransactionOrNil(txHash hornet.Hash, forceRelease bool) CachedBundles {

	var cachedBndls CachedBundles

	cachedTxMeta := GetCachedTxMetadataOrNil(txHash) // meta +1
	if cachedTxMeta == nil {
		return nil
	}
	defer cachedTxMeta.Release(forceRelease) // meta -1

	if cachedTxMeta.GetMetadata().IsTail() {
		cachedBndl := GetCachedBundleOrNil(txHash) // bundle +1
		if cachedBndl == nil {
			return nil
		}
		return append(cachedBndls, cachedBndl)
	}

	tailTxHashes := getTailApproversOfSameBundle(cachedTxMeta.GetMetadata().GetBundleHash(), txHash, forceRelease)
	for _, tailTxHash := range tailTxHashes {
		cachedBndl := GetCachedBundleOrNil(tailTxHash) // bundle +1
		if cachedBndl == nil {
			continue
		}
		cachedBndls = append(cachedBndls, cachedBndl)
	}

	if len(cachedBndls) == 0 {
		return nil
	}

	return cachedBndls
}

////////////////////////////////////////////////////////////////////////////////

// tx +1
func AddTransactionToStorage(hornetTx *hornet.Transaction, latestMilestoneIndex milestone.Index, requested bool, forceRelease bool, reapply bool) (cachedTx *CachedTransaction, alreadyAdded bool) {

	cachedTx, isNew := StoreTransactionIfAbsent(hornetTx) // tx +1
	if !isNew && !reapply {
		return cachedTx, true
	}

	// Store the tx in the bundleTransactionsStorage
	StoreBundleTransaction(cachedTx.GetTransaction().GetBundleHash(), cachedTx.GetTransaction().GetTxHash(), cachedTx.GetTransaction().IsTail()).Release(forceRelease)

	StoreApprover(cachedTx.GetTransaction().GetTrunkHash(), cachedTx.GetTransaction().GetTxHash()).Release(forceRelease)
	if !bytes.Equal(cachedTx.GetTransaction().GetTrunkHash(), cachedTx.GetTransaction().GetBranchHash()) {
		StoreApprover(cachedTx.GetTransaction().GetBranchHash(), cachedTx.GetTransaction().GetTxHash()).Release(forceRelease)
	}

	// Force release Tag, Address, UnconfirmedTx since its not needed for solidification/confirmation
	StoreTag(cachedTx.GetTransaction().GetTag(), cachedTx.GetTransaction().GetTxHash()).Release(true)

	StoreAddress(cachedTx.GetTransaction().GetAddress(), cachedTx.GetTransaction().GetTxHash(), cachedTx.GetTransaction().IsValue()).Release(true)

	// Store only non-requested transactions, since all requested transactions are confirmed by a milestone anyway
	// This is only used to delete unconfirmed transactions from the database at pruning
	if !requested {
		StoreUnconfirmedTx(latestMilestoneIndex, cachedTx.GetTransaction().GetTxHash()).Release(true)
	}

	// If the transaction is part of a milestone, the bundle must be created here
	// Otherwise, bundles are created if tailTx becomes solid
	if IsMaybeMilestoneTx(cachedTx.Retain()) { // tx pass +1
		tryConstructBundle(cachedTx.Retain(), false)
	}

	return cachedTx, false
}

// tryConstructBundle tries to construct a bundle (maybe txs are still missing in the DB)
// isSolidTail should only be false for possible milestone txs
func tryConstructBundle(cachedTx *CachedTransaction, isSolidTail bool) {
	defer cachedTx.Release() // tx -1

	if ContainsBundle(cachedTx.GetTransaction().GetTxHash()) {
		// Bundle already exists
		return
	}

	if !isSolidTail && !cachedTx.GetTransaction().IsTail() {
		// If Tx is not a tail, search all tailTx that reference this tx and try to create the bundles
		tailTxHashes := getTailApproversOfSameBundle(cachedTx.GetTransaction().GetBundleHash(), cachedTx.GetTransaction().GetTxHash(), false)
		for _, tailTxHash := range tailTxHashes {
			cachedTailTx := GetCachedTransactionOrNil(tailTxHash) // tx +1
			if cachedTailTx == nil {
				continue
			}

			tryConstructBundle(cachedTailTx.Retain(), false) // tx pass +1
			cachedTailTx.Release()                           // tx -1
		}
		return
	}

	// create a new bundle instance
	bndl := &Bundle{
		tailTx:    cachedTx.GetTransaction().GetTxHash(),
		metadata:  bitmask.BitMask(0),
		lastIndex: cachedTx.GetTransaction().Tx.LastIndex,
		hash:      cachedTx.GetTransaction().GetBundleHash(),
		txs:       make(map[string]struct{}),
	}

	bndl.txs[string(cachedTx.GetTransaction().GetTxHash())] = struct{}{}

	// check whether it is a bundle with only one transaction
	if cachedTx.GetTransaction().Tx.CurrentIndex == cachedTx.GetTransaction().Tx.LastIndex {
		bndl.headTx = cachedTx.GetTransaction().GetTxHash()
	} else {
		// lets try to complete the bundle by assigning txs into this bundle
		if !constructBundle(bndl, cachedTx.GetCachedMetadata()) { // meta pass +1
			if isSolidTail {
				panic("Can't create bundle, but tailTx is solid")
			}
			return
		}
	}

	isMilestone, err := CheckIfMilestone(bndl)
	if err != nil {
		// Invalid milestone
		Events.ReceivedInvalidMilestone.Trigger(fmt.Errorf("invalid milestone detected! Err: %w", err))

		if !isSolidTail {
			// Only valid milestones should be added to the database if not triggered via solidification
			return
		}
	}

	if !isSolidTail && !isMilestone {
		// Only valid milestones should be added to the database if not triggered via solidification
		return
	}

	newlyAdded := false
	cachedObj := bundleStorage.ComputeIfAbsent(bndl.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject { // bundle +1
		newlyAdded = true

		if bndl.validate() {
			bndl.calcLedgerChanges()
		}

		metrics.SharedServerMetrics.ValidatedBundles.Inc()

		bndl.Persist()
		bndl.SetModified()

		return bndl
	})

	if newlyAdded {

		cachedBndl := &CachedBundle{CachedObject: cachedObj}
		bndl := cachedBndl.GetBundle()
		bndl.ApplySpentAddresses()

		if bndl.IsValid() && bndl.ValidStrictSemantics() && bndl.IsMilestone() {
			// Force release to store milestones without caching
			//
			// Milestone has to be stored after the bundle itself, otherwise there would be a race condition
			// between "ContainsMilestone" and "GetMilestoneOrNil"
			StoreMilestone(bndl).Release(true) // milestone +-0

			Events.ReceivedValidMilestone.Trigger(cachedBndl) // bundle pass +1
		}
	}

	cachedObj.Release() // bundle -1
}

// Remaps transactions into the given bundle by traversing from the given start transaction through the trunk.
func constructBundle(bndl *Bundle, cachedStartTxMeta *CachedMetadata) bool {

	cachedCurrentTxMeta := cachedStartTxMeta

	// iterate as long as the bundle isn't complete and prevent cyclic transactions (such as the genesis)
	for !bytes.Equal(cachedCurrentTxMeta.GetMetadata().GetTxHash(), cachedCurrentTxMeta.GetMetadata().GetTrunkHash()) && !bndl.isComplete() && !cachedCurrentTxMeta.GetMetadata().IsHead() {

		// check whether the trunk transaction is known to the transaction storage.
		if !ContainsTransaction(cachedCurrentTxMeta.GetMetadata().GetTrunkHash()) {
			cachedCurrentTxMeta.Release() // meta -1
			return false
		}

		trunkTxMeta := loadBundleTxMetaIfExistsOrPanic(cachedCurrentTxMeta.GetMetadata().GetTrunkHash(), bndl.hash) // meta +1

		// check whether trunk is in bundle instance already
		if _, trunkAlreadyInBundle := bndl.txs[string(cachedCurrentTxMeta.GetMetadata().GetTrunkHash())]; trunkAlreadyInBundle {
			cachedCurrentTxMeta.Release() // meta -1
			cachedCurrentTxMeta = trunkTxMeta
			continue
		}

		if !bytes.Equal(trunkTxMeta.GetMetadata().GetBundleHash(), cachedStartTxMeta.GetMetadata().GetBundleHash()) {
			trunkTxMeta.Release() // meta -1

			// Tx has invalid structure, but is "complete"
			break
		}

		// assign as head if last tx
		if trunkTxMeta.GetMetadata().IsHead() {
			bndl.headTx = trunkTxMeta.GetMetadata().GetTxHash()
		}

		// assign trunk tx to this bundle
		bndl.txs[string(trunkTxMeta.GetMetadata().GetTxHash())] = struct{}{}

		// modify and advance to perhaps complete the bundle
		bndl.SetModified(true)
		cachedCurrentTxMeta.Release() // meta -1
		cachedCurrentTxMeta = trunkTxMeta
	}

	cachedCurrentTxMeta.Release() // meta -1
	return true
}

// Create a new bundle instance as soon as a tailTx gets solid
func OnTailTransactionSolid(cachedTx *CachedTransaction) {
	tryConstructBundle(cachedTx, true) // tx +-0 (it has +1 and will be released in tryConstructBundle)
}
