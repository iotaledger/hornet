package bqueue

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/peering"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol/helpers"
	"github.com/gohornet/hornet/pkg/protocol/rqueue"
	"github.com/gohornet/hornet/pkg/protocol/sting"
)

// Broadcast defines a transaction and requested transaction hash which should be broadcasted.
type Broadcast struct {
	// The byte encoded transaction data to broadcast.
	TxData []byte
	// The requested transaction hash.
	RequestedTxHash hornet.Hash
	// The IDs of the peers to exclude from broadcasting.
	ExcludePeers map[string]struct{}
}

// Size defines the default size of the broadcast queue.
const Size = 1000

// Queue implements a queue which broadcasts its elements to all wanted peers.
type Queue interface {
	// EnqueueForBroadcast enqueues the given broadcast to be sent to all peers.
	EnqueueForBroadcast(b *Broadcast)
	// Run runs the broadcast queue.
	Run(shutdownSignal <-chan struct{})
}

// New creates a new Queue.
func New(manager *peering.Manager, reqQueue rqueue.Queue) Queue {
	return &queue{c: make(chan *Broadcast, Size), manager: manager, reqQueue: reqQueue}
}

// queue is a broadcast queue which sends the given messages to all peers.
type queue struct {
	c        chan *Broadcast
	manager  *peering.Manager
	reqQueue rqueue.Queue
}

func (bc *queue) EnqueueForBroadcast(b *Broadcast) {
	bc.c <- b
}

func (bc *queue) Run(shutdownSignal <-chan struct{}) {
	for {
		select {
		case <-shutdownSignal:
			return
		case b := <-bc.c:
			bc.manager.ForAllConnected(func(p *peer.Peer) bool {
				if _, excluded := b.ExcludePeers[p.ID]; excluded {
					return true
				}

				// just send the transaction when the peer supports STING
				if p.Protocol.Supports(sting.FeatureSet) {
					helpers.SendTransaction(p, b.TxData)
					return true
				}

				return true
			})
		}
	}
}
