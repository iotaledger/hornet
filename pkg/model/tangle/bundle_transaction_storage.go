package tangle

import (
	"time"

	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/profile"
)

const (
	BundleTxIsTail = 1
)

var (
	bundleTransactionsStorage *objectstorage.ObjectStorage
)

func databaseKeyPrefixForBundleHash(bundleHash trinary.Hash) []byte {
	return trinary.MustTrytesToBytes(bundleHash)[:49]
}

func databaseKeyForBundleTransaction(bundleHash trinary.Hash, txHash trinary.Hash, isTail bool) []byte {
	var isTailByte byte
	if isTail {
		isTailByte = BundleTxIsTail
	}

	result := append(databaseKeyPrefixForBundleHash(bundleHash), isTailByte)
	return append(result, trinary.MustTrytesToBytes(txHash)[:49]...)
}

func databaseKeyForBundleTransactionTxHashBytes(bundleHash trinary.Hash, txHashBytes []byte, isTail bool) []byte {
	var isTailByte byte
	if isTail {
		isTailByte = BundleTxIsTail
	}

	result := append(databaseKeyPrefixForBundleHash(bundleHash), isTailByte)
	return append(result, txHashBytes...)
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

func configureBundleTransactionsStorage() {

	opts := profile.LoadProfile().Caches.BundleTransactions

	bundleTransactionsStorage = objectstorage.New(
		database.BoltStorage(),
		[]byte{DBPrefixBundleTransactions},
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

func (bt *BundleTransaction) GetTransactionHash() trinary.Hash {
	return trinary.MustBytesToTrytes(bt.TxHash, 81)
}

func (bt *BundleTransaction) GetBundleHash() trinary.Hash {
	return trinary.MustBytesToTrytes(bt.BundleHash, 81)
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
func GetCachedBundleTransactionOrNil(bundleHash trinary.Hash, transactionHash trinary.Hash, isTail bool) *CachedBundleTransaction {
	cachedBundleTx := bundleTransactionsStorage.Load(databaseKeyForBundleTransaction(bundleHash, transactionHash, isTail)) // bundleTx +1
	if !cachedBundleTx.Exists() {
		cachedBundleTx.Release(true) // bundleTx -1
		return nil
	}
	return &CachedBundleTransaction{CachedObject: cachedBundleTx}
}

// bundleTx +-0
func GetBundleTransactionHashes(bundleHash trinary.Hash, forceRelease bool, maxFind ...int) []trinary.Hash {
	var bundleTransactionHashes []trinary.Hash

	i := 0
	bundleTransactionsStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			cachedObject.Release(true) // bundleTx -1
			return false
		}

		if !cachedObject.Exists() {
			cachedObject.Release(true) // bundleTx -1
			return true
		}

		bundleTransactionHashes = append(bundleTransactionHashes, (&CachedBundleTransaction{CachedObject: cachedObject}).GetBundleTransaction().GetTransactionHash())
		cachedObject.Release(forceRelease) // bundleTx -1
		return true
	}, databaseKeyPrefixForBundleHash(bundleHash))

	return bundleTransactionHashes
}

// bundleTx +1
func GetBundleTailTransactionHashes(bundleHash trinary.Hash, forceRelease bool, maxFind ...int) []trinary.Hash {
	var bundleTransactionHashes []trinary.Hash

	i := 0
	bundleTransactionsStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			cachedObject.Release(true) // bundleTx -1
			return false
		}

		if !cachedObject.Exists() {
			cachedObject.Release(true) // bundleTx -1
			return true
		}

		bundleTransactionHashes = append(bundleTransactionHashes, (&CachedBundleTransaction{CachedObject: cachedObject}).GetBundleTransaction().GetTransactionHash())
		cachedObject.Release(forceRelease) // bundleTx -1
		return true
	}, append(databaseKeyPrefixForBundleHash(bundleHash), BundleTxIsTail))

	return bundleTransactionHashes
}

// bundleTx +-0
func ContainsBundleTransaction(bundleHash trinary.Hash, transactionHash trinary.Hash, isTail bool) bool {
	return bundleTransactionsStorage.Contains(databaseKeyForBundleTransaction(bundleHash, transactionHash, isTail))
}

// bundleTx +1
func StoreBundleTransaction(bundleHash trinary.Hash, transactionHash trinary.Hash, isTail bool) *CachedBundleTransaction {

	bundleTx := &BundleTransaction{
		BundleHash: trinary.MustTrytesToBytes(bundleHash)[:49],
		IsTail:     isTail,
		TxHash:     trinary.MustTrytesToBytes(transactionHash)[:49],
	}

	cachedObj := bundleTransactionsStorage.ComputeIfAbsent(bundleTx.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject { // bundleTx +1
		bundleTx.Persist()
		bundleTx.SetModified()
		return bundleTx
	})

	return &CachedBundleTransaction{CachedObject: cachedObj}
}

