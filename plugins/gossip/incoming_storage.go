package gossip

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

var (
	incomingStorage *objectstorage.ObjectStorage
)

type CachedNeighborRequest struct {
	*objectstorage.CachedObject
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

func GetIncomingStorageSize() int {
	return incomingStorage.GetSize()
}

// +1
func GetCachedPendingNeighborRequest(recTxBytes []byte) *CachedNeighborRequest {
	return &CachedNeighborRequest{
		incomingStorage.ComputeIfAbsent(recTxBytes, incomingFactory),
	}
}
