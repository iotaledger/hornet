package tangle

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/profile"
)

var (
	txStorage       *objectstorage.ObjectStorage
	metadataStorage *objectstorage.ObjectStorage
)

func TransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedTx *CachedTransaction))(params[0].(*CachedTransaction).Retain())
}

func TransactionHashCaller(handler interface{}, params ...interface{}) {
	handler.(func(txHash hornet.Hash))(params[0].(hornet.Hash))
}

func NewTransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedTx *CachedTransaction, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index))(params[0].(*CachedTransaction).Retain(), params[1].(milestone.Index), params[2].(milestone.Index))
}

func TransactionConfirmedCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMeta *CachedMetadata, msIndex milestone.Index, confTime int64))(params[0].(*CachedMetadata).Retain(), params[1].(milestone.Index), params[2].(int64))
}

// CachedTransaction contains two cached objects, one for transaction data and one for metadata.
type CachedTransaction struct {
	tx       objectstorage.CachedObject
	metadata objectstorage.CachedObject
}

// CachedMetadata contains the cached object only for metadata.
type CachedMetadata struct {
	objectstorage.CachedObject
}

type CachedTransactions []*CachedTransaction

// tx +1
func (cachedTxs CachedTransactions) Retain() CachedTransactions {
	cachedResult := CachedTransactions{}
	for _, cachedTx := range cachedTxs {
		cachedResult = append(cachedResult, cachedTx.Retain())
	}
	return cachedResult
}

// tx -1
func (cachedTxs CachedTransactions) Release(force ...bool) {
	for _, cachedTx := range cachedTxs {
		cachedTx.Release(force...)
	}
}

func (c *CachedTransaction) GetTransaction() *hornet.Transaction {
	return c.tx.Get().(*hornet.Transaction)
}

// meta +1
func (c *CachedTransaction) GetCachedMetadata() *CachedMetadata {
	return &CachedMetadata{c.metadata.Retain()}
}

func (c *CachedTransaction) GetMetadata() *hornet.TransactionMetadata {
	return c.metadata.Get().(*hornet.TransactionMetadata)
}

func (c *CachedMetadata) GetMetadata() *hornet.TransactionMetadata {
	return c.Get().(*hornet.TransactionMetadata)
}

// tx +1
func (c *CachedTransaction) Retain() *CachedTransaction {
	return &CachedTransaction{
		c.tx.Retain(),
		c.metadata.Retain(),
	}
}

func (c *CachedMetadata) Retain() *CachedMetadata {
	return &CachedMetadata{c.CachedObject.Retain()}
}

func (c *CachedTransaction) Exists() bool {
	return c.tx.Exists()
}

// tx -1
// meta -1
func (c *CachedTransaction) ConsumeTransactionAndMetadata(consumer func(*hornet.Transaction, *hornet.TransactionMetadata)) {

	c.tx.Consume(func(txObject objectstorage.StorableObject) {
		c.metadata.Consume(func(metadataObject objectstorage.StorableObject) {
			consumer(txObject.(*hornet.Transaction), metadataObject.(*hornet.TransactionMetadata))
		}, true)
	}, true)
}

// tx -1
// meta -1
func (c *CachedTransaction) ConsumeTransaction(consumer func(*hornet.Transaction)) {
	defer c.metadata.Release(true)
	c.tx.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*hornet.Transaction))
	}, true)
}

// tx -1
// meta -1
func (c *CachedTransaction) ConsumeMetadata(consumer func(*hornet.TransactionMetadata)) {
	defer c.tx.Release(true)
	c.metadata.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*hornet.TransactionMetadata))
	}, true)
}

// meta -1
func (c *CachedMetadata) ConsumeMetadata(consumer func(*hornet.TransactionMetadata)) {
	c.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*hornet.TransactionMetadata))
	}, true)
}

// tx -1
func (c *CachedTransaction) Release(force ...bool) {
	c.tx.Release(force...)
	c.metadata.Release(force...)
}

func transactionFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	tx := hornet.NewTransaction(key[:49])

	if err := tx.UnmarshalObjectStorageValue(data); err != nil {
		return nil, err
	}

	return tx, nil
}

func metadataFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	txMeta := hornet.NewTransactionMetadata(key[:49])

	if err := txMeta.UnmarshalObjectStorageValue(data); err != nil {
		return nil, err
	}

	return txMeta, nil
}

func GetTransactionStorageSize() int {
	return txStorage.GetSize()
}

func configureTransactionStorage(store kvstore.KVStore, opts profile.CacheOpts) {

	txStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixTransactions}),
		transactionFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)

	metadataStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixTransactionMetadata}),
		metadataFactory,
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

// tx +1
func GetCachedTransactionOrNil(txHash hornet.Hash) *CachedTransaction {
	cachedTx := txStorage.Load(txHash) // tx +1
	if !cachedTx.Exists() {
		cachedTx.Release(true) // tx -1
		return nil
	}

	cachedMeta := metadataStorage.Load(txHash) // meta +1
	if !cachedMeta.Exists() {
		cachedTx.Release(true)   // tx -1
		cachedMeta.Release(true) // meta -1
		return nil
	}

	addAdditionalTxInfoToMetadata(cachedMeta.Retain())

	return &CachedTransaction{
		tx:       cachedTx,
		metadata: cachedMeta,
	}
}

