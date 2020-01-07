package tangle

import (
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/syncutils"
	"log"

	iotago_bundle "github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/trinary"
)

func BundleCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx *Bundle))(params[0].(*Bundle))
}

func BundlesCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx []*Bundle))(params[0].([]*Bundle))
}

const (
	HORNET_BUNDLE_METADATA_SOLID        = 0
	HORNET_BUNDLE_METADATA_CONFIRMED    = 1
	HORNET_BUNDLE_METADATA_COMPLETE     = 2
	HORNET_BUNDLE_METADATA_VALID        = 3
	HORNET_BUNDLE_METADATA_CONFLICTING  = 4
	HORNET_BUNDLE_METADATA_IS_MILESTONE = 5
	HORNET_BUNDLE_METADATA_VALIDATED    = 6
)

// A BundleBucket is a container for Bundle instances which have the same bundle hash.
type BundleBucket struct {
	hash trinary.Hash
	mu   syncutils.RWMutex

	// all transactions which are mapped to this bundle hash
	txs map[trinary.Hash]struct{}

	// instances of bundles
	bundleInstances map[trinary.Hash]*Bundle
}

func (bucket *BundleBucket) Bundles() []*Bundle {
	bucket.mu.RLock()
	defer bucket.mu.RUnlock()
	bndls := make([]*Bundle, 0)
	for _, bndl := range bucket.bundleInstances {
		bndls = append(bndls, bndl)
	}
	return bndls
}

func (bucket *BundleBucket) Transactions() CachedTransactions {
	bucket.mu.RLock()
	defer bucket.mu.RUnlock()
	txs := make([]*CachedTransaction, 0)
	for txHash := range bucket.txs {
		// TODO: the transaction could have been pruned away?
		tx := GetCachedTransaction(txHash) //+1
		if !tx.Exists() {
			tx.Release() //-1
			continue
		}
		txs = append(txs, tx)
	}
	return txs
}

func (bucket *BundleBucket) TransactionHashes() []trinary.Hash {
	bucket.mu.RLock()
	defer bucket.mu.RUnlock()
	txHashes := make([]trinary.Hash, 0)
	for txHash := range bucket.txs {
		txHashes = append(txHashes, txHash)
	}
	return txHashes
}

// GetBundlesOfTransaction gets all bundle instances in which this transaction is present.
// A transaction can be in multiple bundle instances simultaneously
// due to the nature of reattached transactions being able to form infinite amount of bundles
// which attach to the same underlying bundle transaction. For example it is possible to reattach
// a bundle's tail transaction directly "on top" of the origin one.
func (bucket *BundleBucket) GetBundlesOfTransaction(txHash trinary.Hash) []*Bundle {
	bucket.mu.RLock()
	defer bucket.mu.RUnlock()

	bndls := make([]*Bundle, 0)
	for _, bndl := range bucket.bundleInstances {
		if _, has := bndl.txs[txHash]; has {
			bndls = append(bndls, bndl)
		}
	}
	return bndls
}

// GetBundleOfTailTransaction gets the bundle this tail transaction is present in or nil.
func (bucket *BundleBucket) GetBundleOfTailTransaction(txHash trinary.Hash) *Bundle {
	bucket.mu.RLock()
	defer bucket.mu.RUnlock()
	bndl, ok := bucket.bundleInstances[txHash]
	if !ok {
		return nil
	}
	return bndl
}

// RemoveBundleByTailTxHash removes the bundle with the given tail transaction
// and also disassociates the transactions from the bucket if not used in another bundle.
func (bucket *BundleBucket) RemoveBundleByTailTxHash(txHash trinary.Hash) {
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	bndl, ok := bucket.bundleInstances[txHash]
	if !ok {
		return
	}

	txsToRemove := map[string]struct{}{}
	for bundleTxHash := range bndl.txs {
		// check if the txs of this bundle are used in another bundle instance
		contains := false

		for tailTxHash, bundle := range bucket.bundleInstances {
			if tailTxHash == txHash {
				// It's the same bundle => skip
				continue
			}

			if _, has := bundle.txs[bundleTxHash]; has {
				contains = true
				break
			}
		}

		if !contains {
			txsToRemove[bundleTxHash] = struct{}{}
		}
	}

	// remove the bundle by first removing its corresponding transactions
	// from the "all" transaction set and then removing the actual bundle instance.
	for txHash := range txsToRemove {
		delete(bucket.txs, txHash)
	}
	delete(bucket.bundleInstances, txHash)
}

