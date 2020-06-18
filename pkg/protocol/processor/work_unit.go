package processor

import (
	"bytes"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol/bqueue"
	"github.com/gohornet/hornet/pkg/protocol/legacy"
	"github.com/gohornet/hornet/pkg/protocol/rqueue"
)

// WorkUnitState defines the state which a WorkUnit is in.
type WorkUnitState byte

const (
	Hashing WorkUnitState = 1 << 0
	Invalid WorkUnitState = 1 << 1
	Hashed  WorkUnitState = 1 << 2
)

// defines the factory function for WorkUnits.
func workUnitFactory(key []byte) (objectstorage.StorableObject, int, error) {
	req := &WorkUnit{
		receivedTxBytes: make([]byte, len(key)),
		requests:        make([]*Request, 0),
	}
	// TODO: check for a more efficient key instead of copying all tx bytes
	copy(req.receivedTxBytes, key)
	return req, len(key), nil
}

// CachedWorkUnit represents a cached WorkUnit.
type CachedWorkUnit struct {
	objectstorage.CachedObject
}

// WorkUnit gets the underlying WorkUnit.
func (c *CachedWorkUnit) WorkUnit() *WorkUnit {
	return c.Get().(*WorkUnit)
}

// WorkUnit defines the work around processing a received transaction and its
// associated requests from peers. There is at most one WorkUnit active per same
// transaction bytes.
type WorkUnit struct {
	objectstorage.StorableObjectFlags
	processingLock syncutils.Mutex

	// data
	dataLock        syncutils.RWMutex
	receivedTxBytes []byte
	receivedTxHash  hornet.Hash
	tx              *hornet.Transaction

	// status
	stateLock syncutils.RWMutex
	state     WorkUnitState

	// requests
	requestsLock syncutils.RWMutex
	requests     []*Request
}

func (wu *WorkUnit) Update(_ objectstorage.StorableObject) {
	panic("WorkUnit should never be updated")
}

func (wu *WorkUnit) ObjectStorageKey() []byte {
	return wu.receivedTxBytes
}

func (wu *WorkUnit) ObjectStorageValue() []byte {
	return nil
}

func (wu *WorkUnit) UnmarshalObjectStorageValue(_ []byte) (consumedBytes int, err error) {
	return 0, nil
}

// UpdateState updates the WorkUnit's state.
func (wu *WorkUnit) UpdateState(state WorkUnitState) {
	wu.stateLock.Lock()
	wu.state = 0
	wu.state |= state
	wu.stateLock.Unlock()
}

// Is tells whether the WorkUnit has the given state.
func (wu *WorkUnit) Is(state WorkUnitState) bool {
	wu.stateLock.Lock()
	defer wu.stateLock.Unlock()
	return wu.state&state > 0
}

// adds a Request for the given peer to this WorkUnit.
// requestedTxHashBytes can be nil to flag that this request just reflects a receive from the given
// peer and has no associated request.
func (wu *WorkUnit) addRequest(p *peer.Peer, requestedTxHash hornet.Hash) {
	wu.requestsLock.Lock()
	defer wu.requestsLock.Unlock()
	wu.requests = append(wu.requests, &Request{
		p:               p,
		requestedTxHash: requestedTxHash,
	})
}

// replies to all requests within this WorkUnit.
func (wu *WorkUnit) replyToAllRequests(requestQueue rqueue.Queue) {
	wu.requestsLock.Lock()
	defer wu.requestsLock.Unlock()

	if len(wu.requests) == 0 {
		return
	}

	for _, peerRequest := range wu.requests {
		// this request might simply just represent that we received the underlying
		// WorkUnit's transaction from the given peer
		if peerRequest.Empty() {
			continue
		}

		// if requested transaction hash is equal to the hash of the received transaction
		// it means that the given peer is synchronized
		isPeerSynced := bytes.Equal(wu.receivedTxHash, peerRequest.requestedTxHash)

		var cachedTxToSend *tangle.CachedTransaction

		// load requested transaction
		if !isPeerSynced {
			if len(peerRequest.requestedTxHash) != 49 {
				peerRequest.p.Metrics.InvalidRequests.Inc()
				metrics.SharedServerMetrics.InvalidRequests.Inc()
				continue
			}

			cachedTxToSend = tangle.GetCachedTransactionOrNil(hornet.Hash(peerRequest.requestedTxHash)) // tx +1
		}

		request := requestQueue.Next()

		// if the peer is synced  and we have no request ourselves,
		// we don't need to reply
		if isPeerSynced && request == nil {
			continue
		}

		if cachedTxToSend == nil {
			if request == nil {
				// we don't reply since we don't have the requested transaction
				// and neither something ourselves we need to request
				continue
			}

			// to keep up the ping-pong between peers which communicate with only
			// legacy messages, we send as our answer to the requested transaction
			// the genesis transaction, to still reply with an own needed transaction request.

			cachedGenesisTx := tangle.GetCachedTransactionOrNil(hornet.NullHashBytes) // tx +1
			if cachedGenesisTx == nil {
				panic("genesis transaction not installed")
			}

			cachedTxToSend = cachedGenesisTx
		}

		// if we have no request ourselves, we use the hash of the transaction which we
		// send in order to signal that we are synchronized.
		var ownRequestHash hornet.Hash
		if request == nil {
			ownRequestHash = cachedTxToSend.GetTransaction().GetTxHash()
		} else {
			ownRequestHash = request.Hash
		}
		transactionAndRequestMsg, _ := legacy.NewTransactionAndRequestMessage(cachedTxToSend.GetTransaction().RawBytes, ownRequestHash)
		cachedTxToSend.Release(true) // tx -1
		peerRequest.p.EnqueueForSending(transactionAndRequestMsg)
	}

	// We processed all the replies, so forget the requests
	wu.requests = []*Request{}
}

// punishes, respectively increases the invalid transaction metric of all peers
// which sent the given underlying transaction of this WorkUnit.
func (wu *WorkUnit) punish() {
	wu.requestsLock.Lock()
	defer wu.requestsLock.Unlock()
	for _, r := range wu.requests {
		r.Punish()
	}
}

// increases the stale transaction metric of all peers
// which sent the given underlying transaction of this WorkUnit.
func (wu *WorkUnit) stale() {
	wu.requestsLock.Lock()
	defer wu.requestsLock.Unlock()
	for _, r := range wu.requests {
		r.Stale()
	}
}

// builds a Broadcast where all peers which are associated with this WorkUnit are excluded from.
func (wu *WorkUnit) broadcast() *bqueue.Broadcast {
	wu.requestsLock.Lock()
	defer wu.requestsLock.Unlock()
	exclude := map[string]struct{}{}
	for _, req := range wu.requests {
		exclude[req.p.ID] = struct{}{}
	}
	return &bqueue.Broadcast{
		TxData:          wu.receivedTxBytes,
		RequestedTxHash: wu.receivedTxHash,
		ExcludePeers:    exclude,
	}
}
