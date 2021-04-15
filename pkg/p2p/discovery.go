package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/multiformats/go-multiaddr"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/timeutil"
)

const (
	defaultRendevousPoint            = "between-two-vertices"
	defaultAdvertiseInterval         = 30 * time.Second
	defaultRoutingTableRefreshPeriod = 1 * time.Minute
	defaultMaxDiscoveredPeers        = 4
)

// DiscoveryServiceEvents are events happening around a DiscoveryService.
// No methods on DiscoveryService must be called from within the event handlers.
type DiscoveryServiceEvents struct {
	// Fired when a search is started.
	Searching *events.Event
	// Fired when a search has ended.
	Searched *events.Event
	// Fired when a peer has been discovered for peering.
	Discovered *events.Event
	// Fired when a Kademlia DHT stream is started.
	ProtocolStarted *events.Event
	// Fired when a Kademlia DHT stream is terminated
	ProtocolTerminated *events.Event
	// Fired when an error occurs.
	Error *events.Event
}

// AddrInfoCaller gets called with a peer.AddrInfo.
func AddrInfoCaller(handler interface{}, params ...interface{}) {
	handler.(func(peer.AddrInfo))(params[0].(peer.AddrInfo))
}

// StreamCaller gets called with a network.Stream.
func StreamCaller(handler interface{}, params ...interface{}) {
	handler.(func(network.Stream))(params[0].(network.Stream))
}

// DiscoveryServiceOptions define options for a DiscoveryService.
type DiscoveryServiceOptions struct {
	RendezvousPoint           string
	AdvertiseInterval         time.Duration
	RoutingTableRefreshPeriod time.Duration
	MaxDiscoveredPeers        int
	Logger                    *logger.Logger
}

// applies the given ServiceOption.
func (dso *DiscoveryServiceOptions) apply(opts ...DiscoveryServiceOption) {
	for _, opt := range opts {
		opt(dso)
	}
}

var defaultDiscoveryServiceOptions = []DiscoveryServiceOption{
	WithDiscoveryServiceRendezvousPoint(defaultRendevousPoint),
	WithDiscoveryServiceAdvertiseInterval(defaultAdvertiseInterval),
	WithDiscoveryServiceRoutingRefreshPeriod(defaultRoutingTableRefreshPeriod),
	WithDiscoveryServiceMaxDiscoveredPeers(defaultMaxDiscoveredPeers),
}

// DiscoveryServiceOption is a function setting a DiscoveryServiceOptions option.
type DiscoveryServiceOption func(opts *DiscoveryServiceOptions)

// WithDiscoveryServiceRendezvousPoint sets the rendezvous point for peer discovery.
func WithDiscoveryServiceRendezvousPoint(rendezvousPoint string) DiscoveryServiceOption {
	return func(opts *DiscoveryServiceOptions) {
		opts.RendezvousPoint = rendezvousPoint
	}
}

// WithDiscoveryServiceAdvertiseInterval sets the interval in which the peer
// advertises itself via the DHT on the rendezvous point.
func WithDiscoveryServiceAdvertiseInterval(interval time.Duration) DiscoveryServiceOption {
	return func(opts *DiscoveryServiceOptions) {
		opts.AdvertiseInterval = interval
	}
}

// WithDiscoveryServiceMaxDiscoveredPeers sets the max. amount of discovered
// peers to retain a connection to.
func WithDiscoveryServiceMaxDiscoveredPeers(max int) DiscoveryServiceOption {
	return func(opts *DiscoveryServiceOptions) {
		opts.MaxDiscoveredPeers = max
	}
}

// WithDiscoveryServiceRoutingRefreshPeriod sets the interval in which buckets are
// refreshed in the routing table.
func WithDiscoveryServiceRoutingRefreshPeriod(period time.Duration) DiscoveryServiceOption {
	return func(opts *DiscoveryServiceOptions) {
		opts.RoutingTableRefreshPeriod = period
	}
}

// WithDiscoveryServiceLogger enables logging within the DiscoveryService.
func WithDiscoveryServiceLogger(logger *logger.Logger) DiscoveryServiceOption {
	return func(opts *DiscoveryServiceOptions) {
		opts.Logger = logger
	}
}

// NewDiscoveryService creates a new DiscoveryService.
func NewDiscoveryService(host host.Host, mng *Manager, opts ...DiscoveryServiceOption) *DiscoveryService {
	dso := &DiscoveryServiceOptions{}
	dso.apply(defaultDiscoveryServiceOptions...)
	dso.apply(opts...)
	dhtCtx, dhtCtxCancel := context.WithCancel(context.Background())
	routingRefreshPeriod := dht.RoutingTableRefreshPeriod(dso.RoutingTableRefreshPeriod)
	kademliaDHT, err := dht.New(dhtCtx, host, routingRefreshPeriod, dht.Mode(dht.ModeServer))
	if err != nil {
		panic(err)
	}
	ds := &DiscoveryService{
		Events: DiscoveryServiceEvents{
			Searching:          events.NewEvent(events.IntCaller),
			Searched:           events.NewEvent(events.IntCaller),
			Discovered:         events.NewEvent(AddrInfoCaller),
			ProtocolStarted:    events.NewEvent(StreamCaller),
			ProtocolTerminated: events.NewEvent(StreamCaller),
			Error:              events.NewEvent(events.ErrorCaller),
		},
		opts:         dso,
		host:         host,
		mng:          mng,
		dht:          kademliaDHT,
		dhtCtx:       dhtCtx,
		dhtCtxCancel: dhtCtxCancel,
	}
	if ds.opts.Logger != nil {
		ds.registerLoggerOnEvents()
	}
	return ds
}

