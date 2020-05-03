package tangle

import (
	"encoding/binary"
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/database"
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
	unconfirmedTx := &hornet.UnconfirmedTx{
		LatestMilestoneIndex: milestone.Index(binary.LittleEndian.Uint32(key[:4])),
		TxHash:               make([]byte, 49),
	}
	copy(unconfirmedTx.TxHash, key[4:])
	return unconfirmedTx, 53, nil
}

func GetUnconfirmedTxStorageSize() int {
	return unconfirmedTxStorage.GetSize()
}

func configureUnconfirmedTxStorage() {

	opts := profile.LoadProfile().Caches.UnconfirmedTx

	unconfirmedTxStorage = objectstorage.New(
		database.GetHornetBadgerInstance(),
		[]byte{DBPrefixUnconfirmedTransactions},
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

// unconfirmedTx +-0
func GetUnconfirmedTxHashes(msIndex milestone.Index, forceRelease bool, maxFind ...int) []trinary.Hash {
	var unconfirmedTxHashes []trinary.Hash

	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(msIndex))

	i := 0
	unconfirmedTxStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			cachedObject.Release(true) // unconfirmedTx -1
			return false
		}

		if !cachedObject.Exists() {
			cachedObject.Release(true) // unconfirmedTx -1
			return true
		}

		unconfirmedTxHashes = append(unconfirmedTxHashes, (&CachedUnconfirmedTx{CachedObject: cachedObject}).GetUnconfirmedTx().GetTransactionHash())
		cachedObject.Release(forceRelease) // unconfirmedTx -1
		return true
	}, key)

	return unconfirmedTxHashes
}

// unconfirmedTx +1
func StoreUnconfirmedTx(msIndex milestone.Index, txHash trinary.Hash) *CachedUnconfirmedTx {

	unconfirmedTx := &hornet.UnconfirmedTx{
		LatestMilestoneIndex: msIndex,
		TxHash:               trinary.MustTrytesToBytes(txHash)[:49],
	}

	cachedObj := unconfirmedTxStorage.ComputeIfAbsent(unconfirmedTx.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject { // unconfirmedTx +1
		unconfirmedTx.Persist()
		unconfirmedTx.SetModified()
		return unconfirmedTx
	})

	return &CachedUnconfirmedTx{CachedObject: cachedObj}
}

// unconfirmedTx +-0
func DeleteUnconfirmedTxs(msIndex milestone.Index) {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(msIndex))

	unconfirmedTxStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		unconfirmedTxStorage.Delete(key)
		cachedObject.Release(true)
		return true
	}, key)
}

// DeleteUnconfirmedTxsFromBadger deletes unconfirmed transactions without accessing the cache.
func DeleteUnconfirmedTxsFromBadger(msIndex milestone.Index) {

	msIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(msIndexBytes, uint32(msIndex))

	var txHashes [][]byte
	unconfirmedTxStorage.ForEachKeyOnly(func(key []byte) bool {
		txHashes = append(txHashes, key)
		return true
	}, true, msIndexBytes)

	unconfirmedTxStorage.DeleteEntriesFromBadger(txHashes)
}

func ShutdownUnconfirmedTxsStorage() {
	unconfirmedTxStorage.Shutdown()
}

func FlushUnconfirmedTxsStorage() {
	unconfirmedTxStorage.Flush()
}