// Remaps transactions into the given bundle by traversing from the given start transaction through the trunk.
func (bucket *BundleBucket) remap(bndl *Bundle, startTx *CachedTransaction, onMapped ...func(mapped *CachedTransaction)) {
	bndl.txsMu.Lock()
	defer bndl.txsMu.Unlock()

	// This will be released while or after the loop as current
	startTx.RegisterConsumer() //+1

	current := startTx
	// iterate as long as the bundle isn't complete and prevent cyclic transactions (such as the genesis)
	for current.GetTransaction().GetHash() != current.GetTransaction().GetTrunk() && !bndl.isComplete() && !current.GetTransaction().IsHead() {

		// check whether the trunk transaction is known to the bucket.
		// this also ensures that the transaction has to be in the database
		if _, ok := bucket.txs[current.GetTransaction().GetTrunk()]; !ok {
			break
		}

		// check whether trunk is in bundle instance already
		if _, trunkAlreadyInBundle := bndl.txs[current.GetTransaction().GetTrunk()]; trunkAlreadyInBundle {
			trunkTx := loadBundleTxIfExistsOrPanic(current.GetTransaction().GetTrunk(), bndl.hash) //+1
			current.Release()                                                                      //-1
			current = trunkTx
			continue
		}

		trunkTx := loadBundleTxIfExistsOrPanic(current.GetTransaction().GetTrunk(), bndl.hash) //+1
		if trunkTx.GetTransaction().Tx.Bundle != startTx.GetTransaction().Tx.Bundle {
			trunkTx.Release() //-1
			break
		}

		// assign as head if last tx
		if trunkTx.GetTransaction().IsHead() {
			bndl.headTx = trunkTx.GetTransaction().GetHash()
		}

		// assign trunk tx to this bundle
		bndl.txs[trunkTx.GetTransaction().GetHash()] = struct{}{}

		// call closure
		if len(onMapped) > 0 {
			trunkTx.RegisterConsumer() //+1
			onMapped[0](trunkTx)
			trunkTx.Release() //-1
		}

		// modify and advance to perhaps complete the bundle
		bndl.SetModified(true)
		current.Release() //-1
		current = trunkTx
	}

	current.Release() //-1
}

// Returns the hash of the bundle the bucket is managing.
func (bucket *BundleBucket) GetHash() trinary.Hash {
	return bucket.hash
}

// Adds a new transaction to the BundleBucket by either creating a new Bundle instance,
// assigning the transaction to an existing Bundle or to the unassigned pool.
// It returns a slice of Bundles to which the transaction was added to. Adding a tail
// transaction will ever only return one Bundle within the slice.
func (bucket *BundleBucket) AddTransaction(tx *CachedTransaction) []*Bundle {
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	tx.RegisterConsumer() //+1
	defer tx.Release()    //-1

	// add the transaction to the "all" transactions pool
	bucket.txs[tx.GetTransaction().GetHash()] = struct{}{}

	if tx.GetTransaction().Tx.CurrentIndex == 0 {
		// don't need to do anything if the tail transaction already is indexed
		if bndl, ok := bucket.bundleInstances[tx.GetTransaction().GetHash()]; ok {
			return []*Bundle{bndl}
		}

		// create a new bundle instance
		bndl := &Bundle{
			txs:      make(map[trinary.Hash]struct{}),
			metadata: bitmask.BitMask(0),
			modified: true,
			hash:     tx.GetTransaction().Tx.Bundle,
			tailTx:   tx.GetTransaction().GetHash(),
		}
		bndl.txs[tx.GetTransaction().GetHash()] = struct{}{}
		bndl.lastIndex = tx.GetTransaction().Tx.LastIndex

		// check whether it is a bundle with only one transaction
		if tx.GetTransaction().Tx.CurrentIndex == tx.GetTransaction().Tx.LastIndex {
			bndl.headTx = tx.GetTransaction().GetHash()
		} else {
			// lets try to complete the bundle by assigning txs into this bundle
			bucket.remap(bndl, tx)
		}

		// add the new bundle to the bucket
		bucket.bundleInstances[tx.GetTransaction().GetHash()] = bndl
		return []*Bundle{bndl}
	}

	// try a remap on every non complete bundle in the bucket.
	addedTo := make([]*Bundle, 0)
	for tailTxHash, bndl := range bucket.bundleInstances {

		// skip complete bundle
		if bndl.IsComplete() {
			continue
		}

		// load tail of bundle as a starting point for the remap
		current := GetCachedTransaction(tailTxHash) //+1

		if !current.Exists() {
			log.Panicf("Tx not found but it should be in storage: %s", tailTxHash)
		}

		// try to add the new transaction to the bundle
		bucket.remap(bndl, current, func(mapped *CachedTransaction) {
			if mapped.GetTransaction().GetHash() == tx.GetTransaction().GetHash() {
				addedTo = append(addedTo, bndl)
			}
		})

		current.Release() //-1
	}

	return addedTo
}

