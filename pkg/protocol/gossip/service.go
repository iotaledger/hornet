package gossip

import (
	"sync"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/iotaledger/hive.go/events"
	"github.com/libp2p/go-libp2p-core/peer"
)

const (
	// broadcastQueueSize defines the default size of the broadcast queue.
	broadcastQueueSize = 1000
)

// ServiceEvents are events happening around a Service.
type ServiceEvents struct {
	// Fired when a new gossip Protocol is created and started.
	Created *events.Event
}

// ProtocolCaller is an event handler called with a Protocol.
func ProtocolCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Protocol))(params[0].(*Protocol))
}

// NewService creates a new gossip Service.
func NewService(msgProc *MessageProcessor, peeringService *p2p.PeeringService, rQueue RequestQueue) *Service {
	return &Service{
		protosMu:      sync.Mutex{},
		protos:        map[peer.ID]*Protocol{},
		broadcastChan: make(chan *Broadcast, broadcastQueueSize),
		Events: ServiceEvents{
			Created: events.NewEvent(ProtocolCaller),
		},
		MessageProcessor: msgProc,
		PeeringService:   peeringService,
		RequestQueue:     rQueue,
	}
}

// Service manages the ongoing Protocol instances.
type Service struct {
	protosMu         sync.Mutex
	protos           map[peer.ID]*Protocol
	broadcastChan    chan *Broadcast
	Events           ServiceEvents
	MessageProcessor *MessageProcessor
	PeeringService   *p2p.PeeringService
	RequestQueue     RequestQueue
}

// Protocol returns the protocol instance for the given peer.
func (s *Service) Protocol(id peer.ID) *Protocol {
	s.protosMu.Lock()
	defer s.protosMu.Unlock()
	p := s.protos[id]
	return p
}

// Register registers the given Protocol with the service and fires
// a ServiceEvents.Created event if no Protocol was registered for the given peer before.
func (s *Service) Register(proto *Protocol) bool {
	s.protosMu.Lock()
	defer s.protosMu.Unlock()
	_, exists := s.protos[proto.PeerID]
	if exists {
		return false
	}
	s.protos[proto.PeerID] = proto
	return true
}

// Unregister unregisters the protocol under the given peer instance.
func (s *Service) Unregister(id peer.ID) bool {
	s.protosMu.Lock()
	defer s.protosMu.Unlock()
	_, exists := s.protos[id]
	if exists {
		return false
	}
	delete(s.protos, id)
	return true
}

// Broadcast defines a message which should be broadcasted.
type Broadcast struct {
	// The message data to broadcast.
	MsgData []byte
	// The IDs of the peers to exclude from broadcasting.
	ExcludePeers map[peer.ID]struct{}
}

// Broadcast broadcasts the given tangle.Message data to all selected peers.
func (s *Service) Broadcast(b *Broadcast) {
	s.broadcastChan <- b
}

// RunBroadcast consumes Broadcast and sends them to all selected peers.
func (s *Service) RunBroadcast(shutdownSignal <-chan struct{}) {
	for {
		select {
		case <-shutdownSignal:
			return
		case b := <-s.broadcastChan:
			s.PeeringService.ForAllConnected(func(p *p2p.Peer) bool {
				if _, excluded := b.ExcludePeers[p.ID]; excluded {
					return true
				}

				proto := s.Protocol(p.ID)
				if proto == nil {
					return true
				}

				proto.SendMessage(b.MsgData)
				return true
			})
		}
	}
}

// SynchronizedCount returns the count of peers which appear to be
// synchronized given their latest Heartbeat message.
func (s *Service) SynchronizedCount(latestMilestoneIndex milestone.Index) int {
	s.protosMu.Lock()
	defer s.protosMu.Unlock()
	var count int
	for _, p := range s.protos {
		if p.IsSynced(latestMilestoneIndex) {
			count++
		}
	}
	return count
}
