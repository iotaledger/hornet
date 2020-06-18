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

func unconfirmedTxFactory(key []byte) (objectstorage.StorableObject, int, error) {

	unconfirmedTx := hornet.NewUnconfirmedTx(milestone.Index(binary.LittleEndian.Uint32(key[:4])), key[4:53])
	return unconfirmedTx, 53, nil
}

func GetUnconfirmedTxStorageSize() int {
	return unconfirmedTxStorage.GetSize()
}

func configureUnconfirmedTxStorage(store kvstore.KVStore) {

	opts := profile.LoadProfile().Caches.UnconfirmedTx

	unconfirmedTxStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixUnconfirmedTransactions}),
		unconfirmedTxFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(4, 49),
		objectstorage.KeysOnly(true),
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

// unconfirmedTx +1
func StoreUnconfirmedTx(msIndex milestone.Index, txHash hornet.Hash) *CachedUnconfirmedTx {

	unconfirmedTx := hornet.NewUnconfirmedTx(msIndex, txHash)

	return &CachedUnconfirmedTx{CachedObject: unconfirmedTxStorage.Store(unconfirmedTx)}
}

// DeleteUnconfirmedTxs deletes unconfirmed transaction entries.
func DeleteUnconfirmedTxs(msIndex milestone.Index) {

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
}

func ShutdownUnconfirmedTxsStorage() {
	unconfirmedTxStorage.Shutdown()
}

func FlushUnconfirmedTxsStorage() {
	unconfirmedTxStorage.Flush()
}
