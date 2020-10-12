package gossip_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

const protocolID = "/iota/abcdf/1.0.0"

func newNode(ctx context.Context, t *testing.T) host.Host {
	// we use Ed25519 because otherwise it takes longer as the default is RSA
	sk, _, _ := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	h, err := libp2p.New(
		ctx,
		libp2p.Identity(sk),
		libp2p.ConnectionManager(connmgr.NewConnManager(1, 100, 0)),
	)
	require.NoError(t, err)
	return h
}

func TestServiceEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	shutdownSignal := make(chan struct{})
	defer close(shutdownSignal)

	cfg := viper.GetViper()
	cfg.Set("logger.disableStacktrace", true)
	require.NoError(t, logger.InitGlobalLogger(cfg))

	reconnectOpt := p2p.WithReconnectInterval(1*time.Second, 500*time.Millisecond)

	node1 := newNode(ctx, t)
	node1Logger := logger.NewLogger(fmt.Sprintf("node1/%s", node1.ID().ShortString()))
	node1Manager := p2p.NewManager(node1, p2p.WithLogger(node1Logger), reconnectOpt)
	go node1Manager.Start(shutdownSignal)
	node1AddrInfo := &peer.AddrInfo{ID: node1.ID(), Addrs: node1.Addrs()}

	node2 := newNode(ctx, t)
	node2Logger := logger.NewLogger(fmt.Sprintf("node2/%s", node2.ID().ShortString()))
	node2Manager := p2p.NewManager(node2, p2p.WithLogger(node2Logger), reconnectOpt)
	go node2Manager.Start(shutdownSignal)
	node2AddrInfo := &peer.AddrInfo{ID: node2.ID(), Addrs: node2.Addrs()}

	node1Service := gossip.NewService(protocolID, node1, node1Manager, gossip.WithLogger(node1Logger))
	node2Service := gossip.NewService(protocolID, node2, node2Manager, gossip.WithLogger(node2Logger))
	go node1Service.Start(shutdownSignal)
	go node2Service.Start(shutdownSignal)

	fmt.Println("node 1", node1.ID().ShortString())
	fmt.Println("node 2", node2.ID().ShortString())

	// connect node 1 and 2 to each other
	go node1Manager.ConnectPeer(node2AddrInfo, p2p.PeerRelationKnown)
	time.Sleep(100 * time.Millisecond)
	go node2Manager.ConnectPeer(node1AddrInfo, p2p.PeerRelationKnown)

	// should eventually both be connected to each other
	connectivity(t, node1Manager, node2.ID(), false, 10*time.Second)
	connectivity(t, node2Manager, node1.ID(), false, 10*time.Second)

	var protocolStartedCalled1, protocolStartedCalled2 bool
	node1Service.Events.ProtocolStarted.Attach(events.NewClosure(func(proto *gossip.Protocol) {
		protocolStartedCalled1 = true
	}))
	node2Service.Events.ProtocolStarted.Attach(events.NewClosure(func(proto *gossip.Protocol) {
		protocolStartedCalled2 = true
	}))

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
	node1Service.Events.ProtocolTerminated.Attach(events.NewClosure(func(proto *gossip.Protocol) {
		protocolTerminatedCalled1 = true
	}))
	node2Service.Events.ProtocolTerminated.Attach(events.NewClosure(func(proto *gossip.Protocol) {
		protocolTerminatedCalled2 = true
	}))

	// disconnecting them should also clean up the gossip protocol streams.
	// we also explicitly disconnect node 1 to remove the relation state
	go node1Manager.DisconnectPeer(node2.ID())
	go node2Manager.DisconnectPeer(node1.ID())
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

	// if we now connect node 1 to 2 with relation 'known'
	// but node 2 doesn't see node 1 as 'known', the protocol should
	// be started and terminated immediately

	protocolStartedCalled1 = false
	protocolTerminatedCalled1 = false
	node1Service.Events.ProtocolStarted.Attach(events.NewClosure(func(proto *gossip.Protocol) {
		protocolStartedCalled1 = true
	}))
	node1Service.Events.ProtocolTerminated.Attach(events.NewClosure(func(proto *gossip.Protocol) {
		protocolTerminatedCalled1 = true
	}))

	go node1Manager.ConnectPeer(node2AddrInfo, p2p.PeerRelationKnown)
	// should eventually both be connected to each other
	connectivity(t, node1Manager, node2.ID(), false, 10*time.Second)
	connectivity(t, node2Manager, node1.ID(), false, 10*time.Second)
	// but only node 2 is seen as protected on node 1
	require.True(t, node1.ConnManager().IsProtected(node2.ID(), p2p.KnownPeerConnectivityProtectionTag))
	require.False(t, node2.ConnManager().IsProtected(node1.ID(), p2p.KnownPeerConnectivityProtectionTag))

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

func TestCommunication(t *testing.T) {

}
