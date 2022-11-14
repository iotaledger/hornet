//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package gossip_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/logger"
	hivep2p "github.com/iotaledger/hive.go/core/p2p"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/p2p"
	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
)

const protocolID = "/iota/abcdf/1.0.0"

func newNode(ctx context.Context, name string, t *testing.T, mngOpts []p2p.ManagerOption, srvOpts []gossip.ServiceOption, privateKey crypto.PrivKey) (
	host.Host, *p2p.Manager, *gossip.Service, peer.AddrInfo,
) {
	connManager, err := connmgr.NewConnManager(
		1,
		100,
		connmgr.WithGracePeriod(0),
	)
	require.NoError(t, err)

	//nolint:contextcheck // false positive
	n, err := libp2p.New(
		libp2p.DefaultListenAddrs,
		libp2p.Identity(privateKey),
		libp2p.ConnectionManager(connManager),
		libp2p.Transport(tcp.NewTCPTransport),
	)
	require.NoError(t, err)

	serverMetrics := &metrics.ServerMetrics{}

	nLogger := logger.NewLogger(fmt.Sprintf("%s/%s", name, n.ID().ShortString()))

	nManager := p2p.NewManager(n, append(mngOpts, p2p.WithManagerLogger(nLogger))...)
	go nManager.Start(ctx)

	service := gossip.NewService(protocolID, n, nManager, serverMetrics, append(srvOpts, gossip.WithLogger(nLogger))...)
	go service.Start(ctx)

	return n, nManager, service, peer.AddrInfo{ID: n.ID(), Addrs: n.Addrs()}
}

func TestServiceEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := configuration.New()
	err := cfg.Set("logger.disableStacktrace", true)
	require.NoError(t, err)

	// no need to check the error, since the global logger could already be initialized
	_ = logger.InitGlobalLogger(cfg)

	mngOpts := []p2p.ManagerOption{
		p2p.WithManagerReconnectInterval(1*time.Second, 500*time.Millisecond),
	}
	var srvOpts []gossip.ServiceOption

	node1PrvKey, err := hivep2p.ParseLibp2pEd25519PrivateKeyFromString("5536d0d7eb7cb3780085d73d55079a373a726df58010d881167add08d7e8108c76d7a7f15c094c292faa22ac81b976034f0b11db86a8863d9a9b0c64820e087d")
	require.NoError(t, err)

	node2PrvKey, err := hivep2p.ParseLibp2pEd25519PrivateKeyFromString("35764adaa5e02cbd677285ffd90f927644d2010dca7608876dd3ea3a44f8fcb491cdffa377a307e1d16df5c18e4beee9fffbd61998bd1f8c76a616c1b6c7ca7d")
	require.NoError(t, err)

	// node 1 <peer.ID 12*rd6tBe>
	node1, node1Manager, node1Service, node1AddrInfo := newNode(ctx, "node1", t, mngOpts, srvOpts, node1PrvKey)

	// node 2 <peer.ID 12*g1issS>
	node2, node2Manager, node2Service, node2AddrInfo := newNode(ctx, "node2", t, mngOpts, srvOpts, node2PrvKey)

	fmt.Println("node 1", node1.ID().ShortString())
	fmt.Println("node 2", node2.ID().ShortString())

	var protocolStartedCalled1, protocolStartedCalled2 bool
	node1Service.Events.ProtocolStarted.Hook(events.NewClosure(func(_ *gossip.Protocol) {
		protocolStartedCalled1 = true
	}))
	node2Service.Events.ProtocolStarted.Hook(events.NewClosure(func(_ *gossip.Protocol) {
		protocolStartedCalled2 = true
	}))

	// connect node 1 and 2 to each other
	go func() {
		_ = node1Manager.ConnectPeer(&node2AddrInfo, p2p.PeerRelationKnown)
	}()
	time.Sleep(100 * time.Millisecond)
	go func() {
		_ = node2Manager.ConnectPeer(&node1AddrInfo, p2p.PeerRelationKnown)
	}()

	// should eventually both be connected to each other
	connectivity(t, node1Manager, node2.ID(), false, 10*time.Second)
	connectivity(t, node2Manager, node1.ID(), false, 10*time.Second)

	// and because both are known to each other, gossip protocol streams should
	// have been instantiated
	require.Eventually(t, func() bool {
		return node1Service.Protocol(node2.ID()) != nil
	}, 10*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return node2Service.Protocol(node1.ID()) != nil
	}, 10*time.Second, 10*time.Millisecond)

	require.True(t, protocolStartedCalled1)
	require.True(t, protocolStartedCalled2)

	var protocolTerminatedCalled1, protocolTerminatedCalled2 bool
	node1Service.Events.ProtocolTerminated.Hook(events.NewClosure(func(_ *gossip.Protocol) {
		protocolTerminatedCalled1 = true
	}))
	node2Service.Events.ProtocolTerminated.Hook(events.NewClosure(func(_ *gossip.Protocol) {
		protocolTerminatedCalled2 = true
	}))

	// disconnecting them should also clean up the gossip protocol streams.
	// we also explicitly disconnect node 1 to remove the relation state
	go func() {
		_ = node1Manager.DisconnectPeer(node2.ID())
	}()
	go func() {
		_ = node2Manager.DisconnectPeer(node1.ID())
	}()

	connectivity(t, node1Manager, node2.ID(), true)
	connectivity(t, node2Manager, node1.ID(), true)
	require.Eventually(t, func() bool {
		return node1Service.Protocol(node2.ID()) == nil
	}, 4*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return node2Service.Protocol(node1.ID()) == nil
	}, 4*time.Second, 10*time.Millisecond)

	require.True(t, protocolTerminatedCalled1)
	require.True(t, protocolTerminatedCalled2)

	connectivity(t, node1Manager, node2.ID(), true, 5*time.Second)
	connectivity(t, node2Manager, node1.ID(), true, 5*time.Second)

	// if we now connect node 1 to 2 with relation 'known'
	// but node 2 doesn't see node 1 as 'known', the protocol should
	// be started and terminated immediately

	protocolStartedCalled1 = false
	protocolTerminatedCalled1 = false
	node1Service.Events.ProtocolStarted.Hook(events.NewClosure(func(_ *gossip.Protocol) {
		protocolStartedCalled1 = true
	}))
	node1Service.Events.ProtocolTerminated.Hook(events.NewClosure(func(_ *gossip.Protocol) {
		protocolTerminatedCalled1 = true
	}))

	go func() {
		_ = node1Manager.ConnectPeer(&node2AddrInfo, p2p.PeerRelationKnown)
	}()

	require.Eventually(t, func() bool { return protocolStartedCalled1 }, 10*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return protocolTerminatedCalled1 }, 10*time.Second, 10*time.Millisecond)
}

func connectivity(t *testing.T, source *p2p.Manager, target peer.ID, disconnected bool, overrideCheckDuration ...time.Duration) {
	dur := 4 * time.Second
	if len(overrideCheckDuration) > 0 {
		dur = overrideCheckDuration[0]
	}
	require.Eventually(t, func() bool {
		return source.IsConnected(target) == !disconnected
	}, dur, 100*time.Millisecond)
}

func TestWithUnknownPeersLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := configuration.New()
	err := cfg.Set("logger.disableStacktrace", true)
	require.NoError(t, err)

	// no need to check the error, since the global logger could already be initialized
	_ = logger.InitGlobalLogger(cfg)

	mngOpts := []p2p.ManagerOption{
		p2p.WithManagerReconnectInterval(2*time.Second, 500*time.Millisecond),
	}
	srvOpts := []gossip.ServiceOption{
		gossip.WithUnknownPeersLimit(1),
	}

	node1PrvKey, err := hivep2p.ParseLibp2pEd25519PrivateKeyFromString("5536d0d7eb7cb3780085d73d55079a373a726df58010d881167add08d7e8108c76d7a7f15c094c292faa22ac81b976034f0b11db86a8863d9a9b0c64820e087d")
	require.NoError(t, err)

	node2PrvKey, err := hivep2p.ParseLibp2pEd25519PrivateKeyFromString("35764adaa5e02cbd677285ffd90f927644d2010dca7608876dd3ea3a44f8fcb491cdffa377a307e1d16df5c18e4beee9fffbd61998bd1f8c76a616c1b6c7ca7d")
	require.NoError(t, err)

	node3PrvKey, err := hivep2p.ParseLibp2pEd25519PrivateKeyFromString("1d586a941f97be3d8ead709c9ff31579c9677f681ec05cd1e0233d36513b178bd2a54ee6c67c84037ae8da89033c1bcfc2252ecd466f6cf472c22cbe0e9a7842")
	require.NoError(t, err)

	// node 1 <peer.ID 12*rd6tBe>
	node1, node1Manager, node1Service, node1AddrInfo := newNode(ctx, "node1", t, mngOpts, srvOpts, node1PrvKey)

	// node 2 <peer.ID 12*g1issS>
	node2, node2Manager, node2Service, node2AddrInfo := newNode(ctx, "node2", t, mngOpts, srvOpts, node2PrvKey)

	// node 3 <peer.ID 12*e9jADP>
	node3, node3Manager, node3Service, _ := newNode(ctx, "node3", t, mngOpts, []gossip.ServiceOption{
		gossip.WithUnknownPeersLimit(2),
	}, node3PrvKey)

	fmt.Println("node 1", node1.ID().ShortString())
	fmt.Println("node 2", node2.ID().ShortString())
	fmt.Println("node 3", node3.ID().ShortString())

	var protocolStartedCalled1, protocolStartedCalled2 bool
	node1Service.Events.ProtocolStarted.Hook(events.NewClosure(func(_ *gossip.Protocol) {
		protocolStartedCalled1 = true
	}))
	node2Service.Events.ProtocolStarted.Hook(events.NewClosure(func(_ *gossip.Protocol) {
		protocolStartedCalled2 = true
	}))

	// connect node 1 and 2 to each other
	go func() {
		_ = node1Manager.ConnectPeer(&node2AddrInfo, p2p.PeerRelationUnknown)
	}()
	time.Sleep(100 * time.Millisecond)
	go func() {
		_ = node2Manager.ConnectPeer(&node1AddrInfo, p2p.PeerRelationUnknown)
	}()

	// should eventually both be connected to each other
	connectivity(t, node1Manager, node2.ID(), false, 10*time.Second)
	connectivity(t, node2Manager, node1.ID(), false, 10*time.Second)

	// and because both peers allow one unknown peer in the gossip service,
	// gossip protocol streams should have been instantiated
	require.Eventually(t, func() bool {
		return node1Service.Protocol(node2.ID()) != nil
	}, 10*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return node2Service.Protocol(node1.ID()) != nil
	}, 10*time.Second, 10*time.Millisecond)

	require.True(t, protocolStartedCalled1)
	require.True(t, protocolStartedCalled2)

	// now lets verify that node 3 can't build a gossip stream to neither node 1 and 2 since both
	// have their available slots filled
	var node3ProtocolTerminated int
	node3Service.Events.ProtocolTerminated.Hook(events.NewClosure(func(_ *gossip.Protocol) {
		node3ProtocolTerminated++
	}))

	// reset
	protocolStartedCalled1, protocolStartedCalled2 = false, false

	go func() {
		_ = node3Manager.ConnectPeer(&node1AddrInfo, p2p.PeerRelationUnknown)
	}()
	go func() {
		_ = node3Manager.ConnectPeer(&node2AddrInfo, p2p.PeerRelationUnknown)
	}()

	// no protocols should have been started on node 1 and 2
	require.Never(t, func() bool {
		return protocolStartedCalled1 && protocolStartedCalled2
	}, 4*time.Second, 10*time.Millisecond)

	// 2 protocols should have been terminated on node 3
	require.Eventually(t, func() bool {
		return node3ProtocolTerminated == 2
	}, 4*time.Second, 10*time.Millisecond)
}
