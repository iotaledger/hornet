package tangle

import (
	"bytes"
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/profile"
)

const (
	BundleTxIsTail = 1
)

var (
	bundleTransactionsStorage *objectstorage.ObjectStorage
)

func databaseKeyPrefixForBundleHash(bundleHash hornet.Hash) []byte {
	return bundleHash
}

func databaseKeyForBundleTransaction(bundleHash hornet.Hash, txHash hornet.Hash, isTail bool) []byte {
	var isTailByte byte
	if isTail {
		isTailByte = BundleTxIsTail
	}

	result := append(databaseKeyPrefixForBundleHash(bundleHash), isTailByte)
	return append(result, txHash...)
}

func bundleTransactionFactory(key []byte) (objectstorage.StorableObject, int, error) {
	bundleTx := &BundleTransaction{
		BundleHash: make([]byte, 49),
		IsTail:     key[49] == BundleTxIsTail,
		TxHash:     make([]byte, 49),
	}
	copy(bundleTx.BundleHash, key[:49])
	copy(bundleTx.TxHash, key[50:])

	return bundleTx, 99, nil
}

func GetBundleTransactionsStorageSize() int {
	return bundleTransactionsStorage.GetSize()
}

func configureBundleTransactionsStorage(store kvstore.KVStore) {

	opts := profile.LoadProfile().Caches.BundleTransactions

	bundleTransactionsStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixBundleTransactions}),
		bundleTransactionFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(49, 1, 49), // BundleHash, IsTail, TxHash
		objectstorage.KeysOnly(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// Storable Object
type BundleTransaction struct {
	objectstorage.StorableObjectFlags

	// Key
	BundleHash []byte
	IsTail     bool
	TxHash     []byte
}

func (bt *BundleTransaction) GetTxHash() hornet.Hash {
	return bt.TxHash
}

func (bt *BundleTransaction) GetBundleHash() hornet.Hash {
	return bt.BundleHash
}

// ObjectStorage interface
func (bt *BundleTransaction) Update(_ objectstorage.StorableObject) {
	panic("BundleTransaction should never be updated")
}

func (bt *BundleTransaction) ObjectStorageKey() []byte {
	var isTailByte byte
	if bt.IsTail {
		isTailByte = BundleTxIsTail
	}

	result := append(bt.BundleHash, isTailByte)
	return append(result, bt.TxHash...)
}

func (bt *BundleTransaction) ObjectStorageValue() (_ []byte) {
	return nil
}

func (bt *BundleTransaction) UnmarshalObjectStorageValue(_ []byte) (consumedBytes int, err error) {
	return 0, nil
}

// Cached Object
type CachedBundleTransaction struct {
	objectstorage.CachedObject
}

type CachedBundleTransactions []*CachedBundleTransaction

func (cachedBundleTransactions CachedBundleTransactions) Retain() CachedBundleTransactions {
	cachedResult := CachedBundleTransactions{}
	for _, cachedBundleTransaction := range cachedBundleTransactions {
		cachedResult = append(cachedResult, cachedBundleTransaction.Retain().(*CachedBundleTransaction))
	}
	return cachedResult
}

func (cachedBundleTransactions CachedBundleTransactions) Release(force ...bool) {
	for _, cachedBundleTransaction := range cachedBundleTransactions {
		cachedBundleTransaction.Release(force...)
	}
}

func (c *CachedBundleTransaction) GetBundleTransaction() *BundleTransaction {
	return c.Get().(*BundleTransaction)
}

// bundleTx +1
func GetCachedBundleTransactionOrNil(bundleHash hornet.Hash, txHash hornet.Hash, isTail bool) *CachedBundleTransaction {
	cachedBundleTx := bundleTransactionsStorage.Load(databaseKeyForBundleTransaction(bundleHash, txHash, isTail)) // bundleTx +1
	if !cachedBundleTx.Exists() {
		cachedBundleTx.Release(true) // bundleTx -1
		return nil
	}
	return &CachedBundleTransaction{CachedObject: cachedBundleTx}
}

// bundleTx +-0
func GetBundleTransactionHashes(bundleHash hornet.Hash, forceRelease bool, maxFind ...int) hornet.Hashes {
	var bundleTransactionHashes hornet.Hashes

	i := 0
	bundleTransactionsStorage.ForEachKeyOnly(func(key []byte) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			return false
		}

		bundleTransactionHashes = append(bundleTransactionHashes, key[50:99])
		return true
	}, false, databaseKeyPrefixForBundleHash(bundleHash))

	return bundleTransactionHashes
}

