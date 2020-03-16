package tangle

import (
	"encoding/binary"
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/profile"
)

var firstSeenTxStorage *objectstorage.ObjectStorage

type CachedFirstSeenTx struct {
	objectstorage.CachedObject
}

type CachedFirstSeenTxs []*CachedFirstSeenTx

func (cachedFirstSeenTxs CachedFirstSeenTxs) Release(force ...bool) {
	for _, cachedFirstSeenTx := range cachedFirstSeenTxs {
		cachedFirstSeenTx.Release(force...)
	}
}

func (c *CachedFirstSeenTx) GetFirstSeenTx() *hornet.FirstSeenTx {
	return c.Get().(*hornet.FirstSeenTx)
}

func firstSeenTxFactory(key []byte) objectstorage.StorableObject {
	firstSeenTx := &hornet.FirstSeenTx{
		FirstSeenLatestMilestoneIndex: milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(key[:4])),
		TxHash:                        make([]byte, 49),
	}
	copy(firstSeenTx.TxHash, key[4:])
	return firstSeenTx
}

func GetFirstSeenTxStorageSize() int {
	return firstSeenTxStorage.GetSize()
}

func configureFirstSeenTxStorage() {

	opts := profile.GetProfile().Caches.FirstSeenTx

	firstSeenTxStorage = objectstorage.New(
		database.GetHornetBadgerInstance(),
		[]byte{DBPrefixFirstSeenTransactions},
		firstSeenTxFactory,
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

// firstSeenTx +-0
func GetFirstSeenTxHashes(msIndex milestone_index.MilestoneIndex, forceRelease bool, maxFind ...int) []trinary.Hash {
	var firstSeenTxHashes []trinary.Hash

	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(msIndex))

	i := 0
	firstSeenTxStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			cachedObject.Release(true) // firstSeenTx -1
			return false
		}

		if !cachedObject.Exists() {
			cachedObject.Release(true) // firstSeenTx -1
			return true
		}

		firstSeenTxHashes = append(firstSeenTxHashes, (&CachedFirstSeenTx{CachedObject: cachedObject}).GetFirstSeenTx().GetTransactionHash())
		cachedObject.Release(forceRelease) // firstSeenTx -1
		return true
	}, key)

	return firstSeenTxHashes
}

// firstSeenTx +1
func StoreFirstSeenTx(msIndex milestone_index.MilestoneIndex, txHash trinary.Hash) *CachedFirstSeenTx {

	firstSeenTx := &hornet.FirstSeenTx{
		FirstSeenLatestMilestoneIndex: msIndex,
		TxHash:                        trinary.MustTrytesToBytes(txHash)[:49],
	}

	return &CachedFirstSeenTx{firstSeenTxStorage.Store(firstSeenTx)}
}

// firstSeenTx +-0
func DeleteFirstSeenTxs(msIndex milestone_index.MilestoneIndex) {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(msIndex))

	firstSeenTxStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		firstSeenTxStorage.Delete(key)
		cachedObject.Release(true)
		return true
	}, key)
}

func ShutdownFirstSeenTxsStorage() {
	firstSeenTxStorage.Shutdown()
}

func FixFirstSeenTxs(msIndex milestone_index.MilestoneIndex) {

	// Search all entries with milestone 0
	for _, firstSeenTxHash := range GetFirstSeenTxHashes(0, true) {

		key := make([]byte, 4)
		binary.LittleEndian.PutUint32(key, uint32(0))
		key = append(key, trinary.MustTrytesToBytes(firstSeenTxHash)[:49]...)
		firstSeenTxStorage.Delete(key)

		StoreFirstSeenTx(msIndex, firstSeenTxHash).Release(true)
	}
}
