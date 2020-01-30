package tangle

import (
	"log"

	iotago_bundle "github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/syncutils"
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

type Bundle struct {
	txsMu syncutils.RWMutex
	txs   map[trinary.Hash]struct{}

	// Metadata
	metadataMutex syncutils.RWMutex
	metadata      bitmask.BitMask

	// Status
	statusMutex syncutils.RWMutex
	modified    bool

	// Requested
	requestedMutex syncutils.RWMutex
	requested      bool

	// cached fields
	cachedFieldsMutex syncutils.RWMutex
	hash              trinary.Hash
	lastIndex         uint64
	tailTx            trinary.Hash
	headTx            trinary.Hash
	ledgerChanges     map[trinary.Trytes]int64
	isValueSpamBundle bool
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
	bundle.requestedMutex.RLock()
	requested := bundle.requested
	bundle.requestedMutex.RUnlock()

	if requested {
		return true
	}
	transactions := bundle.GetTransactions() //+1
	defer transactions.Release()             //-1

	for _, tx := range transactions {
		if tx.GetTransaction().IsRequested() {
			// No need to set modified flag, since it is only temporary
			bundle.requestedMutex.Lock()
			bundle.requested = true
			bundle.requestedMutex.Unlock()
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
