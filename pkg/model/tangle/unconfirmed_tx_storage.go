package tangle

import (
	"encoding/binary"
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/profile"
)

var unconfirmedTxStorage *objectstorage.ObjectStorage

type CachedUnconfirmedTx struct {
	objectstorage.CachedObject
}

type CachedUnconfirmedTxs []*CachedUnconfirmedTx

func (cachedUnconfirmedTxs CachedUnconfirmedTxs) Release(force ...bool) {
	for _, cachedUnconfirmedTx := range cachedUnconfirmedTxs {
		cachedUnconfirmedTx.Release(force...)
	}
}

func (c *CachedUnconfirmedTx) GetUnconfirmedTx() *hornet.UnconfirmedTx {
	return c.Get().(*hornet.UnconfirmedTx)
}

func unconfirmedTxFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {

	unconfirmedTx := hornet.NewUnconfirmedTx(milestone.Index(binary.LittleEndian.Uint32(key[:4])), key[4:53])
	return unconfirmedTx, nil
}

func GetUnconfirmedTxStorageSize() int {
	return unconfirmedTxStorage.GetSize()
}

func configureUnconfirmedTxStorage(store kvstore.KVStore, opts profile.CacheOpts) {

	unconfirmedTxStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixUnconfirmedTransactions}),
		unconfirmedTxFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(4, 49),
		objectstorage.KeysOnly(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// GetUnconfirmedTxHashes returns all hashes of unconfirmed transactions for that milestone.
func GetUnconfirmedTxHashes(msIndex milestone.Index, forceRelease bool) hornet.Hashes {

	var unconfirmedTxHashes hornet.Hashes

	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(msIndex))

	unconfirmedTxStorage.ForEachKeyOnly(func(key []byte) bool {
		unconfirmedTxHashes = append(unconfirmedTxHashes, hornet.Hash(key[4:53]))
		return true
	}, false, key)

	return unconfirmedTxHashes
}

// UnconfirmedTxConsumer consumes the given unconfirmed transaction during looping through all unconfirmed transactions in the persistence layer.
type UnconfirmedTxConsumer func(msIndex milestone.Index, txHash hornet.Hash) bool

// ForEachUnconfirmedTx loops over all unconfirmed transactions.
func ForEachUnconfirmedTx(consumer UnconfirmedTxConsumer, skipCache bool) {
	unconfirmedTxStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(milestone.Index(binary.LittleEndian.Uint32(key[:4])), key[4:53])
	}, skipCache)
}

// unconfirmedTx +1
func StoreUnconfirmedTx(msIndex milestone.Index, txHash hornet.Hash) *CachedUnconfirmedTx {
	unconfirmedTx := hornet.NewUnconfirmedTx(msIndex, txHash)
	return &CachedUnconfirmedTx{CachedObject: unconfirmedTxStorage.Store(unconfirmedTx)}
}

// DeleteUnconfirmedTxs deletes unconfirmed transaction entries.
func DeleteUnconfirmedTxs(msIndex milestone.Index) int {

	msIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(msIndexBytes, uint32(msIndex))

	var keysToDelete [][]byte

	unconfirmedTxStorage.ForEachKeyOnly(func(key []byte) bool {
		keysToDelete = append(keysToDelete, key)
		return true
	}, false, msIndexBytes)

	for _, key := range keysToDelete {
		unconfirmedTxStorage.Delete(key)
	}

	return len(keysToDelete)
}

func ShutdownUnconfirmedTxsStorage() {
	unconfirmedTxStorage.Shutdown()
}

func FlushUnconfirmedTxsStorage() {
	unconfirmedTxStorage.Flush()
}