// Maps the given transactions to their corresponding bundle instances within the bucket.
func (bucket *BundleBucket) Init(txs map[trinary.Hash]*CachedTransaction, metaMap map[trinary.Hash]bitmask.BitMask) {
	if len(bucket.txs) > 0 || len(bucket.bundleInstances) > 0 {
		panic("Init called on a not new BundleBucket")
	}

	if len(txs) == 0 {
		return
	}

	// we don't lock in this function because it should be called only on a fresh Bucket

	// add all transactions to the bucket
	for txHash := range txs {
		bucket.txs[txHash] = struct{}{}
	}

	// go through each tail tx to create a bundle instance
	for _, tx := range txs {
		tx.RegisterConsumer() //+1
		if tx.GetTransaction().Tx.Bundle != bucket.hash {
			log.Fatalf("tx %s was stored for bundle %s, but its bundle hash is %s", tx.GetTransaction().GetHash(), bucket.hash, tx.GetTransaction().Tx.Bundle)
		}

		if !tx.GetTransaction().IsTail() {
			tx.Release() //-1
			continue
		}

		// meta map only holds actual metadata bitmasks for tail txs
		metadata := metaMap[tx.GetTransaction().GetHash()]
		bndl := &Bundle{
			txs:       make(map[trinary.Hash]struct{}),
			metadata:  metadata,
			modified:  false,
			hash:      bucket.hash,
			tailTx:    tx.GetTransaction().GetHash(),
			lastIndex: tx.GetTransaction().Tx.LastIndex,
		}

		bndl.txs[tx.GetTransaction().GetHash()] = struct{}{}

		// full bundle
		if tx.GetTransaction().IsHead() {
			bndl.headTx = tx.GetTransaction().GetHash()
		}

		// fill up this bundle with the transactions.
		// note that this is different than remap() as it ignores whether the bundle is complete
		current := tx
		current.RegisterConsumer() //+1
		for current.GetTransaction().GetHash() != current.GetTransaction().GetTrunk() && !current.GetTransaction().IsHead() {

			if _, ok := bucket.txs[current.GetTransaction().GetTrunk()]; !ok {
				break
			}

			trunkTx := loadBundleTxIfExistsOrPanic(current.GetTransaction().GetTrunk(), bndl.hash) //+1

			if trunkTx.GetTransaction().IsHead() {
				bndl.headTx = trunkTx.GetTransaction().GetHash()
			}

			bndl.txs[trunkTx.GetTransaction().GetHash()] = struct{}{}
			current.Release() //-1
			current = trunkTx
		}
		current.Release() //-1

		bucket.bundleInstances[tx.GetTransaction().GetHash()] = bndl
		tx.Release() //-1
	}

	// now pre compute properties about every bundle
	for _, bndl := range bucket.Bundles() {
		bndl.GetLedgerChanges()
		head := bndl.GetHead() //+1
		if head != nil {
			head.Release() //-1
		}
	}
}

func (bucket *BundleBucket) GetConfirmed() []*Bundle {
	bucket.mu.RLock()
	defer bucket.mu.RUnlock()
	confBndls := []*Bundle{}
	for _, bndl := range bucket.bundleInstances {
		if bndl.IsConfirmed() {
			confBndls = append(confBndls, bndl)
		}
	}
	return confBndls
}

