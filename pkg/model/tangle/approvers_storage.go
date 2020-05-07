package tangle

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/profile"
)

var approversStorage *objectstorage.ObjectStorage

type CachedApprover struct {
	objectstorage.CachedObject
}

type CachedAppprovers []*CachedApprover

func (cachedApprovers CachedAppprovers) Release(force ...bool) {
	for _, cachedApprover := range cachedApprovers {
		cachedApprover.Release(force...)
	}
}

func (c *CachedApprover) GetApprover() *hornet.Approver {
	return c.Get().(*hornet.Approver)
}

func approversFactory(key []byte) (objectstorage.StorableObject, int, error) {
	approver := &hornet.Approver{
		TxHash:       make([]byte, 49),
		ApproverHash: make([]byte, 49),
	}
	copy(approver.TxHash, key[:49])
	copy(approver.ApproverHash, key[49:])
	return approver, 98, nil
}

func GetApproversStorageSize() int {
	return approversStorage.GetSize()
}

func configureApproversStorage() {

	opts := profile.LoadProfile().Caches.Approvers

	approversStorage = objectstorage.New(
		database.BoltStorage(),
		[]byte{DBPrefixApprovers},
		approversFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(49, 49),
		objectstorage.KeysOnly(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// approvers +-0
func GetApproverHashes(transactionHash trinary.Hash, forceRelease bool, maxFind ...int) []trinary.Hash {
	var approverHashes []trinary.Hash

	i := 0
	approversStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			cachedObject.Release(true) // approvers -1
			return false
		}

		if !cachedObject.Exists() {
			cachedObject.Release(true) // approvers -1
			return true
		}

		approverHashes = append(approverHashes, (&CachedApprover{CachedObject: cachedObject}).GetApprover().GetApproverHash())
		cachedObject.Release(forceRelease) // approvers -1
		return true
	}, trinary.MustTrytesToBytes(transactionHash)[:49])

	return approverHashes
}

// approvers +1
func StoreApprover(transactionHash trinary.Hash, approverHash trinary.Hash) *CachedApprover {

	approver := &hornet.Approver{
		TxHash:       trinary.MustTrytesToBytes(transactionHash)[:49],
		ApproverHash: trinary.MustTrytesToBytes(approverHash)[:49],
	}

	cachedObj := approversStorage.ComputeIfAbsent(approver.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject { // approvers +1
		approver.Persist()
		approver.SetModified()
		return approver
	})

	return &CachedApprover{CachedObject: cachedObj}
}

// approvers +-0
func DeleteApprover(transactionHash trinary.Hash, approverHash trinary.Hash) {

	approver := &hornet.Approver{
		TxHash:       trinary.MustTrytesToBytes(transactionHash)[:49],
		ApproverHash: trinary.MustTrytesToBytes(approverHash)[:49],
	}

	approversStorage.Delete(approver.ObjectStorageKey())
}

// DeleteApproverFromBadger deletes the approver from the persistence layer without accessing the cache.
func DeleteApproverFromBadger(transactionHash trinary.Hash, approverHashBytes []byte) {
	approversStorage.DeleteEntryFromBadger(append(trinary.MustTrytesToBytes(transactionHash)[:49], approverHashBytes...))
}

// approvers +-0
func DeleteApprovers(transactionHash trinary.Hash) {

	txHash := trinary.MustTrytesToBytes(transactionHash)[:49]

	approversStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approversStorage.Delete(key)
		cachedObject.Release(true)
		return true
	}, txHash)
}

// DeleteApproversFromBadger deletes the approvers from the persistence layer without accessing the cache.
func DeleteApproversFromBadger(txHashBytes []byte) {

	var approversToDelete [][]byte
	approversStorage.ForEachKeyOnly(func(key []byte) bool {
		approversToDelete = append(approversToDelete, key)
		return true
	}, true, txHashBytes)

	approversStorage.DeleteEntriesFromBadger(approversToDelete)
}

func ShutdownApproversStorage() {
	approversStorage.Shutdown()
}

func FlushApproversStorage() {
	approversStorage.Flush()
}
