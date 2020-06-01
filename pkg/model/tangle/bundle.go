package tangle

import (
	"bytes"
	"log"
	"sync"

	iotagobundle "github.com/iotaledger/iota.go/bundle"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

func BundleCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedBndl *CachedBundle))(params[0].(*CachedBundle).Retain())
}

const (
	MetadataSolid                = 0
	MetadataValid                = 1
	MetadataConfirmed            = 2
	MetadataIsMilestone          = 3
	MetadataIsValueSpam          = 4
	MetadataValidStrictSemantics = 5
)

// Storable Object
type Bundle struct {
	objectstorage.StorableObjectFlags
	syncutils.RWMutex

	// Key
	tailTx hornet.Hash

	// Value
	metadata      bitmask.BitMask
	lastIndex     uint64
	hash          hornet.Hash
	headTx        hornet.Hash
	txs           map[string]struct{}
	ledgerChanges map[string]int64

	milestoneIndexOnce sync.Once
	milestoneIndex     milestone.Index
}

func (bundle *Bundle) GetBundleHash() hornet.Hash {
	return bundle.hash
}

func (bundle *Bundle) GetTrunk(forceRelease bool) hornet.Hash {
	cachedHeadTx := bundle.GetHead()         // tx +1
	defer cachedHeadTx.Release(forceRelease) // tx -1
	return cachedHeadTx.GetTransaction().GetTrunkHash()
}

func (bundle *Bundle) GetBranch(forceRelease bool) hornet.Hash {
	cachedHeadTx := bundle.GetHead()         // tx +1
	defer cachedHeadTx.Release(forceRelease) // tx -1
	return cachedHeadTx.GetTransaction().GetBranchHash()
}

func (bundle *Bundle) GetLedgerChanges() map[string]int64 {
	return bundle.ledgerChanges
}

func (bundle *Bundle) GetHead() *CachedTransaction {
	if len(bundle.headTx) == 0 {
		panic("head hash can never be empty")
	}

	return loadBundleTxIfExistsOrPanic(bundle.headTx, bundle.hash) // tx +1
}

func (bundle *Bundle) GetTailHash() hornet.Hash {
	if len(bundle.tailTx) == 0 {
		panic("tail hash can never be empty")
	}

	return bundle.tailTx
}

func (bundle *Bundle) GetTail() *CachedTransaction {
	if len(bundle.tailTx) == 0 {
		panic("tail hash can never be empty")
	}

	return loadBundleTxIfExistsOrPanic(bundle.tailTx, bundle.hash) // tx +1
}

func (bundle *Bundle) GetTxHashes() hornet.Hashes {

	var values hornet.Hashes
	for txHash := range bundle.txs {
		values = append(values, hornet.Hash(txHash))
	}

	return values
}

func (bundle *Bundle) GetTransactions() CachedTransactions {

	var cachedTxs CachedTransactions
	for txHash := range bundle.txs {
		tx := loadBundleTxIfExistsOrPanic(hornet.Hash(txHash), bundle.hash) // tx +1
		cachedTxs = append(cachedTxs, tx)
	}

	return cachedTxs
}