func (bucket *BundleBucket) GetComplete() []*Bundle {
	bucket.mu.RLock()
	defer bucket.mu.RUnlock()
	complBndls := []*Bundle{}
	for _, bndl := range bucket.bundleInstances {
		if bndl.IsComplete() {
			complBndls = append(complBndls, bndl)
		}
	}
	return complBndls
}

func (bucket *BundleBucket) GetIncomplete() []*Bundle {
	bucket.mu.RLock()
	defer bucket.mu.RUnlock()
	incomplBndls := []*Bundle{}
	for _, bndl := range bucket.bundleInstances {
		if !bndl.IsComplete() {
			incomplBndls = append(incomplBndls, bndl)
		}
	}
	return incomplBndls
}

type Bundle struct {
	txsMu syncutils.RWMutex
	txs   map[trinary.Hash]struct{}

	// Metadata
	metadataMutex syncutils.RWMutex
	metadata      bitmask.BitMask

	// Status
	statusMutex syncutils.RWMutex
	modified    bool
	requested   bool

	// cached fields
	cachedFieldsMutex syncutils.RWMutex
	hash              trinary.Hash
	lastIndex         uint64
	tailTx            trinary.Hash
	headTx            trinary.Hash
	ledgerChanges     map[trinary.Trytes]int64
	isValueSpamBundle bool
}

func NewBundleBucket(bundleHash trinary.Hash, transactions map[trinary.Hash]*CachedTransaction) *BundleBucket {
	return newBundleBucket(bundleHash, transactions, nil)
}

func NewBundleBucketFromDatabase(bundleHash trinary.Hash, transactions map[trinary.Hash]*CachedTransaction, metaMap map[trinary.Hash]bitmask.BitMask) *BundleBucket {
	return newBundleBucket(bundleHash, transactions, metaMap)
}

func newBundleBucket(bundleHash trinary.Hash, transactions map[trinary.Hash]*CachedTransaction, metaMap map[trinary.Hash]bitmask.BitMask) *BundleBucket {

	bucket := &BundleBucket{
		bundleInstances: make(map[trinary.Hash]*Bundle),
		txs:             make(map[trinary.Hash]struct{}),
		hash:            bundleHash,
	}

	bucket.Init(transactions, metaMap)
	return bucket
}

func (bundle *Bundle) GetHash() trinary.Hash {
	if bundle.hash != "" {
		return bundle.hash
	}
	bundle.txsMu.RLock()
	defer bundle.txsMu.RUnlock()
	for txHash := range bundle.txs {
		tx := GetCachedTransaction(txHash) //+1
		if !tx.Exists() {
			tx.Release() //-1
			continue
		}

		bundle.hash = tx.GetTransaction().Tx.Bundle
		tx.Release() //-1
		return bundle.hash
	}
	panic("GetHash() called on a bundle without any transactions")
}

func (bundle *Bundle) GetLedgerChanges() (map[trinary.Trytes]int64, bool) {
	isComplete := bundle.IsComplete()
	bundle.txsMu.RLock()
	defer bundle.txsMu.RUnlock()

	if isComplete && bundle.ledgerChanges != nil {
		return bundle.ledgerChanges, bundle.isValueSpamBundle
	}

	changes := map[trinary.Trytes]int64{}
	for txHash := range bundle.txs {
		tx := loadBundleTxIfExistsOrPanic(txHash, bundle.hash) //+1
		if tx.GetTransaction().Tx.Value == 0 {
			tx.Release() //-1
			continue
		}
		changes[tx.GetTransaction().Tx.Address] += tx.GetTransaction().Tx.Value
		tx.Release() //-1
	}

	isValueSpamBundle := true
	for _, change := range changes {
		if change != 0 {
			isValueSpamBundle = false
			break
		}
	}

	// if the bundle was complete, we cache the changes
	// as they won't change anymore
	if isComplete {
		bundle.cachedFieldsMutex.Lock()
		bundle.ledgerChanges = changes
		bundle.isValueSpamBundle = isValueSpamBundle
		bundle.cachedFieldsMutex.Unlock()
	}

	return changes, isValueSpamBundle
}

