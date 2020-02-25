package gossip

import (
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/packages/compressed"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/plugins/gossip/server"
)

type NeighborRequest struct {
	p                          *protocol
	reqHashBytes               []byte
	reqMilestoneIndex          milestone_index.MilestoneIndex
	isMilestoneRequest         bool
	isTransactionRequest       bool
	isLegacyTransactionRequest bool
	hasNoRequest               bool
}

func (n *NeighborRequest) punish() {
	n.p.Neighbor.Metrics.IncrInvalidTransactionsCount()
}

func (n *NeighborRequest) notify(recHashBytes []byte) {
	n.p.Neighbor.Reply(recHashBytes, n)
}

type PendingNeighborRequests struct {
	objectstorage.StorableObjectFlags
	startProcessingLock syncutils.Mutex

	// data
	dataLock     syncutils.RWMutex
	recTxBytes   []byte
	recHashBytes []byte
	recHash      trinary.Hash
	hornetTx     *hornet.Transaction

	// status
	statusLock syncutils.RWMutex
	invalid    bool
	hashing    bool

	// requests
	requestsLock syncutils.RWMutex
	requests     []*NeighborRequest
}

// ObjectStorage interface
func (p *PendingNeighborRequests) Update(other objectstorage.StorableObject) {
	panic("PendingNeighborRequests should never be updated")
	/*
		if obj, ok := other.(*PendingNeighborRequests); !ok {
			panic("invalid object passed to PendingNeighborRequests.Update()")
		} else {
			// data
			p.recTxBytes = obj.recTxBytes
			p.recHashBytes = obj.recHashBytes
			p.recHash = obj.recHash
			p.hornetTx = obj.hornetTx

			// status
			p.invalid = obj.invalid
			p.hashing = obj.hashing

			// requests
			p.requests = obj.requests
		}
	*/
}

func (p *PendingNeighborRequests) GetStorageKey() []byte {
	return p.recTxBytes
}

func (p *PendingNeighborRequests) MarshalBinary() (data []byte, err error) {
	return nil, nil
}

func (p *PendingNeighborRequests) UnmarshalBinary(data []byte) error {
	return nil
}

func (p *PendingNeighborRequests) AddLegacyTxRequest(neighbor *protocol, reqHashBytes []byte) {
	p.requestsLock.Lock()
	defer p.requestsLock.Unlock()

	p.requests = append(p.requests, &NeighborRequest{
		p:                          neighbor,
		reqHashBytes:               reqHashBytes,
		isLegacyTransactionRequest: true,
	})
}

func (p *PendingNeighborRequests) BlockFeedback(neighbor *protocol) {
	p.requestsLock.Lock()
	defer p.requestsLock.Unlock()

	p.requests = append(p.requests, &NeighborRequest{
		p:            neighbor,
		hasNoRequest: true,
	})
}

func (p *PendingNeighborRequests) IsHashing() bool {
	p.statusLock.RLock()
	defer p.statusLock.RUnlock()
	return p.hashing
}

func (p *PendingNeighborRequests) IsHashed() bool {
	p.statusLock.RLock()
	defer p.statusLock.RUnlock()
	return len(p.recHashBytes) > 0
}

func (p *PendingNeighborRequests) IsInvalid() bool {
	p.statusLock.RLock()
	defer p.statusLock.RUnlock()
	return p.invalid
}

func (p *PendingNeighborRequests) GetTxHash() trinary.Hash {
	p.dataLock.RLock()
	defer p.dataLock.RUnlock()
	return p.recHash
}

func (p *PendingNeighborRequests) GetTxHashBytes() []byte {
	p.dataLock.RLock()
	defer p.dataLock.RUnlock()
	return p.recHashBytes
}

func (p *PendingNeighborRequests) process() {
	p.startProcessingLock.Lock()

	if p.IsHashing() {
		p.startProcessingLock.Unlock()
		return
	} else if p.IsInvalid() {
		p.startProcessingLock.Unlock()
		p.punish()
		return
	} else if p.IsHashed() {
		p.startProcessingLock.Unlock()

		// Mark the pending request as received because we received the requested Tx Hash
		requested, reqMilestoneIndex := RequestQueue.MarkReceived(p.hornetTx.Tx.Hash)

		if requested {
			// Tx is requested => ignore that it was marked as stale before
			Events.ReceivedTransaction.Trigger(p.hornetTx, requested, reqMilestoneIndex)
		}

		p.notify()
		return
	}

	p.statusLock.Lock()
	p.hashing = true
	p.statusLock.Unlock()
	p.startProcessingLock.Unlock()

	tx, err := compressed.TransactionFromCompressedBytes(p.recTxBytes)
	if err != nil {
		return
	}

	if !transaction.HasValidNonce(tx, ownMWM) {
		// PoW is invalid => punish neighbor
		p.statusLock.Lock()
		p.invalid = true
		p.statusLock.Unlock()

		// Do not answer
		p.punish()
		return
	}

	// Mark the pending request as received because we received the requested Tx Hash
	requested, reqMilestoneIndex := RequestQueue.MarkReceived(tx.Hash)

	// POW valid => Process the message
	hornetTx := hornet.NewTransaction(tx, p.recTxBytes)

	// received tx was not requested and has an invalid timestamp (maybe before snapshot?)
	// => do not store in our database
	// => we need to reply to answer the neighbors request
	timeValid, broadcast := checkTimestamp(hornetTx)
	stale := !requested && !timeValid
	recHashBytes := trinary.MustTrytesToBytes(tx.Hash)[:49]

	p.dataLock.Lock()
	p.recHash = tx.Hash
	p.recHashBytes = recHashBytes
	p.hornetTx = hornetTx
	p.dataLock.Unlock()

	p.statusLock.Lock()
	p.hashing = false
	p.statusLock.Unlock()

	if !stale {
		// Ignore stale transactions until they are requested
		Events.ReceivedTransaction.Trigger(hornetTx, requested, reqMilestoneIndex)

		if !requested && broadcast {
			p.broadcast()
		}
	} else if len(p.requests) == 1 {
		p.requests[0].p.Neighbor.Metrics.IncrInvalidTransactionsCount()
	}

	p.notify()
}

func (p *PendingNeighborRequests) notify() {
	p.requestsLock.Lock()

	for _, n := range p.requests {
		n.notify(p.recHashBytes)
	}

	p.requests = make([]*NeighborRequest, 0)
	p.requestsLock.Unlock()
}

func (p *PendingNeighborRequests) punish() {
	p.requestsLock.Lock()

	for _, n := range p.requests {
		// Tx is known as invalid => punish neighbor
		server.SharedServerMetrics.IncrInvalidTransactionsCount()
		n.p.Neighbor.Metrics.IncrInvalidTransactionsCount()
		n.punish()
	}

	p.requests = make([]*NeighborRequest, 0)
	p.requestsLock.Unlock()
}

func (p *PendingNeighborRequests) broadcast() {
	p.requestsLock.RLock()

	excludedNeighbors := make(map[string]struct{})
	for _, neighbor := range p.requests {
		excludedNeighbors[neighbor.p.Neighbor.Identity] = struct{}{}
	}
	p.requestsLock.RUnlock()

	BroadcastTransaction(excludedNeighbors, p.recTxBytes, p.GetTxHashBytes())
}
