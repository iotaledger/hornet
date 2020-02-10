package tangle

import (
	"log"

	iotago_bundle "github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"
)

func BundleCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedBndl *CachedBundle))(params[0].(*CachedBundle).Retain())
}

func BundlesCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedBndls CachedBundles))(params[0].(CachedBundles).Retain())
}

const (
	HORNET_BUNDLE_METADATA_SOLID         = 0
	HORNET_BUNDLE_METADATA_VALID         = 1
	HORNET_BUNDLE_METADATA_CONFIRMED     = 2
	HORNET_BUNDLE_METADATA_IS_MILESTONE  = 3
	HORNET_BUNDLE_METADATA_IS_VALUE_SPAM = 4
)

// Storable Object
type Bundle struct {
	objectstorage.StorableObjectFlags
	syncutils.RWMutex

	// Key
	tailTx trinary.Hash

	// Value
	metadata      bitmask.BitMask
	lastIndex     uint64
	hash          trinary.Hash
	headTx        trinary.Hash
	txs           map[trinary.Hash]struct{}
	ledgerChanges map[trinary.Trytes]int64
}

func (bundle *Bundle) GetHash() trinary.Hash {
	bundle.RLock()
	defer bundle.RUnlock()
	return bundle.hash
}

func (bundle *Bundle) GetTrunk() trinary.Hash {
	bundle.RLock()
	defer bundle.RUnlock()

	cachedHeadTx := bundle.getHead() // tx +1
	defer cachedHeadTx.Release()     // tx -1
	return cachedHeadTx.GetTransaction().GetTrunk()
}

func (bundle *Bundle) GetBranch() trinary.Hash {
	bundle.RLock()
	defer bundle.RUnlock()

	cachedHeadTx := bundle.getHead() // tx +1
	defer cachedHeadTx.Release()     // tx -1
	return cachedHeadTx.GetTransaction().GetBranch()
}

func (bundle *Bundle) GetLedgerChanges() map[trinary.Trytes]int64 {
	bundle.RLock()
	defer bundle.RUnlock()
	return bundle.ledgerChanges
}

func (bundle *Bundle) getHead() *CachedTransaction {

	if len(bundle.headTx) == 0 {
		panic("head hash can never be empty")
	}

	return loadBundleTxIfExistsOrPanic(bundle.headTx, bundle.hash) // tx +1
}

func (bundle *Bundle) GetHead() *CachedTransaction {
	bundle.RLock()
	defer bundle.RUnlock()

	return bundle.getHead()
}

func (bundle *Bundle) GetTailHash() trinary.Hash {
	bundle.RLock()
	defer bundle.RUnlock()

	if len(bundle.tailTx) == 0 {
		panic("tail hash can never be empty")
	}

	return bundle.tailTx
}

func (bundle *Bundle) getTail() *CachedTransaction {

	if len(bundle.tailTx) == 0 {
		panic("tail hash can never be empty")
	}

	return loadBundleTxIfExistsOrPanic(bundle.tailTx, bundle.hash) // tx +1
}

func (bundle *Bundle) GetTail() *CachedTransaction {
	bundle.RLock()
	defer bundle.RUnlock()

	return bundle.getTail()
}

func (bundle *Bundle) GetTransactionHashes() []trinary.Hash {
	bundle.RLock()
	defer bundle.RUnlock()

	var values []trinary.Hash
	for txHash := range bundle.txs {
		values = append(values, txHash)
	}

	return values
}

func (bundle *Bundle) GetTransactions() CachedTransactions {
	bundle.RLock()
	defer bundle.RUnlock()

	var cachedTxs CachedTransactions
	for txHash := range bundle.txs {
		tx := loadBundleTxIfExistsOrPanic(txHash, bundle.hash) // tx +1
		cachedTxs = append(cachedTxs, tx)
	}

	return cachedTxs
}

func (bundle *Bundle) setSolid(solid bool) {
	if solid != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_SOLID) {
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_SOLID, solid)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) IsSolid() bool {
	bundle.RLock()
	defer bundle.RUnlock()

	solid := bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_SOLID)

	if solid {
		return true
	}

	// Check tail tx
	cachedTailTx := bundle.getTail() // tx +1
	tailSolid := cachedTailTx.GetTransaction().IsSolid()
	cachedTailTx.Release() // tx -1

	if tailSolid {
		bundle.setSolid(true)
	}

	return tailSolid
}