// DiscoveryService is a service which discovers to peer to.
type DiscoveryService struct {
	// Events happening around the DiscoveryService.
	Events       DiscoveryServiceEvents
	host         host.Host
	mng          *Manager
	opts         *DiscoveryServiceOptions
	dht          *dht.IpfsDHT
	dhtCtx       context.Context
	dhtCtxCancel context.CancelFunc
}

// Start starts the DiscoveryService.
// This function blocks until shutdownSignal is signaled.
func (ds *DiscoveryService) Start(shutdownSignal <-chan struct{}) {
	ds.host.Network().Notify((*dsNotifiee)(ds))
	// if you look very close, you can see that Bootstrap doesn't even use the context,
	// genius, isn't it?
	if err := ds.dht.Bootstrap(context.Background()); err != nil {
		panic(err)
	}
	timeutil.NewTicker(ds.discover, ds.opts.AdvertiseInterval, shutdownSignal)
	ds.dhtCtxCancel()
	ds.host.Network().StopNotify((*dsNotifiee)(ds))
}

// tries to discover peers to connect to by advertising itself on the DHT on a given
// "rendezvous point".
// btw, this isn't the right way to do it according to also the rendezvous example
// in the libp2p repository and they refer to https://github.com/libp2p/specs/pull/56.
// however, it seems they just merged the spec without any actual implementation yet.
func (ds *DiscoveryService) discover() {
	discoveredPeersCount := ds.mng.ConnectedCount(PeerRelationDiscovered)
	delta := ds.opts.MaxDiscoveredPeers - discoveredPeersCount
	if delta <= 0 {
		return
	}

	ds.Events.Searching.Trigger(delta)

	findPeersCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	routingDiscovery := discovery.NewRoutingDiscovery(ds.dht)
	discovery.Advertise(findPeersCtx, routingDiscovery, ds.opts.RendezvousPoint)

	// TODO: how long does this block etc.? docs don't tell anything
	peerChan, err := routingDiscovery.FindPeers(findPeersCtx, ds.opts.RendezvousPoint)
	if err != nil {
		ds.Events.Error.Trigger(fmt.Errorf("unable to find peers: %w", err))
		return
	}

	var found int
	for addrInfo := range peerChan {
		if delta == 0 {
			continue
		}

		// apparently we can even find ourselves
		if addrInfo.ID == ds.host.ID() {
			continue
		}

		var has bool
		ds.mng.Call(addrInfo.ID, func(p *Peer) {
			has = true
		})
		if has {
			continue
		}

		ds.Events.Discovered.Trigger(addrInfo)
		found++
	}
	ds.Events.Searched.Trigger(found)
}

// registers the logger on the events of the DiscoveryService.
func (ds *DiscoveryService) registerLoggerOnEvents() {
	ds.Events.Searching.Attach(events.NewClosure(func(searchingFor int) {
		ds.opts.Logger.Infof("searching for %d peers...", searchingFor)
	}))
	ds.Events.Searched.Attach(events.NewClosure(func(found int) {
		ds.opts.Logger.Infof("search ended, found %d peers", found)
	}))
	ds.Events.Discovered.Attach(events.NewClosure(func(addrInfo peer.AddrInfo) {
		ds.opts.Logger.Infof("discovered %s: %v", addrInfo.ID.ShortString(), addrInfo.Addrs)
	}))
	ds.Events.ProtocolStarted.Attach(events.NewClosure(func(stream network.Stream) {
		remotePeer := stream.Conn().RemotePeer()
		ds.opts.Logger.Infof("started Kademlia DHT protocol with %s: %v", remotePeer.ShortString(), stream.Conn().RemoteMultiaddr())
	}))
	ds.Events.ProtocolTerminated.Attach(events.NewClosure(func(stream network.Stream) {
		remotePeer := stream.Conn().RemotePeer()
		ds.opts.Logger.Infof("terminated Kademlia DHT protocol with %s: %v", remotePeer.ShortString(), stream.Conn().RemoteMultiaddr())
	}))
	ds.Events.Error.Attach(events.NewClosure(func(err error) {
		ds.opts.Logger.Warn(err)
	}))
}

// lets DiscoveryService implement network.Notifiee
// in order to listen to opened and terminated Kademlia DHT streams.
type dsNotifiee DiscoveryService

func (m *dsNotifiee) Listen(net network.Network, multiaddr multiaddr.Multiaddr)      {}
func (m *dsNotifiee) ListenClose(net network.Network, multiaddr multiaddr.Multiaddr) {}
func (m *dsNotifiee) Connected(net network.Network, conn network.Conn)               {}
func (m *dsNotifiee) Disconnected(net network.Network, conn network.Conn)            {}
func (m *dsNotifiee) OpenedStream(net network.Network, stream network.Stream) {
	/*
		if stream.Protocol() != dht.ProtocolDHT {
			return
		}
		m.Events.ProtocolStarted.Trigger(stream)

	*/
}
func (m *dsNotifiee) ClosedStream(net network.Network, stream network.Stream) {
	/*
		if stream.Protocol() != dht.ProtocolDHT {
			return
		}
		m.Events.ProtocolTerminated.Trigger(stream)

	*/
}
