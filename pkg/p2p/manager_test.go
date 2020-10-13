package p2p_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func newNode(ctx context.Context, t require.TestingT) host.Host {
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

func TestManager(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	shutdownSignal := make(chan struct{})
	defer close(shutdownSignal)

	reconnectOpt := p2p.WithReconnectInterval(1*time.Second, 500*time.Millisecond)

	cfg := viper.GetViper()
	cfg.Set("logger.disableStacktrace", true)
	require.NoError(t, logger.InitGlobalLogger(cfg))

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

	node3 := newNode(ctx, t)
	node3Logger := logger.NewLogger(fmt.Sprintf("node3/%s", node3.ID().ShortString()))
	node3Manager := p2p.NewManager(node3, p2p.WithLogger(node3Logger), reconnectOpt)
	go node3Manager.Start(shutdownSignal)
	node3AddrInfo := &peer.AddrInfo{ID: node3.ID(), Addrs: node3.Addrs()}

	node4 := newNode(ctx, t)
	node4Logger := logger.NewLogger(fmt.Sprintf("node4/%s", node4.ID().ShortString()))
	node4Manager := p2p.NewManager(node4, p2p.WithLogger(node4Logger), reconnectOpt)
	go node4Manager.Start(shutdownSignal)
	node4AddrInfo := &peer.AddrInfo{ID: node4.ID(), Addrs: node4.Addrs()}

	//fmt.Println("node 1", node1.ID())
	//fmt.Println("node 2", node2.ID())
	//fmt.Println("node 3", node3.ID())
	//fmt.Println("node 4", node4.ID())

	// can't connect to itself
	require.True(t, errors.Is(node1Manager.ConnectPeer(node1AddrInfo, p2p.PeerRelationKnown), p2p.ErrCantConnectToItself))

	// connect to each other
	go node1Manager.ConnectPeer(node2AddrInfo, p2p.PeerRelationKnown)
	go node2Manager.ConnectPeer(node1AddrInfo, p2p.PeerRelationKnown)
	go node2Manager.ConnectPeer(node3AddrInfo, p2p.PeerRelationUnknown)

	// note we do not explicitly let node 3 connect to node 2

	// should eventually both be connected to each other
	connectivity(t, node1Manager, node2.ID(), false)
	connectivity(t, node2Manager, node1.ID(), false)
	// and node 2 and 3
	connectivity(t, node2Manager, node3.ID(), false)
	connectivity(t, node3Manager, node2.ID(), false)

	// connectivity should be protected from getting trimmed
	require.True(t, node1.ConnManager().IsProtected(node2.ID(), p2p.KnownPeerConnectivityProtectionTag))
	require.True(t, node2.ConnManager().IsProtected(node1.ID(), p2p.KnownPeerConnectivityProtectionTag))
	// but not for node 2<->3
	require.False(t, node2.ConnManager().IsProtected(node3.ID(), p2p.KnownPeerConnectivityProtectionTag))
	require.False(t, node3.ConnManager().IsProtected(node2.ID(), p2p.KnownPeerConnectivityProtectionTag))

	// disconnect node 1 from 2
	require.Nil(t, node1Manager.DisconnectPeer(node2.ID()))
	connectivity(t, node1Manager, node2.ID(), true)
	connectivity(t, node2Manager, node1.ID(), true)

	// we instructed node 1 explicitly to disconnect from node 2, therefore
	// node 2 is no longer "protected" from trimming on node 1 onwards
	require.False(t, node1.ConnManager().IsProtected(node2.ID(), p2p.KnownPeerConnectivityProtectionTag))
	// however, for node 2, node 1 is simply disconnected, so it is still considered protected
	require.True(t, node2.ConnManager().IsProtected(node1.ID(), p2p.KnownPeerConnectivityProtectionTag))

	// so eventually, node 2 does a reconnect to node 1
	connectivity(t, node1Manager, node2.ID(), false, 10*time.Second)
	connectivity(t, node2Manager, node1.ID(), false, 10*time.Second)

	// and just for sanity's sake, the protected state should still be the same between the nodes as before:
	require.False(t, node1.ConnManager().IsProtected(node2.ID(), p2p.KnownPeerConnectivityProtectionTag))
	require.True(t, node2.ConnManager().IsProtected(node1.ID(), p2p.KnownPeerConnectivityProtectionTag))

	// if we then tell node 1 to connect to node 2 again explicitly (even if they're already connected),
	// but with a different relation than what node 2 currently is for node 1, it will be updated:
	_ = node1Manager.ConnectPeer(node2AddrInfo, p2p.PeerRelationKnown)
	require.True(t, node1.ConnManager().IsProtected(node2.ID(), p2p.KnownPeerConnectivityProtectionTag))

	// connect node 4 to node 2 too
	go node2Manager.ConnectPeer(node4AddrInfo, p2p.PeerRelationUnknown)
	connectivity(t, node2Manager, node4.ID(), false)
	connectivity(t, node4Manager, node2.ID(), false)
	require.False(t, node2.ConnManager().IsProtected(node4.ID(), p2p.KnownPeerConnectivityProtectionTag))

	// lets check that we do have as many connections as we think we should actually have
	require.Len(t, node1.Network().Conns(), 1) // to node 2
	require.Len(t, node2.Network().Conns(), 3) // to node 1,3,4
	require.Len(t, node3.Network().Conns(), 1) // to node 2
	require.Len(t, node4.Network().Conns(), 1) // to node 2

	// if we now trim connections on node 2 which currently is connected to node 1, 3 and 4,
	// then the connection to node 3 or 4 should be closed, as they aren't protected and our "low watermark" is 1.
	node2.ConnManager().TrimOpenConns(context.Background())
	require.Len(t, node2.Network().Conns(), 2) // to node 1, 3||4
	require.Eventually(t, func() bool {
		return (!node2Manager.IsConnected(node3.ID()) && node2Manager.IsConnected(node4.ID())) ||
			(node2Manager.IsConnected(node3.ID()) && !node2Manager.IsConnected(node4.ID()))
	}, 10*time.Second, 100*time.Millisecond)

	// but the connection to node 1 from node 2 is still in tact
	connectivity(t, node1Manager, node2.ID(), false)
	connectivity(t, node2Manager, node1.ID(), false)

	node2Manager.ForEach(func(p *p2p.Peer) bool {
		require.Equal(t, node1.ID(), p.ID)
		return true
	}, p2p.PeerRelationKnown)
	node2Manager.ForEach(func(p *p2p.Peer) bool {
		require.True(t, p.ID == node3.ID() || p.ID == node4.ID())
		return true
	}, p2p.PeerRelationUnknown)
}

func connectivity(t *testing.T, source *p2p.Manager, target peer.ID, disconnected bool, overrideCheckDuration ...time.Duration) {
	dur := 6 * time.Second
	if len(overrideCheckDuration) > 0 {
		dur = overrideCheckDuration[0]
	}
	require.Eventually(t, func() bool {
		return source.IsConnected(target) == !disconnected
	}, dur, 100*time.Millisecond)
}

func TestManagerEvents(t *testing.T) {
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
	_ = node1AddrInfo

	node2 := newNode(ctx, t)
	node2Logger := logger.NewLogger(fmt.Sprintf("node2/%s", node2.ID().ShortString()))
	node2Manager := p2p.NewManager(node2, p2p.WithLogger(node2Logger), reconnectOpt)
	go node2Manager.Start(shutdownSignal)
	node2AddrInfo := &peer.AddrInfo{ID: node2.ID(), Addrs: node2.Addrs()}

	var connectCalled, connectedCalled bool
	node1Manager.Events.Connect.Attach(events.NewClosure(func(p *p2p.Peer) {
		connectCalled = true
	}))
	node1Manager.Events.Connected.Attach(events.NewClosure(func(p *p2p.Peer, _ network.Conn) {
		connectedCalled = true
	}))

	go node1Manager.ConnectPeer(node2AddrInfo, p2p.PeerRelationUnknown)
	require.Eventually(t, func() bool {
		return connectCalled
	}, 4*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return connectedCalled
	}, 4*time.Second, 10*time.Millisecond)

	var disconnectCalled, disconnectedCalled bool
	node1Manager.Events.Disconnect.Attach(events.NewClosure(func(p *p2p.Peer) {
		disconnectCalled = true
	}))
	node1Manager.Events.Disconnected.Attach(events.NewClosure(func(p *p2p.Peer) {
		disconnectedCalled = true
	}))

	go node1Manager.DisconnectPeer(node2.ID())
	require.Eventually(t, func() bool {
		return disconnectCalled
	}, 4*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return disconnectedCalled
	}, 4*time.Second, 10*time.Millisecond)

	go node1Manager.ConnectPeer(node2AddrInfo, p2p.PeerRelationUnknown)
	connectivity(t, node1Manager, node2.ID(), false)
	connectivity(t, node2Manager, node1.ID(), false)

	var relationUpdatedCalled bool
	var updatedRelation, oldRelation p2p.PeerRelation
	node1Manager.Events.RelationUpdated.Attach(events.NewClosure(func(p *p2p.Peer, oldRel p2p.PeerRelation) {
		relationUpdatedCalled = true
		updatedRelation = p.Relation
		oldRelation = oldRel
	}))

	node1Manager.ConnectPeer(node2AddrInfo, p2p.PeerRelationKnown)
	require.True(t, relationUpdatedCalled)
	require.Equal(t, p2p.PeerRelationKnown, updatedRelation)
	require.Equal(t, p2p.PeerRelationUnknown, oldRelation)

	var reconnectingCalled, reconnectedCalled bool
	node1Manager.Events.Reconnecting.Attach(events.NewClosure(func(p *p2p.Peer) {
		reconnectingCalled = true
	}))
	node1Manager.Events.Reconnected.Attach(events.NewClosure(func(p *p2p.Peer) {
		reconnectedCalled = true
	}))

	go node2Manager.DisconnectPeer(node1.ID())

	// node 1 should reconnect to node 2
	connectivity(t, node1Manager, node2.ID(), true, 10*time.Second)
	connectivity(t, node2Manager, node1.ID(), true, 10*time.Second)
	connectivity(t, node1Manager, node2.ID(), false, 10*time.Second)
	connectivity(t, node2Manager, node1.ID(), false, 10*time.Second)
	require.True(t, reconnectingCalled)
	require.True(t, reconnectedCalled)
}

func BenchmarkManager_ForEach(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	shutdownSignal := make(chan struct{})
	defer close(shutdownSignal)
	node1 := newNode(ctx, b)
	node1Manager := p2p.NewManager(node1)
	go node1Manager.Start(shutdownSignal)
	time.Sleep(1 * time.Second)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node1Manager.ForEach(func(p *p2p.Peer) bool {
			return true
		})
	}
}
