package tangle

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/profile"
)

var (
	bundleStorage *objectstorage.ObjectStorage
)

func databaseKeyForBundle(tailTxHash trinary.Hash) []byte {
	return trinary.MustTrytesToBytes(tailTxHash)[:49]
}

func bundleFactory(key []byte) objectstorage.StorableObject {
	return &Bundle{
		tailTx: trinary.MustBytesToTrytes(key[:49], 81),
		txs:    make(map[trinary.Hash]struct{}),
	}
}

func GetBundleStorageSize() int {
	return bundleStorage.GetSize()
}

func configureBundleStorage() {

	opts := profile.GetProfile().Caches.Bundles

	bundleStorage = objectstorage.New(
		database.GetHornetBadgerInstance(),
		[]byte{DBPrefixBundles},
		bundleFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// ObjectStorage interface
func (bundle *Bundle) Update(other objectstorage.StorableObject) {
	if obj, ok := other.(*Bundle); !ok {
		panic("invalid object passed to Bundle.Update()")
	} else {
		bundle.Lock()
		defer bundle.Unlock()
		other.(*Bundle).Lock()
		defer other.(*Bundle).Unlock()

		bundle.tailTx = obj.tailTx

		bundle.metadata = obj.metadata
		bundle.lastIndex = obj.lastIndex
		bundle.hash = obj.hash
		bundle.headTx = obj.headTx
		bundle.txs = obj.txs
		bundle.ledgerChanges = obj.ledgerChanges
	}
}

func (bundle *Bundle) GetStorageKey() []byte {
	return databaseKeyForBundle(bundle.tailTx)
}

func (bundle *Bundle) MarshalBinary() (data []byte, err error) {

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
	copy(value[25:74], trinary.MustTrytesToBytes(bundle.hash))
	copy(value[74:123], trinary.MustTrytesToBytes(bundle.headTx))

	offset := 123
	for txHash := range bundle.txs {
		copy(value[offset:offset+49], trinary.MustTrytesToBytes(txHash))
		offset += 49
	}

	for addr, change := range bundle.ledgerChanges {
		copy(value[offset:offset+49], trinary.MustTrytesToBytes(addr))
		offset += 49
		binary.LittleEndian.PutUint64(value[offset:], uint64(change))
		offset += 8
	}

	return value, nil
}

func (bundle *Bundle) UnmarshalBinary(data []byte) error {

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
	bundle.hash = trinary.MustBytesToTrytes(data[25:74], 81)
	bundle.headTx = trinary.MustBytesToTrytes(data[74:123], 81)

	offset := 123
	for i := 0; i < txCount; i++ {
		bundle.txs[trinary.MustBytesToTrytes(data[offset:offset+49], 81)] = struct{}{}
		offset += 49
	}

	if ledgerChangesCount > 0 {
		bundle.ledgerChanges = make(map[trinary.Trytes]int64, ledgerChangesCount)
	}

	for i := 0; i < ledgerChangesCount; i++ {
		address := trinary.MustBytesToTrytes(data[offset:offset+49], 81)
		offset += 49
		balance := int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
		offset += 8
		bundle.ledgerChanges[address] = balance
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

func (cachedBundles CachedBundles) Release() {
	for _, cachedBundle := range cachedBundles {
		cachedBundle.Release()
	}
}

func (c *CachedBundle) Retain() *CachedBundle {
	return &CachedBundle{c.CachedObject.Retain()}
}

func (c *CachedBundle) ConsumeBundle(consumer func(*Bundle)) {

	c.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Bundle))
	})
}

func (c *CachedBundle) GetBundle() *Bundle {
	return c.Get().(*Bundle)
}

// bundle +1
func GetCachedBundle(tailTxHash trinary.Hash) *CachedBundle {
	return &CachedBundle{bundleStorage.Load(databaseKeyForBundle(tailTxHash))}
}

// bundle +-0
func ContainsBundle(tailTxHash trinary.Hash) bool {
	return bundleStorage.Contains(databaseKeyForBundle(tailTxHash))
}

// bundle + 1
func StoreBundleIfAbsent(bundle *Bundle) (*CachedBundle, bool) {
	// Wait until all ongoing changes are done
	bundle.RLock()
	defer bundle.RUnlock()

	cachedBndl, isNew := bundleStorage.StoreIfAbsent(bundle)
	if !isNew {
		return nil, false
	}

	return &CachedBundle{CachedObject: cachedBndl}, true
}

// bundle +-0
func DeleteBundle(tailTxHash trinary.Hash) {
	bundleStorage.Delete(databaseKeyForBundle(tailTxHash))
}

func ShutdownBundleStorage() {
	bundleStorage.Shutdown()
}

////////////////////////////////////////////////////////////////////////////////////

