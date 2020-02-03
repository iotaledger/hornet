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

func incomingFactory(key []byte) objectstorage.StorableObject {
	return &PendingNeighborRequests{
		recTxBytes: key,
		requests:   make([]*NeighborRequest, 0),
	}
}

func configureIncomingStorage() {
	opts := profile.GetProfile().Caches.IncomingTransactionFilter

	incomingStorage = objectstorage.New(nil,
		incomingFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(false),
		//objectstorage.EnableLeakDetection(),
	)
}

func GetIncomingStorageSize() int {
	return incomingStorage.GetSize()
}

// +1
func GetCachedPendingNeighborRequest(recTxBytes []byte) *CachedNeighborRequest {
	return &CachedNeighborRequest{
		incomingStorage.ComputeIfAbsent(recTxBytes, incomingFactory),
	}
}
