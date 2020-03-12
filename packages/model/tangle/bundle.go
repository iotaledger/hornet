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

const (
	HORNET_BUNDLE_METADATA_SOLID                  = 0
	HORNET_BUNDLE_METADATA_VALID                  = 1
	HORNET_BUNDLE_METADATA_CONFIRMED              = 2
	HORNET_BUNDLE_METADATA_IS_MILESTONE           = 3
	HORNET_BUNDLE_METADATA_IS_VALUE_SPAM          = 4
	HORNET_BUNDLE_METADATA_VALID_STRICT_SEMANTICS = 5
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
	return bundle.hash
}

func (bundle *Bundle) GetTrunk(forceRelease bool) trinary.Hash {
	cachedHeadTx := bundle.getHead()         // tx +1
	defer cachedHeadTx.Release(forceRelease) // tx -1
	return cachedHeadTx.GetTransaction().GetTrunk()
}

func (bundle *Bundle) GetBranch(forceRelease bool) trinary.Hash {
	cachedHeadTx := bundle.getHead()         // tx +1
	defer cachedHeadTx.Release(forceRelease) // tx -1
	return cachedHeadTx.GetTransaction().GetBranch()
}

func (bundle *Bundle) GetLedgerChanges() map[trinary.Trytes]int64 {
	return bundle.ledgerChanges
}

func (bundle *Bundle) getHead() *CachedTransaction {

	if len(bundle.headTx) == 0 {
		panic("head hash can never be empty")
	}

	return loadBundleTxIfExistsOrPanic(bundle.headTx, bundle.hash) // tx +1
}

func (bundle *Bundle) GetHead() *CachedTransaction {
	return bundle.getHead()
}

func (bundle *Bundle) GetTailHash() trinary.Hash {
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
	return bundle.getTail()
}

func (bundle *Bundle) GetTransactionHashes() []trinary.Hash {

	var values []trinary.Hash
	for txHash := range bundle.txs {
		values = append(values, txHash)
	}

	return values
}

func (bundle *Bundle) GetTransactions() CachedTransactions {

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

	solid := bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_SOLID)

	if solid {
		return true
	}

	// Check tail tx
	cachedTailTx := bundle.getTail() // tx +1
	tailSolid := cachedTailTx.GetMetadata().IsSolid()
	cachedTailTx.Release(true) // tx -1

	if tailSolid {
		bundle.setSolid(true)
	}

	return tailSolid
}

func (bundle *Bundle) setValid(valid bool) {
	if valid != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_VALID) {
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_VALID, valid)
	}
}

func (bundle *Bundle) IsValid() bool {
	return bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_VALID)
}

func (bundle *Bundle) setValidStrictSemantics(valid bool) {
	if valid != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_VALID_STRICT_SEMANTICS) {
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_VALID_STRICT_SEMANTICS, valid)
	}
}

func (bundle *Bundle) ValidStrictSemantics() bool {
	return bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_VALID_STRICT_SEMANTICS)
}

func (bundle *Bundle) setConfirmed(confirmed bool) {
	if confirmed != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_CONFIRMED) {
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_CONFIRMED, confirmed)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) IsConfirmed() bool {

	confirmed := bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_CONFIRMED)

	if confirmed {
		return true
	}

	// Check tail tx
	cachedTailTx := bundle.getTail() // tx +1
	defer cachedTailTx.Release(true) // tx -1
	tailConfirmed, _ := cachedTailTx.GetMetadata().GetConfirmed()

	if tailConfirmed {
		bundle.setConfirmed(true)
	}

	return tailConfirmed
}

func (bundle *Bundle) setValueSpam(valueSpam bool) {
	if valueSpam != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_IS_VALUE_SPAM) {
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_IS_VALUE_SPAM, valueSpam)
	}
}

func (bundle *Bundle) IsValueSpam() bool {
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

	// Because the bundle is already complete when this function gets called, the amount of tx has to be correct,
	// otherwise the bundle was not constructed correctly
	if !bundle.isComplete() {
		bundle.setValid(false)
		return false
	}

	// check all tx
	iotaGoBundle := make(iotago_bundle.Bundle, len(bundle.txs))

	cachedCurrentTailTx := loadBundleTxIfExistsOrPanic(bundle.tailTx, bundle.hash) // tx +1
	lastIndex := int(cachedCurrentTailTx.GetTransaction().Tx.LastIndex)
	iotaGoBundle[0] = *cachedCurrentTailTx.GetTransaction().Tx
	defer cachedCurrentTailTx.Release(true) // tx -1

	cachedCurrentTx := cachedCurrentTailTx
	for i := 1; i < lastIndex+1; i++ {
		cachedCurrentTx = loadBundleTxIfExistsOrPanic(cachedCurrentTx.GetTransaction().GetTrunk(), bundle.hash) // tx +1
		iotaGoBundle[i] = *cachedCurrentTx.GetTransaction().Tx
		cachedCurrentTx.Release(true) // tx -1
	}

	// validate bundle semantics and signatures
	if iotago_bundle.ValidBundle(iotaGoBundle) != nil {
		bundle.setValid(false)
		bundle.setValidStrictSemantics(false)
		return false
	}

	validStrictSemantics := true

	// verify that the head transaction only approves tail transactions.
	// this is fine within the validation code, since the bundle is only complete when it is solid.
	// however, as a special rule, milestone bundles might not be solid
	checkTails := !bundle.IsMilestone()

	// enforce that non head transactions within the bundle approve as their branch transaction
	// the trunk transaction of the head transaction.
	headTx := iotaGoBundle[len(iotaGoBundle)-1]
	if len(iotaGoBundle) > 1 {
		for i := 0; i < len(iotaGoBundle)-1; i++ {
			if iotaGoBundle[i].BranchTransaction != headTx.TrunkTransaction {
				validStrictSemantics = false
			}
		}
	}

	// check whether the bundle approves tails
	if checkTails {
		approveeHashes := []trinary.Hash{headTx.TrunkTransaction}
		if headTx.TrunkTransaction != headTx.BranchTransaction {
			approveeHashes = append(approveeHashes, headTx.BranchTransaction)
		}

		for _, approveeHash := range approveeHashes {
			if SolidEntryPointsContain(approveeHash) {
				continue
			}
			cachedApproveeTx := GetCachedTransactionOrNil(approveeHash) // tx +1
			if cachedApproveeTx == nil {
				log.Panicf("Tx with hash %v not found", approveeHash)
			}

			if !cachedApproveeTx.GetTransaction().IsTail() {
				validStrictSemantics = false
				cachedApproveeTx.Release(true) // tx -1
				break
			}
			cachedApproveeTx.Release(true) // tx -1
		}
	}

	bundle.setValidStrictSemantics(validStrictSemantics)
	bundle.setValid(true)
	return true
}

// Calculates the ledger changes of the bundle
func (bundle *Bundle) calcLedgerChanges() {

	changes := map[trinary.Trytes]int64{}
	for txHash := range bundle.txs {
		cachedTx := loadBundleTxIfExistsOrPanic(txHash, bundle.hash) // tx +1
		if value := cachedTx.GetTransaction().Tx.Value; value != 0 {
			changes[cachedTx.GetTransaction().Tx.Address] += value
		}
		cachedTx.Release(true) // tx -1
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

func loadBundleTxIfExistsOrPanic(txHash trinary.Hash, bundleHash trinary.Hash) *CachedTransaction {
	cachedTx := GetCachedTransactionOrNil(txHash) // tx +1
	if cachedTx == nil {
		log.Panicf("bundle %s has a reference to a non persisted transaction: %s", bundleHash, txHash)
	}
	return cachedTx
}