// bundleTx +1
func GetBundleTailTransactionHashes(bundleHash hornet.Hash, forceRelease bool, maxFind ...int) hornet.Hashes {
	var bundleTransactionHashes hornet.Hashes

	i := 0
	bundleTransactionsStorage.ForEachKeyOnly(func(key []byte) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			return false
		}

		bundleTransactionHashes = append(bundleTransactionHashes, key[50:99])
		return true
	}, false, append(databaseKeyPrefixForBundleHash(bundleHash), BundleTxIsTail))

	return bundleTransactionHashes
}

// bundleTx +-0
func ContainsBundleTransaction(bundleHash hornet.Hash, txHash hornet.Hash, isTail bool) bool {
	return bundleTransactionsStorage.Contains(databaseKeyForBundleTransaction(bundleHash, txHash, isTail))
}

// bundleTx +1
func StoreBundleTransaction(bundleHash hornet.Hash, txHash hornet.Hash, isTail bool) *CachedBundleTransaction {

	bundleTx := &BundleTransaction{
		BundleHash: bundleHash,
		IsTail:     isTail,
		TxHash:     txHash,
	}

	cachedObj := bundleTransactionsStorage.ComputeIfAbsent(bundleTx.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject { // bundleTx +1
		bundleTx.Persist()
		bundleTx.SetModified()
		return bundleTx
	})

	return &CachedBundleTransaction{CachedObject: cachedObj}
}

// bundleTx +-0
func DeleteBundleTransaction(bundleHash hornet.Hash, txHash hornet.Hash, isTail bool) {
	bundleTransactionsStorage.Delete(databaseKeyForBundleTransaction(bundleHash, txHash, isTail))
}

func ShutdownBundleTransactionsStorage() {
	bundleTransactionsStorage.Shutdown()
}

func FlushBundleTransactionsStorage() {
	bundleTransactionsStorage.Flush()
}

////////////////////////////////////////////////////////////////////////////////

// getTailApproversOfSameBundle returns all tailTx hashes of the same bundle that approve this transaction
func getTailApproversOfSameBundle(bundleHash hornet.Hash, txHash hornet.Hash, forceRelease bool) hornet.Hashes {
	var tailTxHashes hornet.Hashes

	txsToCheck := make(map[string]struct{})
	txsToCheck[string(txHash)] = struct{}{}

	// Loop as long as new transactions are added in every loop cycle
	for len(txsToCheck) != 0 {
		for txHashToCheck := range txsToCheck {
			delete(txsToCheck, txHashToCheck)

			for _, approverHash := range GetApproverHashes(hornet.Hash(txHashToCheck), forceRelease) {
				cachedApproverTx := GetCachedTransactionOrNil(approverHash) // tx +1
				if cachedApproverTx == nil {
					continue
				}

				approverTx := cachedApproverTx.GetTransaction()
				if !bytes.Equal(approverTx.GetBundleHash(), bundleHash) {
					// Not the same bundle => skip
					cachedApproverTx.Release(forceRelease) // tx -1
					continue
				}

				if approverTx.IsTail() {
					// TailTx of the bundle
					tailTxHashes = append(tailTxHashes, approverHash)
				} else {
					// Not the tail, but in the same bundle => walk the future cone
					txsToCheck[string(approverHash)] = struct{}{}
				}

				cachedApproverTx.Release(forceRelease) // tx -1
			}
		}
	}

	return tailTxHashes
}