// bundleTx +-0
func DeleteBundleTransaction(bundleHash trinary.Hash, transactionHash trinary.Hash, isTail bool) {
	bundleTransactionsStorage.Delete(databaseKeyForBundleTransaction(bundleHash, transactionHash, isTail))
}

// DeleteBundleTransactionFromBadger deletes the bundle transaction from the persistence layer without accessing the cache.
func DeleteBundleTransactionFromBadger(bundleHash trinary.Hash, txHashBytes []byte, isTail bool) {
	bundleTransactionsStorage.DeleteEntryFromBadger(databaseKeyForBundleTransactionTxHashBytes(bundleHash, txHashBytes, isTail))
}

func ShutdownBundleTransactionsStorage() {
	bundleTransactionsStorage.Shutdown()
}

func FlushBundleTransactionsStorage() {
	bundleTransactionsStorage.Flush()
}

////////////////////////////////////////////////////////////////////////////////

// getTailApproversOfSameBundle returns all tailTx hashes of the same bundle that approve this transaction
func getTailApproversOfSameBundle(bundleHash trinary.Hash, txHash trinary.Hash, forceRelease bool) []trinary.Hash {
	var tailTxHashes []trinary.Hash

	txsToCheck := make(map[trinary.Hash]struct{})
	txsToCheck[txHash] = struct{}{}

	// Loop as long as new transactions are added in every loop cycle
	for len(txsToCheck) != 0 {
		for txHashToCheck := range txsToCheck {
			delete(txsToCheck, txHashToCheck)

			for _, approverHash := range GetApproverHashes(txHashToCheck, forceRelease) {
				cachedApproverTx := GetCachedTransactionOrNil(approverHash) // tx +1
				if cachedApproverTx == nil {
					continue
				}

				approverTx := cachedApproverTx.GetTransaction()
				if approverTx.Tx.Bundle != bundleHash {
					// Not the same bundle => skip
					cachedApproverTx.Release(forceRelease) // tx -1
					continue
				}

				if approverTx.IsTail() {
					// TailTx of the bundle
					tailTxHashes = append(tailTxHashes, approverHash)
				} else {
					// Not the tail, but in the same bundle => walk the future cone
					txsToCheck[approverHash] = struct{}{}
				}

				cachedApproverTx.Release(forceRelease) // tx -1
			}
		}
	}

	return tailTxHashes
}

// approversFromSameBundleExist returns whether there are other transactions in the same bundle, that approve this transaction
func approversFromSameBundleExist(bundleHash trinary.Hash, txHash trinary.Hash, forceRelease bool) bool {

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
func RemoveTransactionFromBundle(tx *transaction.Transaction) map[trinary.Hash]struct{} {

	txsToRemove := make(map[trinary.Hash]struct{})

	// check whether this transaction is a tail or respectively stored as a bundle tail
	isTail := ContainsBundleTransaction(tx.Bundle, tx.Hash, true)
	if !isTail {
		// Tx is not a tail => check if the tx is part of another bundle instance, otherwise remove the tx from the storage
		if approversFromSameBundleExist(tx.Bundle, tx.Hash, true) {
			return txsToRemove
		}

		DeleteBundleTransaction(tx.Bundle, tx.Hash, false)
		txsToRemove[tx.Hash] = struct{}{}
		return txsToRemove
	}

	// Tx is a tail => remove all txs of this bundle that are not used in another bundle instance

	// Tails can't be in another bundle instance => remove it
	DeleteBundle(tx.Hash)
	DeleteBundleTransaction(tx.Bundle, tx.Hash, true)
	txsToRemove[tx.Hash] = struct{}{}

	cachedCurrentTx := loadBundleTxIfExistsOrPanic(tx.Hash, tx.Bundle) // tx +1

	// iterate as long as the bundle isn't complete and prevent cyclic transactions (such as the genesis)
	for !cachedCurrentTx.GetTransaction().IsHead() && cachedCurrentTx.GetTransaction().GetHash() != cachedCurrentTx.GetTransaction().GetTrunk() {

		// check whether the trunk transaction is known to the bundle storage.
		// this also ensures that the transaction has to be in the database
		if !ContainsBundleTransaction(tx.Bundle, cachedCurrentTx.GetTransaction().GetTrunk(), false) {
			// Tx may not exist if the bundle was not received completly
			// Do not force release, since it is loaded again for pruning
			cachedCurrentTx.Release() // tx -1
			return txsToRemove
		}

		// Tx is not a tail => check if the tx is part of another bundle instance, otherwise remove the tx from the bucket
		if approversFromSameBundleExist(tx.Bundle, cachedCurrentTx.GetTransaction().GetTrunk(), true) {
			// Do not force release, since it is loaded again for pruning
			cachedCurrentTx.Release() // tx -1
			return txsToRemove
		}

		DeleteBundleTransaction(tx.Bundle, cachedCurrentTx.GetTransaction().GetTrunk(), false)
		txsToRemove[cachedCurrentTx.GetTransaction().GetTrunk()] = struct{}{}

		cachedTx := GetCachedTransactionOrNil(cachedCurrentTx.GetTransaction().GetTrunk()) // tx +1
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
