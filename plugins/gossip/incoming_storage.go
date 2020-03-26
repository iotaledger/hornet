package gossip

import (
	"time"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/profile"
)

var (
	incomingStorage *objectstorage.ObjectStorage
)

type CachedNeighborRequest struct {
	objectstorage.CachedObject
}

func (c *CachedNeighborRequest) GetRequest() *PendingNeighborRequests {
	return c.Get().(*PendingNeighborRequests)
}

func incomingFactory(key []byte) (objectstorage.StorableObject, error, int) {
	req := &PendingNeighborRequests{
		recTxBytes: make([]byte, len(key)),
		requests:   make([]*NeighborRequest, 0),
	}
	copy(req.recTxBytes, key)
	return req, nil, len(key)
}

func configureIncomingStorage() {
	opts := profile.GetProfile().Caches.IncomingTransactionFilter

	incomingStorage = objectstorage.New(
		nil,
		nil,
		incomingFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(false),
		objectstorage.KeysOnly(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

func GetIncomingStorageSize() int {
	return incomingStorage.GetSize()
}

// neighborReq +1
func GetCachedPendingNeighborRequest(recTxBytes []byte) *CachedNeighborRequest {
	return &CachedNeighborRequest{
		CachedObject: incomingStorage.ComputeIfAbsent(recTxBytes, func(key []byte) objectstorage.StorableObject { // neighborReq +1
			cachedNeighborReq, _, _ := incomingFactory(recTxBytes)
			return cachedNeighborReq
		}),
	}
}