func (bundle *Bundle) GetHead() *CachedTransaction {
	bundle.txsMu.RLock()
	defer bundle.txsMu.RUnlock()
	if bundle.headTx != "" {
		return loadBundleTxIfExistsOrNil(bundle.headTx, bundle.hash) //+1
	}

	for txHash := range bundle.txs {
		tx := loadBundleTxIfExistsOrPanic(txHash, bundle.hash) //+1
		if tx.GetTransaction().Tx.CurrentIndex == tx.GetTransaction().Tx.LastIndex {
			bundle.headTx = tx.GetTransaction().Tx.Hash
			return tx
		}
		tx.Release() //-1
	}
	return nil
}

func (bundle *Bundle) GetTailHash() trinary.Hash {
	bundle.txsMu.RLock()
	defer bundle.txsMu.RUnlock()
	if len(bundle.tailTx) == 0 {
		panic("tail hash can never be empty")
	}
	return bundle.tailTx
}

func (bundle *Bundle) GetTail() *CachedTransaction {
	bundle.txsMu.RLock()
	defer bundle.txsMu.RUnlock()
	if bundle.tailTx != "" {
		return loadBundleTxIfExistsOrNil(bundle.tailTx, bundle.hash) //+1
	}

	for txHash := range bundle.txs {
		tx := loadBundleTxIfExistsOrPanic(txHash, bundle.hash) //+1
		if tx.GetTransaction().Tx.CurrentIndex == 0 {
			bundle.headTx = tx.GetTransaction().Tx.Hash
			return tx
		}
		tx.Release() //-1
	}
	return nil
}

func (bundle *Bundle) GetTransactionHashes() []trinary.Hash {
	bundle.txsMu.RLock()
	defer bundle.txsMu.RUnlock()

	var values []trinary.Hash
	for txHash := range bundle.txs {
		values = append(values, txHash)
	}

	return values
}

func (bundle *Bundle) GetTransactions() CachedTransactions {
	bundle.txsMu.RLock()
	defer bundle.txsMu.RUnlock()

	var values CachedTransactions
	for txHash := range bundle.txs {
		tx := loadBundleTxIfExistsOrPanic(txHash, bundle.hash) //+1
		values = append(values, tx)
	}

	return values
}

func (bundle *Bundle) IsValid() bool {
	if !bundle.IsComplete() {
		return false
	}

	bundle.metadataMutex.RLock()
	valid := bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_VALID)
	validated := bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_VALIDATED)
	bundle.metadataMutex.RUnlock()

	if valid {
		return true
	}

	// we validated the bundle already but it is invalid, so
	// lets not recompute the bundle's validity
	if validated {
		return false
	}

	// check all tx
	bundle.txsMu.RLock()
	defer bundle.txsMu.RUnlock()

	iotaGoBundle := make(iotago_bundle.Bundle, len(bundle.txs))

	current := loadBundleTxIfExistsOrPanic(bundle.tailTx, bundle.hash) //+1
	lastIndex := int(current.GetTransaction().Tx.LastIndex)
	iotaGoBundle[0] = *current.GetTransaction().Tx
	current.Release() //-1

	for i := 1; i < lastIndex+1; i++ {
		current = loadBundleTxIfExistsOrPanic(current.GetTransaction().GetTrunk(), bundle.hash) //+1
		iotaGoBundle[i] = *current.GetTransaction().Tx
		current.Release() //-1
	}

	// validate bundle semantics and signatures
	err := iotago_bundle.ValidBundle(iotaGoBundle)
	if err != nil {
		bundle.setValid(false)
		return false
	}

	bundle.setValid(true)
	return true
}

func (bundle *Bundle) setValid(valid bool) {
	bundle.metadataMutex.Lock()
	defer bundle.metadataMutex.Unlock()

	if valid != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_VALID) {
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_VALID, valid)
		bundle.metadata = bundle.metadata.SetFlag(HORNET_BUNDLE_METADATA_VALIDATED)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) isComplete() bool {
	bundle.metadataMutex.RLock()
	complete := bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_COMPLETE)
	bundle.metadataMutex.RUnlock()

	if complete {
		return true
	}

	amount := uint64(len(bundle.txs))
	if amount == 0 {
		return false
	}

	if amount == bundle.lastIndex+1 {
		bundle.setComplete(true)
		return true
	}
	return false
}

