package peer

import (
	"net"
	"strconv"
	"strings"

	"github.com/iotaledger/hive.go/events"
	"go.uber.org/atomic"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/iputils"
	"github.com/iotaledger/hive.go/network"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/protocol"
	"github.com/gohornet/hornet/pkg/protocol/sting"
)

// ConnectionOrigin defines whether a connection was initialized inbound or outbound.
type ConnectionOrigin byte

const (
	// Inbound connection.
	Inbound ConnectionOrigin = iota
	// Outbound connection.
	Outbound
)

// SendQueueSize defines the size of the send queue of every created peer.
const SendQueueSize = 1500

func Caller(handler interface{}, params ...interface{}) {
	handler.(func(*Peer))(params[0].(*Peer))
}

func OriginAddressCaller(handler interface{}, params ...interface{}) {
	handler.(func(*iputils.OriginAddress))(params[0].(*iputils.OriginAddress))
}

// NewInboundPeer creates a new peer instance which is marked as being inbound.
func NewInboundPeer(remoteAddr net.Addr) *Peer {
	addresses := iputils.NewIPAddresses()
	primaryAddr := net.ParseIP(remoteAddr.(*net.TCPAddr).IP.String())
	addresses.Add(primaryAddr)

	// InitAddress and ID are set after handshaking
	return &Peer{
		PrimaryAddress:   primaryAddr,
		Addresses:        addresses,
		ConnectionOrigin: Inbound,
		SendQueue:        make(chan []byte, SendQueueSize),
		Events: Events{
			HeartbeatUpdated: events.NewEvent(sting.HeartbeatCaller),
		},
	}
}

// NewOutboundPeer creates a new peer instance which is marked as being outbound.
func NewOutboundPeer(originAddr *iputils.OriginAddress, primaryAddr net.IP, port uint16, addresses *iputils.IPAddresses) *Peer {
	return &Peer{
		InitAddress:             originAddr,
		ID:                      NewID(primaryAddr.String(), port),
		PrimaryAddress:          primaryAddr,
		Addresses:               addresses,
		MoveBackToReconnectPool: true,
		ConnectionOrigin:        Outbound,
		SendQueue:               make(chan []byte, SendQueueSize),
		Events: Events{
			HeartbeatUpdated: events.NewEvent(sting.HeartbeatCaller),
		},
	}
}

// Events happening on the peer instance.
type Events struct {
	HeartbeatUpdated *events.Event
}

// Peer is a node to which the node is connected to.
type Peer struct {
	// The ip/port combination of the peer.
	ID string
	// The underlying connection of the peer.
	Conn *network.ManagedConnection
	// The original address of this peer.
	InitAddress *iputils.OriginAddress
	// The address IP address under which the peer is connected.
	PrimaryAddress net.IP
	// The IP addresses which were looked up during peer initialisation.
	Addresses *iputils.IPAddresses
	// The protocol instance under which this peer operates.
	Protocol *protocol.Protocol
	// Metrics about the peer.
	Metrics Metrics
	// Whether the connection for this peer was handled inbound or was created outbound.
	ConnectionOrigin ConnectionOrigin
	// Whether to place this peer back into the reconnect pool when the connection is closed.
	MoveBackToReconnectPool bool
	// Whether the peer is a duplicate, as it is already connected.
	Duplicate bool
	// The peer's latest heartbeat message.
	LatestHeartbeat *sting.Heartbeat
	// Holds the autopeering info if this peer was added via autopeering.
	Autopeering *peer.Peer
	// A channel which contains messages to be sent to the given peer.
	SendQueue chan []byte
	// Whether this peer is marked as disconnected.
	// Used to suppress errors stemming from connection closure.
	Disconnected bool
	// Events happening on the peer.
	Events Events
}

// IsInbound tells whether the peer's connection was inbound.
func (p *Peer) IsInbound() bool {
	return p.ConnectionOrigin == Inbound
}

// EnqueueForSending enqueues the given data to be sent to the peer.
// If it can't because the send queue is over capacity, the message gets dropped.
func (p *Peer) EnqueueForSending(data []byte) {
	select {
	case p.SendQueue <- data:
	default:
		p.Metrics.DroppedMessages.Inc()
	}
}