// GetBundles returns all existing bundle instances for that bundle hash
// bundle +1
func GetBundles(bundleHash trinary.Hash) CachedBundles {

	var cachedBndls CachedBundles

	cachedBndlTailTxs := GetCachedBundleTailTransactions(bundleHash) // bundleTxs +1
	defer cachedBndlTailTxs.Release()                                // bundleTxs -1

	for _, bndlTailTx := range cachedBndlTailTxs {
		cachedBndl := GetCachedBundle(bndlTailTx.GetBundleTransaction().GetTxHash()) // bundle +1
		if !cachedBndl.Exists() {
			cachedBndl.Release() // bundle -1
			continue
		}

		cachedBndls = append(cachedBndls, cachedBndl)
	}

	if len(cachedBndls) == 0 {
		return nil
	}

	return cachedBndls
}

// GetBundleOfTailTransactionOrNil gets the bundle this tail transaction is present in or nil.
// bundle +1
func GetBundleOfTailTransactionOrNil(tailTxHash trinary.Hash) *CachedBundle {

	cachedBndl := GetCachedBundle(tailTxHash) // bundle +1
	if !cachedBndl.Exists() {
		cachedBndl.Release() // bundle -1
		return nil
	}

	return cachedBndl
}

// GetBundlesOfTransactionOrNil gets all bundle instances in which this transaction is present.
// A transaction can be in multiple bundle instances simultaneously
// due to the nature of reattached transactions being able to form infinite amount of bundles
// which attach to the same underlying bundle transaction. For example it is possible to reattach
// a bundle's tail transaction directly "on top" of the origin one.
// bundle +1
func GetBundlesOfTransactionOrNil(txHash trinary.Hash) CachedBundles {

	var cachedBndls CachedBundles

	cachedTx := GetCachedTransaction(txHash) // tx +1
	defer cachedTx.Release()                 // tx -1
	if !cachedTx.Exists() {
		return nil
	}

	if cachedTx.GetTransaction().IsTail() {
		cachedBndl := GetBundleOfTailTransactionOrNil(txHash) // bundle +1
		if cachedBndl == nil {
			return nil
		}
		return append(cachedBndls, cachedBndl)
	}

	tailTxHashes := getTailApproversOfSameBundle(cachedTx.GetTransaction().Tx.Bundle, txHash)
	for _, tailTxHash := range tailTxHashes {
		cachedBndl := GetBundleOfTailTransactionOrNil(tailTxHash) // bundle +1
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

func AddTransactionToStorage(hornetTx *hornet.Transaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, requested bool) (alreadyAdded bool) {

	// Store the tx in the storage, this will update the tx reference automatically
	cachedTx, isNew := StoreTransactionIfAbsent(hornetTx) // tx +1
	if !isNew {
		return true
	}
	defer cachedTx.Release() // tx -1

	// Store the tx in the bundleTransactionsStorage
	StoreBundleTransaction(cachedTx.GetTransaction().Tx.Bundle, cachedTx.GetTransaction().GetHash(), cachedTx.GetTransaction().IsTail()).Release()

	StoreApprover(cachedTx.GetTransaction().GetTrunk(), cachedTx.GetTransaction().GetHash()).Release()
	if cachedTx.GetTransaction().GetTrunk() != cachedTx.GetTransaction().GetBranch() {
		StoreApprover(cachedTx.GetTransaction().GetBranch(), cachedTx.GetTransaction().GetHash()).Release()
	}

	StoreTag(cachedTx.GetTransaction().Tx.Tag, cachedTx.GetTransaction().GetHash()).Release()

	StoreAddress(cachedTx.GetTransaction().Tx.Address, cachedTx.GetTransaction().GetHash()).Release()

	// Store only non-requested transactions, since all requested transactions are confirmed by a milestone anyway
	// This is only used to delete unconfirmed transactions from the database at pruning
	if !requested {
		StoreFirstSeenTx(firstSeenLatestMilestoneIndex, cachedTx.GetTransaction().GetHash()).Release()
	}

	// If the transaction is part of a milestone, the bundle must be created here
	// Otherwise, bundles are created if tailTx becomes solid
	if IsMaybeMilestoneTx(cachedTx.Retain()) { // tx pass +1
		tryConstructBundle(cachedTx.Retain(), false)
	}

	return false
}

func tryConstructBundle(cachedTx *CachedTransaction, isSolidTail bool) {
	defer cachedTx.Release() // tx -1

	if !isSolidTail && !cachedTx.GetTransaction().IsTail() {
		// If Tx is not a tail, search all tailTx that reference this tx and try to create the bundles
		tailTxHashes := getTailApproversOfSameBundle(cachedTx.GetTransaction().Tx.Bundle, cachedTx.GetTransaction().GetHash())
		for _, tailTxHash := range tailTxHashes {
			cachedTailTx := GetCachedTransaction(tailTxHash) // tx +1
			if !cachedTailTx.Exists() {
				cachedTailTx.Release() // tx -1
				continue
			}

			tryConstructBundle(cachedTailTx.Retain(), false) // tx pass +1
			cachedTailTx.Release()                           // tx -1
		}
		return
	}

	if ContainsBundle(cachedTx.GetTransaction().GetHash()) {
		// Bundle already exists
		return
	}

	// create a new bundle instance
	bndl := &Bundle{
		tailTx:    cachedTx.GetTransaction().GetHash(),
		metadata:  bitmask.BitMask(0),
		lastIndex: cachedTx.GetTransaction().Tx.LastIndex,
		hash:      cachedTx.GetTransaction().Tx.Bundle,
		txs:       make(map[trinary.Hash]struct{}),
	}

	bndl.txs[cachedTx.GetTransaction().GetHash()] = struct{}{}

	// check whether it is a bundle with only one transaction
	if cachedTx.GetTransaction().Tx.CurrentIndex == cachedTx.GetTransaction().Tx.LastIndex {
		bndl.headTx = cachedTx.GetTransaction().GetHash()
	} else {
		// lets try to complete the bundle by assigning txs into this bundle
		if !constructBundle(bndl, cachedTx.Retain()) { // tx pass +1
			if isSolidTail {
				panic("Can't create bundle, but tailTx is solid")
			}
			return
		}
	}

	cachedBndl, isNew := StoreBundleIfAbsent(bndl) // bundle +1
	if !isNew {
		return
	}
	defer cachedBndl.Release() // bundle -1

	if !cachedBndl.GetBundle().validate() {
		return
	}
	cachedBndl.GetBundle().calcLedgerChanges()

	if !cachedBndl.GetBundle().IsValueSpam() {
		for addr, change := range cachedBndl.GetBundle().GetLedgerChanges() {
			if change < 0 {
				Events.AddressSpent.Trigger(addr)

				// ToDo:
				//markedSpentAddrs.Inc()
			}
		}
		return
	}

	if IsMaybeMilestone(cachedBndl.GetBundle().GetTail()) { // tx pass +1
		isMilestone, err := CheckIfMilestone(cachedBndl.Retain()) // bundle pass +1
		if err != nil {
			Events.ReceivedInvalidMilestone.Trigger(fmt.Errorf("Invalid milestone detected! Err: %s", err.Error()))
		} else if isMilestone {
			StoreMilestone(cachedBndl.Retain()).Release()     // bundle pass +1
			Events.ReceivedValidMilestone.Trigger(cachedBndl) // bundle pass +1
		}
	}
}

// Remaps transactions into the given bundle by traversing from the given start transaction through the trunk.
func constructBundle(bndl *Bundle, cachedStartTx *CachedTransaction) bool {

	cachedCurrentTx := cachedStartTx

	// iterate as long as the bundle isn't complete and prevent cyclic transactions (such as the genesis)
	for cachedCurrentTx.GetTransaction().GetHash() != cachedCurrentTx.GetTransaction().GetTrunk() && !bndl.isComplete() && !cachedCurrentTx.GetTransaction().IsHead() {

		// check whether the trunk transaction is known to the transaction storage.
		if !ContainsTransaction(cachedCurrentTx.GetTransaction().GetTrunk()) {
			cachedCurrentTx.Release() // tx -1
			return false
		}

		trunkTx := loadBundleTxIfExistsOrPanic(cachedCurrentTx.GetTransaction().GetTrunk(), bndl.hash) // tx +1

		// check whether trunk is in bundle instance already
		if _, trunkAlreadyInBundle := bndl.txs[cachedCurrentTx.GetTransaction().GetTrunk()]; trunkAlreadyInBundle {
			cachedCurrentTx.Release() // tx -1
			cachedCurrentTx = trunkTx
			continue
		}

		if trunkTx.GetTransaction().Tx.Bundle != cachedStartTx.GetTransaction().Tx.Bundle {
			trunkTx.Release() // tx -1

			// Tx has invalid structure, but is "complete"
			break
		}

		// assign as head if last tx
		if trunkTx.GetTransaction().IsHead() {
			bndl.headTx = trunkTx.GetTransaction().GetHash()
		}

		// assign trunk tx to this bundle
		bndl.txs[trunkTx.GetTransaction().GetHash()] = struct{}{}

		// modify and advance to perhaps complete the bundle
		bndl.SetModified(true)
		cachedCurrentTx.Release() // tx -1
		cachedCurrentTx = trunkTx
	}

	cachedCurrentTx.Release() // tx -1
	return true
}

// Create a new bundle instance as soon as a tailTx gets solid
func OnTailTransactionSolid(cachedTx *CachedTransaction) {
	tryConstructBundle(cachedTx, true) // tx +-0 (it has +1 and will be released in tryConstructBundle)
}