func (bundle *Bundle) setValid(valid bool) {
	if valid != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_VALID) {
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_VALID, valid)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) IsValid() bool {
	bundle.RLock()
	defer bundle.RUnlock()
	return bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_VALID)
}

func (bundle *Bundle) setConfirmed(confirmed bool) {
	if confirmed != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_CONFIRMED) {
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_CONFIRMED, confirmed)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) IsConfirmed() bool {
	bundle.RLock()
	defer bundle.RUnlock()

	confirmed := bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_CONFIRMED)

	if confirmed {
		return true
	}

	// Check tail tx
	cachedTailTx := bundle.getTail() // tx +1
	defer cachedTailTx.Release()     // tx -1
	tailConfirmed, _ := cachedTailTx.GetTransaction().GetConfirmed()

	if tailConfirmed {
		bundle.setConfirmed(true)
	}

	return tailConfirmed
}

func (bundle *Bundle) setValueSpam(valueSpam bool) {
	if valueSpam != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_IS_VALUE_SPAM) {
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_IS_VALUE_SPAM, valueSpam)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) IsValueSpam() bool {
	bundle.RLock()
	defer bundle.RUnlock()
	return bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_IS_VALUE_SPAM)
}

func (bundle *Bundle) GetMetadata() byte {
	bundle.RLock()
	defer bundle.RUnlock()
	return byte(bundle.metadata)
}

////////////////////////////////////////////////////////////////////////////////

// Checks if a bundle is complete
func (bundle *Bundle) isComplete() bool {
	return uint64(len(bundle.txs)) == bundle.lastIndex+1
}

// Checks if a bundle is syntactically valid and has valid signatures
func (bundle *Bundle) validate() bool {
	bundle.Lock()
	defer bundle.Unlock()

	// Because the bundle is already complete when this function gets called, the amount of tx has to be correct,
	// otherwise the bundle was not constructed correctly
	if !bundle.isComplete() {
		bundle.setValid(false)
		return false
	}

	// check all tx
	iotaGoBundle := make(iotago_bundle.Bundle, len(bundle.txs))

	cachedCurrentTx := loadBundleTxIfExistsOrPanic(bundle.tailTx, bundle.hash) // tx +1
	lastIndex := int(cachedCurrentTx.GetTransaction().Tx.LastIndex)
	iotaGoBundle[0] = *cachedCurrentTx.GetTransaction().Tx
	cachedCurrentTx.Release() // tx -1

	for i := 1; i < lastIndex+1; i++ {
		cachedCurrentTx = loadBundleTxIfExistsOrPanic(cachedCurrentTx.GetTransaction().GetTrunk(), bundle.hash) // tx +1
		iotaGoBundle[i] = *cachedCurrentTx.GetTransaction().Tx
		cachedCurrentTx.Release() // tx -1
	}

	// validate bundle semantics and signatures
	valid := iotago_bundle.ValidBundle(iotaGoBundle) == nil

	bundle.setValid(valid)
	return valid
}

// Calculates the ledger changes of the bundle
func (bundle *Bundle) calcLedgerChanges() {
	bundle.Lock()
	defer bundle.Unlock()

	changes := map[trinary.Trytes]int64{}
	for txHash := range bundle.txs {
		cachedTx := loadBundleTxIfExistsOrPanic(txHash, bundle.hash) // tx +1
		if cachedTx.GetTransaction().Tx.Value == 0 {
			cachedTx.Release() // tx -1
			continue
		}
		changes[cachedTx.GetTransaction().Tx.Address] += cachedTx.GetTransaction().Tx.Value
		cachedTx.Release() // tx -1
	}

	isValueSpamBundle := true
	for _, change := range changes {
		if change != 0 {
			isValueSpamBundle = false
			break
		}
	}

	bundle.ledgerChanges = changes
	bundle.setValueSpam(isValueSpamBundle)
}

////////////////////////////////////////////////////////////////////////////////

func loadBundleTxIfExistsOrNil(txHash trinary.Hash, bundleHash trinary.Hash) *CachedTransaction {
	cachedTx := GetCachedTransaction(txHash) // tx +1
	if !cachedTx.Exists() {
		cachedTx.Release() // tx -1
		return nil
	}
	return cachedTx
}

func loadBundleTxIfExistsOrPanic(txHash trinary.Hash, bundleHash trinary.Hash) *CachedTransaction {
	cachedTx := GetCachedTransaction(txHash) // tx +1
	if !cachedTx.Exists() {
		log.Panicf("bundle %s has a reference to a non persisted transaction: %s", bundleHash, txHash)
	}
	return cachedTx
}