func (bundle *Bundle) IsComplete() bool {
	bundle.txsMu.RLock()
	defer bundle.txsMu.RUnlock()
	return bundle.isComplete()
}

func (bundle *Bundle) setComplete(complete bool) {
	bundle.metadataMutex.Lock()
	defer bundle.metadataMutex.Unlock()

	if complete != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_COMPLETE) {
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_COMPLETE, complete)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) IsSolid() bool {
	if !bundle.IsComplete() {
		return false
	}

	bundle.metadataMutex.RLock()
	solid := bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_SOLID)
	bundle.metadataMutex.RUnlock()

	if !solid {
		tailTx := bundle.GetTail() //+1
		if tailTx == nil {
			return false
		}
		if tailTx.GetTransaction().IsSolid() {
			bundle.setSolid(true)
			tailTx.Release() //-1
			return true
		}
		tailTx.Release() //-1
		return false
	} else {
		return true
	}
}

func (bundle *Bundle) setSolid(solid bool) {
	bundle.metadataMutex.Lock()
	defer bundle.metadataMutex.Unlock()

	if solid != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_SOLID) {
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_SOLID, solid)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) IsConfirmed() bool {
	if !bundle.IsComplete() {
		return false
	}

	bundle.metadataMutex.RLock()
	confirmed := bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_CONFIRMED)
	bundle.metadataMutex.RUnlock()

	if !confirmed {
		// Check all tx
		bundle.txsMu.RLock()
		defer bundle.txsMu.RUnlock()

		for txHash := range bundle.txs {
			tx := loadBundleTxIfExistsOrPanic(txHash, bundle.hash) //+1
			if confirmed, _ := tx.GetTransaction().GetConfirmed(); confirmed {
				bundle.setConfirmed(true)
				tx.Release() //-1
				return true
			}
			tx.Release() //-1
		}
		return false
	} else {
		return true
	}
}

func (bundle *Bundle) setConfirmed(confirmed bool) {
	bundle.metadataMutex.Lock()
	defer bundle.metadataMutex.Unlock()

	if confirmed != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_CONFIRMED) {
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_CONFIRMED, confirmed)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) IsConflicting() bool {
	bundle.metadataMutex.RLock()
	defer bundle.metadataMutex.RUnlock()

	return bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_CONFLICTING)
}

func (bundle *Bundle) SetConflicting(conflicting bool) {
	bundle.metadataMutex.Lock()
	defer bundle.metadataMutex.Unlock()

	if conflicting != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_CONFLICTING) {
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_CONFLICTING, conflicting)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) GetMetadata() byte {
	bundle.metadataMutex.RLock()
	defer bundle.metadataMutex.RUnlock()

	return byte(bundle.metadata)
}

func (bundle *Bundle) IsModified() bool {
	bundle.statusMutex.RLock()
	defer bundle.statusMutex.RUnlock()

	return bundle.modified
}

func (bundle *Bundle) SetModified(modified bool) {
	bundle.statusMutex.Lock()
	defer bundle.statusMutex.Unlock()

	bundle.modified = modified
}

func (bundle *Bundle) WasRequested() bool {
	bundle.statusMutex.RLock()
	requested := bundle.requested
	bundle.statusMutex.RUnlock()

	if requested {
		return true
	}
	transactions := bundle.GetTransactions() //+1
	defer transactions.Release()             //-1

	for _, tx := range transactions {
		if tx.GetTransaction().IsRequested() {
			// No need to set modified flag, since it is only temporary
			bundle.statusMutex.Lock()
			bundle.requested = true
			bundle.statusMutex.Unlock()
			return true
		}
	}
	return false
}

func loadBundleTxIfExistsOrNil(txHash trinary.Hash, bundleHash trinary.Hash) *CachedTransaction {
	tx := GetCachedTransaction(txHash) //+1
	if !tx.Exists() {
		tx.Release() //-1
		return nil
	}
	return tx
}

func loadBundleTxIfExistsOrPanic(txHash trinary.Hash, bundleHash trinary.Hash) *CachedTransaction {
	tx := GetCachedTransaction(txHash) //+1
	if !tx.Exists() {
		log.Panicf("bundle %s has a reference to a non persisted transaction: %s", bundleHash, txHash)
	}
	return tx
}
