package tangle

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/profile"
)

var approversStorage *objectstorage.ObjectStorage

type CachedApprover struct {
	objectstorage.CachedObject
}

type CachedAppprovers []*CachedApprover

func (cachedApprovers CachedAppprovers) Release() {
	for _, cachedApprover := range cachedApprovers {
		cachedApprover.Release()
	}
}

func (c *CachedApprover) GetApprover() *hornet.Approver {
	return c.Get().(*hornet.Approver)
}

func approversFactory(key []byte) objectstorage.StorableObject {
	approver := &hornet.Approver{
		TxHash:       make([]byte, 49),
		ApproverHash: make([]byte, 49),
	}
	copy(approver.TxHash, key[:49])
	copy(approver.ApproverHash, key[49:])
	return approver
}

func GetApproversStorageSize() int {
	return approversStorage.GetSize()
}

func configureApproversStorage() {

	opts := profile.GetProfile().Caches.Approvers

	approversStorage = objectstorage.New(
		[]byte{DBPrefixApprovers},
		approversFactory,
		objectstorage.BadgerInstance(database.GetHornetBadgerInstance()),
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(49, 49),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// approvers +1
func GetCachedApprovers(transactionHash trinary.Hash) CachedAppprovers {
	txHash := trinary.MustTrytesToBytes(transactionHash)[:49]

	cachedApprovers := CachedAppprovers{}

	approversStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedApprovers = append(cachedApprovers, &CachedApprover{cachedObject})
		return true
	}, txHash)

	return cachedApprovers
}

// approvers +1
func StoreApprover(transactionHash trinary.Hash, approverHash trinary.Hash) *CachedApprover {

	approver := &hornet.Approver{
		TxHash:       trinary.MustTrytesToBytes(transactionHash)[:49],
		ApproverHash: trinary.MustTrytesToBytes(approverHash)[:49],
	}

	return &CachedApprover{approversStorage.Store(approver)}
}

// approvers +-0
func DeleteApprovers(transactionHash trinary.Hash) {

	txHash := trinary.MustTrytesToBytes(transactionHash)[:49]

	approversStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approversStorage.Delete(key)
		return true
	}, txHash)
}

func ShutdownApproversStorage() {
	approversStorage.Shutdown()
}