// Info returns a snapshot of the peer in time of calling Info().
func (p *Peer) Info() *Info {
	info := &Info{
		Peer:                           p,
		Address:                        p.ID,
		Port:                           p.InitAddress.Port,
		Domain:                         p.InitAddress.Addr,
		DomainWithPort:                 p.InitAddress.String(),
		Alias:                          p.InitAddress.Alias,
		PreferIPv6:                     p.InitAddress.PreferIPv6,
		NumberOfAllTransactions:        p.Metrics.ReceivedTransactions.Load(),
		NumberOfNewTransactions:        p.Metrics.NewTransactions.Load(),
		NumberOfKnownTransactions:      p.Metrics.KnownTransactions.Load(),
		NumberOfInvalidTransactions:    p.Metrics.InvalidTransactions.Load(),
		NumberOfInvalidRequests:        p.Metrics.InvalidRequests.Load(),
		NumberOfStaleTransactions:      p.Metrics.StaleTransactions.Load(),
		NumberOfReceivedTransactionReq: p.Metrics.ReceivedTransactionRequests.Load(),
		NumberOfReceivedMilestoneReq:   p.Metrics.ReceivedMilestoneRequests.Load(),
		NumberOfReceivedHeartbeats:     p.Metrics.ReceivedHeartbeats.Load(),
		NumberOfSentTransactions:       p.Metrics.SentTransactions.Load(),
		NumberOfSentTransactionsReq:    p.Metrics.SentTransactionRequests.Load(),
		NumberOfSentMilestoneReq:       p.Metrics.SentMilestoneRequests.Load(),
		NumberOfSentHeartbeats:         p.Metrics.SentHeartbeats.Load(),
		NumberOfDroppedSentPackets:     p.Metrics.DroppedMessages.Load(),
		ConnectionType:                 "tcp",
		Connected:                      false,
		AutopeeringID:                  "",
	}
	if p.Autopeering != nil {
		info.AutopeeringID = p.Autopeering.ID().String()
	}
	return info
}

// HasDataFor tells whether the peer given the latest heartbeat message, has the cone data for the given milestone.
// Returns false if no heartbeat message was received yet.
func (p *Peer) HasDataFor(index milestone.Index) bool {
	if p.LatestHeartbeat == nil {
		return false
	}
	return p.LatestHeartbeat.PrunedMilestoneIndex < index && p.LatestHeartbeat.SolidMilestoneIndex >= index
}

// Handshaked tells whether the peer was handshaked.
func (p *Peer) Handshaked() bool {
	return p.Protocol != nil && p.Protocol.FeatureSet != 0
}

// NewID returns a peer ID which consists of the given IP address and server socket port number.
func NewID(ip string, port uint16) string {
	// prevent double square brackets
	ip = strings.ReplaceAll(ip, "[", "")
	ip = strings.ReplaceAll(ip, "]", "")
	return net.JoinHostPort(ip, strconv.FormatUint(uint64(port), 10))
}

// Metrics defines a set of metrics regarding a peer.
type Metrics struct {
	// The number of received transactions which are new.
	NewTransactions atomic.Uint32
	// The number of received transactions which are already known.
	KnownTransactions atomic.Uint32
	// The number of received invalid transactions.
	InvalidTransactions atomic.Uint32
	// The number of received transactions of which their timestamp is stale.
	StaleTransactions atomic.Uint32
	// The number of received invalid requests (both transactions and milestones).
	InvalidRequests atomic.Uint32
	// The number of received transactions.
	ReceivedTransactions atomic.Uint32
	// The number of received transaction requests.
	ReceivedTransactionRequests atomic.Uint32
	// The number of received milestone requests.
	ReceivedMilestoneRequests atomic.Uint32
	// The number of received heartbeats.
	ReceivedHeartbeats atomic.Uint32
	// The number of sent transactions.
	SentTransactions atomic.Uint32
	// The number of sent transaction requests.
	SentTransactionRequests atomic.Uint32
	// The number of sent milestone requests.
	SentMilestoneRequests atomic.Uint32
	// The number of sent heartbeats.
	SentHeartbeats atomic.Uint32
	// The number of dropped messages.
	DroppedMessages atomic.Uint32
}

// Info acts as a static snapshot of information about a peer.
type Info struct {
	Peer                           *Peer  `json:"-"`
	Address                        string `json:"address"`
	Port                           uint16 `json:"port,omitempty"`
	Domain                         string `json:"domain,omitempty"`
	DomainWithPort                 string `json:"-"`
	Alias                          string `json:"alias,omitempty"`
	PreferIPv6                     bool   `json:"-"`
	NumberOfAllTransactions        uint32 `json:"numberOfAllTransactions"`
	NumberOfNewTransactions        uint32 `json:"numberOfNewTransactions"`
	NumberOfKnownTransactions      uint32 `json:"numberOfKnownTransactions"`
	NumberOfInvalidTransactions    uint32 `json:"numberOfInvalidTransactions"`
	NumberOfInvalidRequests        uint32 `json:"numberOfInvalidRequests"`
	NumberOfStaleTransactions      uint32 `json:"numberOfStaleTransactions"`
	NumberOfReceivedTransactionReq uint32 `json:"numberOfReceivedTransactionReq"`
	NumberOfReceivedMilestoneReq   uint32 `json:"numberOfReceivedMilestoneReq"`
	NumberOfReceivedHeartbeats     uint32 `json:"numberOfReceivedHeartbeats"`
	NumberOfSentTransactions       uint32 `json:"numberOfSentTransactions"`
	NumberOfSentTransactionsReq    uint32 `json:"numberOfSentTransactionsReq"`
	NumberOfSentMilestoneReq       uint32 `json:"numberOfSentMilestoneReq"`
	NumberOfSentHeartbeats         uint32 `json:"numberOfSentHeartbeats"`
	NumberOfDroppedSentPackets     uint32 `json:"numberOfDroppedSentPackets"`
	ConnectionType                 string `json:"connectionType"`
	Connected                      bool   `json:"connected"`
	AutopeeringID                  string `json:"autopeeringId,omitempty"`
}
