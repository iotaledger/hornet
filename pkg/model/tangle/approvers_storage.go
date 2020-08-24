package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

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
	approver := hornet.NewApprover(key[:49], key[49:98])
	return approver, 98, nil
}

func GetApproversStorageSize() int {
	return approversStorage.GetSize()
}

func configureApproversStorage(store kvstore.KVStore, opts profile.CacheOpts) {

	approversStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixApprovers}),
		approversFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(49, 49),
		objectstorage.KeysOnly(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// approvers +-0
func GetApproverHashes(txHash hornet.Hash, maxFind ...int) hornet.Hashes {
	var approverHashes hornet.Hashes

	i := 0
	approversStorage.ForEachKeyOnly(func(key []byte) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			return false
		}

		approverHashes = append(approverHashes, key[49:98])
		return true
	}, false, txHash)

	return approverHashes
}

// ContainsApprover returns if the given approver exists in the cache/persistence layer.
func ContainsApprover(txHash hornet.Hash, approverHash hornet.Hash) bool {
	return approversStorage.Contains(append(txHash, approverHash...))
}

// ApproverConsumer consumes the given approver during looping through all approvers in the persistence layer.
type ApproverConsumer func(txHash hornet.Hash, approverHash hornet.Hash) bool

// ForEachApprover loops over all approvers.
func ForEachApprover(consumer ApproverConsumer, skipCache bool) {
	approversStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(key[:49], key[49:98])
	}, skipCache)
}

// approvers +1
func StoreApprover(txHash hornet.Hash, approverHash hornet.Hash) *CachedApprover {
	approver := hornet.NewApprover(txHash, approverHash)
	return &CachedApprover{CachedObject: approversStorage.Store(approver)}
}

// approvers +-0
func DeleteApprover(txHash hornet.Hash, approverHash hornet.Hash) {
	approver := hornet.NewApprover(txHash, approverHash)
	approversStorage.Delete(approver.ObjectStorageKey())
}

// approvers +-0
func DeleteApprovers(txHash hornet.Hash) {

	var keysToDelete [][]byte

	approversStorage.ForEachKeyOnly(func(key []byte) bool {
		keysToDelete = append(keysToDelete, key)
		return true
	}, false, txHash)

	for _, key := range keysToDelete {
		approversStorage.Delete(key)
	}
}

func ShutdownApproversStorage() {
	approversStorage.Shutdown()
}

func FlushApproversStorage() {
	approversStorage.Flush()
}