// metadata +1
func GetCachedTxMetadataOrNil(txHash hornet.Hash) *CachedMetadata {
	cachedMeta := metadataStorage.Load(txHash) // meta +1
	if !cachedMeta.Exists() {
		cachedMeta.Release(true) // metadata -1
		return nil
	}

	addAdditionalTxInfoToMetadata(cachedMeta.Retain())

	return &CachedMetadata{CachedObject: cachedMeta}
}

func addAdditionalTxInfoToMetadata(cachedMetadata objectstorage.CachedObject) {
	cachedMetadata.Consume(func(metadataObject objectstorage.StorableObject) {
		metadata := metadataObject.(*hornet.TransactionMetadata)

		trunkHash := metadata.GetTrunkHash()
		branchHash := metadata.GetTrunkHash()

		if len(trunkHash) == 0 || len(branchHash) == 0 {
			cachedTx := txStorage.Load(metadata.GetTxHash())
			if !cachedTx.Exists() {
				panic(fmt.Sprintf("transaction not found for metadata: %v", metadata.GetTxHash().Trytes()))
			}

			cachedTx.Consume(func(transactionObject objectstorage.StorableObject) {
				tx := transactionObject.(*hornet.Transaction)
				metadata.SetAdditionalTxInfo(tx.GetTrunkHash(), tx.GetBranchHash(), tx.GetBundleHash(), tx.IsHead(), tx.IsTail(), tx.IsValue())
			}, true)
		}
	}, true)
}

// GetStoredMetadataOrNil returns a metadata object without accessing the cache layer.
func GetStoredMetadataOrNil(txHash hornet.Hash) *hornet.TransactionMetadata {
	storedMeta := metadataStorage.LoadObjectFromStore(txHash)
	if storedMeta == nil {
		return nil
	}
	return storedMeta.(*hornet.TransactionMetadata)
}

// ContainsTransaction returns if the given transaction exists in the cache/persistence layer.
func ContainsTransaction(txHash hornet.Hash) bool {
	return txStorage.Contains(txHash)
}

// TransactionExistsInStore returns if the given transaction exists in the persistence layer.
func TransactionExistsInStore(txHash hornet.Hash) bool {
	return txStorage.ObjectExistsInStore(txHash)
}

// tx +1
func StoreTransactionIfAbsent(transaction *hornet.Transaction) (cachedTx *CachedTransaction, newlyAdded bool) {

	// Store tx + metadata atomically in the same callback
	var cachedMeta objectstorage.CachedObject

	cachedTxData := txStorage.ComputeIfAbsent(transaction.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject { // tx +1
		newlyAdded = true

		metadata := hornet.NewTransactionMetadata(transaction.GetTxHash()[:49])
		metadata.SetAdditionalTxInfo(transaction.GetTrunkHash(), transaction.GetBranchHash(), transaction.GetBundleHash(), transaction.IsHead(), transaction.IsTail(), transaction.IsValue())
		cachedMeta = metadataStorage.Store(metadata) // meta +1

		transaction.Persist()
		transaction.SetModified()
		return transaction
	})

	// if we didn't create a new entry - retrieve the corresponding metadata (it should always exist since it gets created atomically)
	if !newlyAdded {
		cachedMeta = metadataStorage.Load(transaction.GetTxHash()) // meta +1
		addAdditionalTxInfoToMetadata(cachedMeta.Retain())
	}

	return &CachedTransaction{tx: cachedTxData, metadata: cachedMeta}, newlyAdded
}

// TransactionHashConsumer consumes the given transaction hash during looping through all transactions in the persistence layer.
type TransactionHashConsumer func(txHash hornet.Hash) bool

// ForEachTransactionHash loops over all transaction hashes.
func ForEachTransactionHash(consumer TransactionHashConsumer, skipCache bool) {
	txStorage.ForEachKeyOnly(func(txHash []byte) bool {
		return consumer(txHash)
	}, skipCache)
}

// ForEachTransactionMetadataHash loops over all transaction metadata hashes.
func ForEachTransactionMetadataHash(consumer TransactionHashConsumer, skipCache bool) {
	metadataStorage.ForEachKeyOnly(func(txHash []byte) bool {
		return consumer(txHash)
	}, skipCache)
}

// DeleteTransaction deletes the transaction and metadata in the cache/persistence layer.
func DeleteTransaction(txHash hornet.Hash) {
	// metadata has to be deleted before the tx, otherwise we could run into a data race in the object storage
	metadataStorage.Delete(txHash)
	txStorage.Delete(txHash)
}

// DeleteTransactionMetadata deletes the metadata in the cache/persistence layer.
func DeleteTransactionMetadata(txHash hornet.Hash) {
	metadataStorage.Delete(txHash)
}

func ShutdownTransactionStorage() {
	txStorage.Shutdown()
	metadataStorage.Shutdown()
}

func FlushTransactionStorage() {
	txStorage.Flush()
	metadataStorage.Flush()
}
