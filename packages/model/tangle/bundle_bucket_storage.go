package tangle

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/iota.go/trinary"

	hornetDB "github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/profile"
)

const (
	IS_TAIL = 1
)

var (
	bundleTransactionsStorage *objectstorage.ObjectStorage
)

func databaseKeyPrefixForBundleHash(bundleHash trinary.Hash) []byte {
	return trinary.MustTrytesToBytes(bundleHash)[:49]
}

func databaseKeyForBundle(bundleHash trinary.Hash, txHash trinary.Hash, isTail bool) []byte {
	var isTailByte byte
	if isTail {
		isTailByte = IS_TAIL
	}

	result := append(databaseKeyPrefixForBundleHash(bundleHash), isTailByte)
	return append(result, trinary.MustTrytesToBytes(txHash)[:49]...)
}

func bundleTransactionFactory(key []byte) objectstorage.StorableObject {
	return &BundleTransaction{
		BundleHash: key[:49],
		IsTail:     key[49] == IS_TAIL,
		TxHash:     key[50:],
	}
}

func GetBundleTransactionsStorageSize() int {
	return bundleTransactionsStorage.GetSize()
}

func configureBundleTransactionsStorage() {

	opts := profile.GetProfile().Caches.Bundles

	bundleTransactionsStorage = objectstorage.New(
		[]byte{DBPrefixBundles},
		bundleTransactionFactory,
		objectstorage.BadgerInstance(hornetDB.GetHornetBadgerInstance()),
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(49, 49)) // BundleHash, TxHash
}

// Storable Object
type BundleTransaction struct {
	objectstorage.StorableObjectFlags

	// Key
	BundleHash []byte
	IsTail     bool
	TxHash     []byte

	// Value
	meta byte
}

func (bt *BundleTransaction) GetMetadata() byte {
	return bt.meta
}

// ObjectStorage interface
func (bt *BundleTransaction) Update(other objectstorage.StorableObject) {
	if obj, ok := other.(*BundleTransaction); !ok {
		panic("invalid object passed to BundleTransaction.Update()")
	} else {
		bt.BundleHash = obj.BundleHash
		bt.IsTail = obj.IsTail
		bt.TxHash = obj.TxHash

		bt.meta = obj.meta
	}
}

func (bt *BundleTransaction) GetStorageKey() []byte {
	var isTailByte byte
	if bt.IsTail {
		isTailByte = IS_TAIL
	}

	result := append(bt.BundleHash, isTailByte)
	return append(result, bt.TxHash...)
}

func (bt *BundleTransaction) MarshalBinary() (data []byte, err error) {
	return []byte{bt.meta}, nil
}

func (bt *BundleTransaction) UnmarshalBinary(data []byte) error {
	bt.meta = data[0]
	return nil
}

// Cached Object
type CachedBundleTransaction struct {
	*objectstorage.CachedObject
}

type CachedBundleTransactions []*CachedBundleTransaction

func (cachedBundleTransactions CachedBundleTransactions) RegisterConsumer() {
	for _, cachedBundleTransaction := range cachedBundleTransactions {
		cachedBundleTransaction.RegisterConsumer()
	}
}

func (cachedBundleTransactions CachedBundleTransactions) Release() {
	for _, cachedBundleTransaction := range cachedBundleTransactions {
		cachedBundleTransaction.Release()
	}
}

func (c *CachedBundleTransaction) GetBundleTransaction() *BundleTransaction {
	return c.Get().(*BundleTransaction)
}

// +1
func GetCachedBundleTransaction(bundleHash trinary.Hash, transactionHash trinary.Hash, isTail bool) *CachedBundleTransaction {
	return &CachedBundleTransaction{bundleTransactionsStorage.Load(databaseKeyForBundle(bundleHash, transactionHash, isTail))}
}

// + 1
func GetCachedBundleTransactions(bundleHash trinary.Hash, maxFindTransactions ...int) CachedBundleTransactions {
	bndlHash := databaseKeyPrefixForBundleHash(bundleHash)

	bndlTxs := CachedBundleTransactions{}

	bundleTransactionsStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject) bool {
		bndlTxs = append(bndlTxs, &CachedBundleTransaction{cachedObject})
		return true
	}, bndlHash)

	return bndlTxs
}

func GetCachedBundleTailTransactions(bundleHash trinary.Hash) CachedBundleTransactions {
	bndlHash := databaseKeyPrefixForBundleHash(bundleHash)

	bndlTxs := CachedBundleTransactions{}

	bundleTransactionsStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject) bool {
		bndlTxs = append(bndlTxs, &CachedBundleTransaction{cachedObject})
		return true
	}, append(bndlHash, IS_TAIL))

	return bndlTxs
}