func (bundle *Bundle) setSolid(solid bool) {
	if solid != bundle.metadata.HasFlag(MetadataSolid) {
		bundle.metadata = bundle.metadata.ModifyFlag(MetadataSolid, solid)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) IsSolid() bool {

	solid := bundle.metadata.HasFlag(MetadataSolid)

	if solid {
		return true
	}

	// Check tail tx
	cachedTailTx := bundle.GetTail() // tx +1
	tailSolid := cachedTailTx.GetMetadata().IsSolid()
	cachedTailTx.Release(true) // tx -1

	if tailSolid {
		bundle.setSolid(true)
	}

	return tailSolid
}

func (bundle *Bundle) setValid(valid bool) {
	if valid != bundle.metadata.HasFlag(MetadataValid) {
		bundle.metadata = bundle.metadata.ModifyFlag(MetadataValid, valid)
	}
}

func (bundle *Bundle) IsValid() bool {
	return bundle.metadata.HasFlag(MetadataValid)
}

func (bundle *Bundle) setValidStrictSemantics(valid bool) {
	if valid != bundle.metadata.HasFlag(MetadataValidStrictSemantics) {
		bundle.metadata = bundle.metadata.ModifyFlag(MetadataValidStrictSemantics, valid)
	}
}

func (bundle *Bundle) ValidStrictSemantics() bool {
	return bundle.metadata.HasFlag(MetadataValidStrictSemantics)
}

func (bundle *Bundle) setConfirmed(confirmed bool) {
	if confirmed != bundle.metadata.HasFlag(MetadataConfirmed) {
		bundle.metadata = bundle.metadata.ModifyFlag(MetadataConfirmed, confirmed)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) IsConfirmed() bool {

	confirmed := bundle.metadata.HasFlag(MetadataConfirmed)

	if confirmed {
		return true
	}

	// Check tail tx
	cachedTailTx := bundle.GetTail() // tx +1
	defer cachedTailTx.Release(true) // tx -1
	tailConfirmed := cachedTailTx.GetMetadata().IsConfirmed()

	if tailConfirmed {
		bundle.setConfirmed(true)
	}

	return tailConfirmed
}

func (bundle *Bundle) setValueSpam(valueSpam bool) {
	if valueSpam != bundle.metadata.HasFlag(MetadataIsValueSpam) {
		bundle.metadata = bundle.metadata.ModifyFlag(MetadataIsValueSpam, valueSpam)
	}
}

func (bundle *Bundle) IsValueSpam() bool {
	return bundle.metadata.HasFlag(MetadataIsValueSpam)
}

func (bundle *Bundle) GetMetadata() byte {
	bundle.RLock()
	defer bundle.RUnlock()
	return byte(bundle.metadata)
}

func (bundle *Bundle) ApplySpentAddresses() {
	if !bundle.IsValueSpam() {
		spentAddressesEnabled := GetSnapshotInfo().IsSpentAddressesEnabled()
		for addr, change := range bundle.GetLedgerChanges() {
			if change < 0 {
				if spentAddressesEnabled && MarkAddressAsSpent(hornet.Hash(addr)) {
					metrics.SharedServerMetrics.SeenSpentAddresses.Inc()
				}
				Events.AddressSpent.Trigger(hornet.Hash(addr).Trytes())
			}
		}
	}
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
	iotaGoBundle := make(iotagobundle.Bundle, len(bundle.txs))

	cachedCurrentTailTx := loadBundleTxIfExistsOrPanic(bundle.tailTx, bundle.hash) // tx +1
	lastIndex := int(cachedCurrentTailTx.GetTransaction().Tx.LastIndex)
	iotaGoBundle[0] = *cachedCurrentTailTx.GetTransaction().Tx
	defer cachedCurrentTailTx.Release(true) // tx -1

	cachedCurrentTx := cachedCurrentTailTx
	headTx := *cachedCurrentTx.GetTransaction()
	for i := 1; i < lastIndex+1; i++ {
		cachedCurrentTx = loadBundleTxIfExistsOrPanic(cachedCurrentTx.GetTransaction().GetTrunkHash(), bundle.hash) // tx +1
		iotaGoBundle[i] = *cachedCurrentTx.GetTransaction().Tx
		if i == lastIndex {
			headTx = *cachedCurrentTx.GetTransaction()
		}
		cachedCurrentTx.Release(true) // tx -1
	}

	// validate bundle semantics and signatures
	if iotagobundle.ValidBundle(iotaGoBundle) != nil {
		bundle.setValid(false)
		bundle.setValidStrictSemantics(false)
		return false
	}

	validStrictSemantics := true

	// enforce that non head transactions within the bundle approve as their branch transaction
	// the trunk transaction of the head transaction.
	// Milestones already follow these rules.
	if !bundle.IsMilestone() {
		if len(iotaGoBundle) > 1 {
			for i := 0; i < len(iotaGoBundle)-1; i++ {
				if iotaGoBundle[i].BranchTransaction != headTx.Tx.TrunkTransaction {
					validStrictSemantics = false
				}
			}
		}
	}

	// verify that the head transaction only approves tail transactions.
	// this is fine within the validation code, since the bundle is only complete when it is solid.
	// however, as a special rule, milestone bundles might not be solid
	if !bundle.IsMilestone() && validStrictSemantics {
		approveeHashes := hornet.Hashes{headTx.GetTrunkHash()}
		if !bytes.Equal(headTx.GetTrunkHash(), headTx.GetBranchHash()) {
			approveeHashes = append(approveeHashes, headTx.GetBranchHash())
		}

		for _, approveeHash := range approveeHashes {
			if SolidEntryPointsContain(approveeHash) {
				continue
			}
			cachedApproveeTx := GetCachedTransactionOrNil(approveeHash) // tx +1
			if cachedApproveeTx == nil {
				log.Panicf("Tx with hash %v not found", approveeHash.Trytes())
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

	changes := map[string]int64{}
	for txHash := range bundle.txs {
		cachedTx := loadBundleTxIfExistsOrPanic(hornet.Hash(txHash), bundle.hash) // tx +1
		if value := cachedTx.GetTransaction().Tx.Value; value != 0 {
			changes[string(cachedTx.GetTransaction().GetAddress())] += value
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

func loadBundleTxIfExistsOrPanic(txHash hornet.Hash, bundleHash hornet.Hash) *CachedTransaction {
	cachedTx := GetCachedTransactionOrNil(txHash) // tx +1
	if cachedTx == nil {
		log.Panicf("bundle %s has a reference to a non persisted transaction: %s", bundleHash.Trytes(), txHash.Trytes())
	}
	return cachedTx
}