// approversFromSameBundleExist returns whether there are other transactions in the same bundle, that approve this transaction
func approversFromSameBundleExist(bundleHash hornet.Hash, txHash hornet.Hash, forceRelease bool) bool {

	for _, approverHash := range GetApproverHashes(txHash, forceRelease) {
		if ContainsBundleTransaction(bundleHash, approverHash, true) || ContainsBundleTransaction(bundleHash, approverHash, false) {
			// Tx is used in another bundle instance => do not delete
			return true
		}
	}

	return false
}

// RemoveTransactionFromBundle removes the transaction if non-tail and not associated to a bundle instance or
// if tail, it removes all the transactions of the bundle from the storage that are not used in another bundle instance.
func RemoveTransactionFromBundle(tx *hornet.Transaction) map[string]struct{} {

	txsToRemove := make(map[string]struct{})

	// check whether this transaction is a tail or respectively stored as a bundle tail
	isTail := ContainsBundleTransaction(tx.GetBundleHash(), tx.GetTxHash(), true)
	if !isTail {
		// Tx is not a tail => check if the tx is part of another bundle instance, otherwise remove the tx from the storage
		if approversFromSameBundleExist(tx.GetBundleHash(), tx.GetTxHash(), true) {
			return txsToRemove
		}

		DeleteBundleTransaction(tx.GetBundleHash(), tx.GetTxHash(), false)
		txsToRemove[string(tx.GetTxHash())] = struct{}{}
		return txsToRemove
	}

	// Tx is a tail => remove all txs of this bundle that are not used in another bundle instance

	// Tails can't be in another bundle instance => remove it
	DeleteBundle(tx.GetTxHash())
	DeleteBundleTransaction(tx.GetBundleHash(), tx.GetTxHash(), true)
	txsToRemove[string(tx.GetTxHash())] = struct{}{}

	cachedCurrentTx := loadBundleTxIfExistsOrPanic(tx.GetTxHash(), tx.GetBundleHash()) // tx +1

	// iterate as long as the bundle isn't complete and prevent cyclic transactions (such as the genesis)
	for !cachedCurrentTx.GetTransaction().IsHead() && !bytes.Equal(cachedCurrentTx.GetTransaction().GetTxHash(), cachedCurrentTx.GetTransaction().GetTrunkHash()) {

		// check whether the trunk transaction is known to the bundle storage.
		// this also ensures that the transaction has to be in the database
		if !ContainsBundleTransaction(tx.GetBundleHash(), cachedCurrentTx.GetTransaction().GetTrunkHash(), false) {
			// Tx may not exist if the bundle was not received completly
			// Do not force release, since it is loaded again for pruning
			cachedCurrentTx.Release() // tx -1
			return txsToRemove
		}

		// Tx is not a tail => check if the tx is part of another bundle instance, otherwise remove the tx from the bucket
		if approversFromSameBundleExist(tx.GetBundleHash(), cachedCurrentTx.GetTransaction().GetTrunkHash(), true) {
			// Do not force release, since it is loaded again for pruning
			cachedCurrentTx.Release() // tx -1
			return txsToRemove
		}

		DeleteBundleTransaction(tx.GetBundleHash(), cachedCurrentTx.GetTransaction().GetTrunkHash(), false)
		txsToRemove[string(cachedCurrentTx.GetTransaction().GetTrunkHash())] = struct{}{}

		cachedTx := GetCachedTransactionOrNil(cachedCurrentTx.GetTransaction().GetTrunkHash()) // tx +1
		if cachedTx == nil {
			// Tx may not exist if the bundle was not received completly
			// Do not force release, since it is loaded again for pruning
			cachedCurrentTx.Release() // tx -1
			return txsToRemove
		}

		// Do not force release, since it is loaded again for pruning
		cachedCurrentTx.Release() // tx -1
		cachedCurrentTx = cachedTx
	}

	// Do not force release, since it is loaded again for pruning
	cachedCurrentTx.Release() // tx -1

	return txsToRemove
}