// +-0
func ContainsBundleTransaction(bundleHash trinary.Hash, transactionHash trinary.Hash, isTail bool) bool {
	return bundleTransactionsStorage.Contains(databaseKeyForBundle(bundleHash, transactionHash, isTail))
}

// + 1
func StoreBundleTransaction(bundleHash trinary.Hash, transactionHash trinary.Hash, isTail bool) *CachedBundleTransaction {

	bundleTx := &BundleTransaction{
		BundleHash: trinary.MustTrytesToBytes(bundleHash)[:49],
		IsTail:     isTail,
		TxHash:     trinary.MustTrytesToBytes(transactionHash)[:49],
		meta:       0,
	}

	return &CachedBundleTransaction{bundleTransactionsStorage.Store(bundleTx)}
}

// +-0
func DeleteBundleTransaction(bundleHash trinary.Hash, transactionHash trinary.Hash, isTail bool) {
	bundleTransactionsStorage.Delete(databaseKeyForBundle(bundleHash, transactionHash, isTail))
}

func ShutdownBundleTransactionsStorage() {
	bundleTransactionsStorage.Shutdown()
}

// Adds a new transaction to the storage by either creating a new Bundle instance,
// assigning the transaction to an existing Bundle.
// It returns a slice of Bundles to which the transaction was added to. Adding a tail
// transaction will ever only return one Bundle within the slice.
func AddTransactionToBundles(hornetTx *hornet.Transaction) (bundles []*Bundle, alreadyAdded bool) {

	cachedTx := GetCachedTransaction(hornetTx.GetHash())
	if cachedTx.Exists() {
		cachedTx.Release()
		return nil, true
	}
	cachedTx.Release()

	// ToDo: global lock?

	// Store the tx in the storage, this will update the tx reference automatically
	cachedTx = StoreTransaction(hornetTx)
	defer cachedTx.Release()

	// Store the tx in the bundleTransactionsStorage
	StoreBundleTransaction(hornetTx.Tx.Bundle, hornetTx.GetHash(), hornetTx.IsTail()).Release()

	if hornetTx.IsTail() {
		// create a new bundle instance
		bndl := &Bundle{
			txs:      make(map[trinary.Hash]struct{}),
			metadata: bitmask.BitMask(0),
			modified: true,
			hash:     hornetTx.Tx.Bundle,
			tailTx:   hornetTx.GetHash(),
		}
		bndl.txs[hornetTx.GetHash()] = struct{}{}
		bndl.lastIndex = hornetTx.Tx.LastIndex

		// check whether it is a bundle with only one transaction
		if hornetTx.Tx.CurrentIndex == hornetTx.Tx.LastIndex {
			bndl.headTx = hornetTx.GetHash()
		} else {
			// lets try to complete the bundle by assigning txs into this bundle
			remap(bndl, cachedTx)
		}

		return []*Bundle{bndl}, false
	}

	// try a remap on every non complete bundle in the bucket.
	bndls := GetBundlesOfTransaction(hornetTx.Tx.Bundle, hornetTx.GetHash())
	return bndls, false
}

// Remaps transactions into the given bundle by traversing from the given start transaction through the trunk.
func remap(bndl *Bundle, startTx *CachedTransaction, onMapped ...func(mapped *CachedTransaction)) {
	bndl.txsMu.Lock()
	defer bndl.txsMu.Unlock()

	// This will be released while or after the loop as current
	startTx.RegisterConsumer() //+1

	current := startTx

	// iterate as long as the bundle isn't complete and prevent cyclic transactions (such as the genesis)
	for current.GetTransaction().GetHash() != current.GetTransaction().GetTrunk() && !bndl.isComplete() && !current.GetTransaction().IsHead() {

		// check whether the trunk transaction is known to the bundle storage.
		// this also ensures that the transaction has to be in the database
		if !ContainsBundleTransaction(bndl.GetHash(), current.GetTransaction().GetTrunk(), false) {
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

// getBundleOfTailTransaction returns the bundle that belongs to the tailTx
func getBundleOfTailTransaction(bndlTailTx *CachedBundleTransaction) *Bundle {
	bundleTailTx := bndlTailTx.GetBundleTransaction()
	metadata := bundleTailTx.GetMetadata()
	txTailHash := trinary.MustBytesToTrytes(bundleTailTx.TxHash, 81)

	cachedTxTail := GetCachedTransaction(txTailHash) //+1
	if !cachedTxTail.Exists() {
		cachedTxTail.Release() //-1
		return nil
	}

	txTail := cachedTxTail.GetTransaction()

	// create a new bundle instance
	bndl := &Bundle{
		txs:      make(map[trinary.Hash]struct{}),
		metadata: bitmask.BitMask(metadata),
		modified: false,
		hash:     txTail.Tx.Bundle,
		tailTx:   txTail.GetHash(),
	}
	bndl.txs[txTailHash] = struct{}{}
	bndl.lastIndex = txTail.Tx.LastIndex

	// check whether it is a bundle with only one transaction
	if txTail.Tx.CurrentIndex == txTail.Tx.LastIndex {
		bndl.headTx = txTail.GetHash()
	} else {
		// lets try to complete the bundle by assigning txs into this bundle
		remap(bndl, cachedTxTail)
	}

	return bndl
}

// getTailApproversOfSameBundle returns all tailTx hashes of the same bundle that approve this transaction
func getTailApproversOfSameBundle(bundleHash trinary.Hash, txHash trinary.Hash) []trinary.Hash {
	var tailTxHashes []trinary.Hash

	txsToCheck := make(map[trinary.Hash]struct{})
	txsToCheck[txHash] = struct{}{}

	// Loop as long as new transactions are added in every loop cycle
	for len(txsToCheck) != 0 {
		for txHashToCheck := range txsToCheck {
			delete(txsToCheck, txHashToCheck)

			cachedTxApprovers := GetCachedApprovers(txHashToCheck) // approvers +1
			for _, cachedTxApprover := range cachedTxApprovers {
				if !cachedTxApprover.Exists() {
					continue
				}

				approverHash := cachedTxApprover.GetApprover().GetHash()

				cachedApproverTx := GetCachedTransaction(approverHash) // tx +1
				if !cachedApproverTx.Exists() {
					cachedApproverTx.Release() // tx -1
					continue
				}

				approverTx := cachedApproverTx.GetTransaction()
				if approverTx.Tx.Bundle != bundleHash {
					// Not the same bundle => skip
					cachedApproverTx.Release() // tx -1
					continue
				}

				if approverTx.IsTail() {
					// TailTx of the bundle
					tailTxHashes = append(tailTxHashes, approverHash)
				} else {
					// Not the tail, but in the same bundle => walk the future cone
					txsToCheck[approverHash] = struct{}{}
				}

				cachedApproverTx.Release() // tx -1
			}
			cachedTxApprovers.Release() // approvers -1
		}
	}

	return tailTxHashes
}

// existApproversFromSameBundle returns if there are other transactions in the same bundle, that approve this transaction
func existApproversFromSameBundle(bundleHash trinary.Hash, txHash trinary.Hash) bool {

	cachedTxApprovers := GetCachedApprovers(txHash) // approvers +1
	for _, approver := range cachedTxApprovers {
		if approver.Exists() {
			cachedApproverTx := GetCachedTransaction(approver.GetApprover().GetHash()) // tx +1
			if cachedApproverTx.Exists() {
				approverTx := cachedApproverTx.GetTransaction()

				if approverTx.Tx.Bundle == bundleHash {
					// Tx is used in another bundle instance => do not delete
					cachedApproverTx.Release()  // tx -1
					cachedTxApprovers.Release() // approvers -1
					return true
				}
			}
			cachedApproverTx.Release() // tx -1
		}
	}
	cachedTxApprovers.Release() // approvers -1

	return false
}

////////////////////////////////////////////////////////////////////////////////////

// GetBundles returns all existing bundle instances for that bundle hash
func GetBundles(bundleHash trinary.Hash) []*Bundle {

	var bndls []*Bundle

	bndlTailTxs := GetCachedBundleTailTransactions(bundleHash) // bundles +1
	defer bndlTailTxs.Release()                                // bundles -1

	for _, bndlTailTx := range bndlTailTxs {
		bndl := getBundleOfTailTransaction(bndlTailTx)
		if bndl != nil {
			bndls = append(bndls, bndl)
		}
	}

	if len(bndls) == 0 {
		return nil
	}

	return bndls
}

// GetBundleOfTailTransaction gets the bundle this tail transaction is present in or nil.
func GetBundleOfTailTransaction(bundleHash trinary.Hash, txHash trinary.Hash) *Bundle {

	bndlTailTx := GetCachedBundleTransaction(bundleHash, txHash, true) // bundle +1
	defer bndlTailTx.Release()                                         // bundle -1

	if !bndlTailTx.Exists() {
		return nil
	}

	return getBundleOfTailTransaction(bndlTailTx)
}

// GetBundlesOfTransaction gets all bundle instances in which this transaction is present.
// A transaction can be in multiple bundle instances simultaneously
// due to the nature of reattached transactions being able to form infinite amount of bundles
// which attach to the same underlying bundle transaction. For example it is possible to reattach
// a bundle's tail transaction directly "on top" of the origin one.
func GetBundlesOfTransaction(bundleHash trinary.Hash, txHash trinary.Hash) []*Bundle {

	var bndls []*Bundle

	cachedTx := GetCachedTransaction(txHash) // tx +1
	if !cachedTx.Exists() {
		cachedTx.Release() // tx -1
		return nil
	}

	tx := cachedTx.GetTransaction()
	cachedTx.Release() // tx -1

	if tx.IsTail() {
		bndl := GetBundleOfTailTransaction(bundleHash, txHash)
		if bndl == nil {
			return nil
		}
		return append(bndls, bndl)
	}

	tailTxHashes := getTailApproversOfSameBundle(bundleHash, txHash)
	for _, tailTxHash := range tailTxHashes {
		bndl := GetBundleOfTailTransaction(bundleHash, tailTxHash)
		if bndl == nil {
			continue
		}
		bndls = append(bndls, bndl)
	}

	if len(bndls) == 0 {
		return nil
	}

	return bndls
}

// GetTransactionHashes returns all transaction hashes that belong to this bundle
func GetTransactionHashes(bundleHash trinary.Hash, maxFindTransactions ...int) []trinary.Hash {
	var txHashes []trinary.Hash

	bndlTxs := GetCachedBundleTransactions(bundleHash, maxFindTransactions...)
	defer bndlTxs.Release()

	for _, bndlTx := range bndlTxs {
		bundleTx := bndlTx.GetBundleTransaction()
		txHash := trinary.MustBytesToTrytes(bundleTx.TxHash, 81)
		txHashes = append(txHashes, txHash)
	}

	return txHashes
}

// RemoveTransactionFromBundle removes the transaction if non-tail and not associated to a bundle instance or
// if tail, it removes all the transactions of the bundle from the storage that are not used in another bundle instance.
func RemoveTransactionFromBundle(bundleHash trinary.Hash, txHash trinary.Hash) map[trinary.Hash]struct{} {

	txsToRemove := make(map[trinary.Hash]struct{})

	isTail := ContainsBundleTransaction(bundleHash, txHash, true)
	if isTail {
		// Tx is a tail => remove all txs of this bundle that are not used in another bundle instance

		// Tails can't be in another bundle instance => remove it
		DeleteBundleTransaction(bundleHash, txHash, true)
		txsToRemove[txHash] = struct{}{}

		current := loadBundleTxIfExistsOrPanic(txHash, bundleHash) // tx +1

		// iterate as long as the bundle isn't complete and prevent cyclic transactions (such as the genesis)
		for !current.GetTransaction().IsHead() && current.GetTransaction().GetHash() != current.GetTransaction().GetTrunk() {

			// check whether the trunk transaction is known to the bundle storage.
			// this also ensures that the transaction has to be in the database
			if !ContainsBundleTransaction(bundleHash, current.GetTransaction().GetTrunk(), false) {
				panic(fmt.Sprintf("bundle %s has a reference to a non persisted transaction: %s", bundleHash, current.GetTransaction().GetTrunk()))
			}

			// Tx is not a tail => check if the tx is part of another bundle instance, otherwise remove the tx from the bucket
			if existApproversFromSameBundle(bundleHash, current.GetTransaction().GetTrunk()) {
				current.Release() // tx -1
				return txsToRemove
			}

			DeleteBundleTransaction(bundleHash, current.GetTransaction().GetTrunk(), false)
			txsToRemove[current.GetTransaction().GetTrunk()] = struct{}{}
			current.Release() // tx -1

			current = loadBundleTxIfExistsOrPanic(current.GetTransaction().GetTrunk(), bundleHash) // tx +1
		}
		current.Release() // tx -1

	} else {
		// Tx is not a tail => check if the tx is part of another bundle instance, otherwise remove the tx from the storage
		if existApproversFromSameBundle(bundleHash, txHash) {
			return txsToRemove
		}

		DeleteBundleTransaction(bundleHash, txHash, false)
		txsToRemove[txHash] = struct{}{}
	}

	return txsToRemove
}
