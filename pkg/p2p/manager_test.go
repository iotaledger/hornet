//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package p2p_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hornet/v2/pkg/p2p"
)

func newNode(t require.TestingT) host.Host {
	// we use Ed25519 because otherwise it takes longer as the default is RSA
	sk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	require.NoError(t, err)

	connManager, err := connmgr.NewConnManager(
		1,
		100,
		connmgr.WithGracePeriod(0),
	)
	require.NoError(t, err)

	h, err := libp2p.New(
		libp2p.Identity(sk),
		libp2p.ConnectionManager(connManager),
	)
	require.NoError(t, err)

	return h
}

func TestManager(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reconnectOpt := p2p.WithManagerReconnectInterval(1*time.Second, 500*time.Millisecond)

	cfg := configuration.New()
	err := cfg.Set("logger.disableStacktrace", true)
	require.NoError(t, err)

	// no need to check the error, since the global logger could already be initialized
	_ = logger.InitGlobalLogger(cfg)

	node1 := newNode(t)
	node1Logger := logger.NewLogger(fmt.Sprintf("node1/%s", node1.ID().ShortString()))
	node1Manager := p2p.NewManager(node1, p2p.WithManagerLogger(node1Logger), reconnectOpt)
	go node1Manager.Start(ctx)
	node1AddrInfo := &peer.AddrInfo{ID: node1.ID(), Addrs: node1.Addrs()[:1]}

	node2 := newNode(t)
	node2Logger := logger.NewLogger(fmt.Sprintf("node2/%s", node2.ID().ShortString()))
	node2Manager := p2p.NewManager(node2, p2p.WithManagerLogger(node2Logger), reconnectOpt)
	go node2Manager.Start(ctx)
	node2AddrInfo := &peer.AddrInfo{ID: node2.ID(), Addrs: node2.Addrs()[:1]}

	node3 := newNode(t)
	node3Logger := logger.NewLogger(fmt.Sprintf("node3/%s", node3.ID().ShortString()))
	node3Manager := p2p.NewManager(node3, p2p.WithManagerLogger(node3Logger), reconnectOpt)
	go node3Manager.Start(ctx)
	node3AddrInfo := &peer.AddrInfo{ID: node3.ID(), Addrs: node3.Addrs()[:1]}

	node4 := newNode(t)
	node4Logger := logger.NewLogger(fmt.Sprintf("node4/%s", node4.ID().ShortString()))
	node4Manager := p2p.NewManager(node4, p2p.WithManagerLogger(node4Logger), reconnectOpt)
	go node4Manager.Start(ctx)
	node4AddrInfo := &peer.AddrInfo{ID: node4.ID(), Addrs: node4.Addrs()[:1]}

	//fmt.Println("node 1", node1.ID())
	//fmt.Println("node 2", node2.ID())
	//fmt.Println("node 3", node3.ID())
	//fmt.Println("node 4", node4.ID())

	// can't connect to itself
	require.True(t, errors.Is(node1Manager.ConnectPeer(node1AddrInfo, p2p.PeerRelationKnown), p2p.ErrCantConnectToItself))

	// connect to each other
	node2AliasOnNode1 := "Node 2"
	go func() {
		_ = node1Manager.ConnectPeer(node2AddrInfo, p2p.PeerRelationKnown, node2AliasOnNode1)
	}()
	go func() {
		_ = node2Manager.ConnectPeer(node1AddrInfo, p2p.PeerRelationKnown)
	}()
	go func() {
		_ = node2Manager.ConnectPeer(node3AddrInfo, p2p.PeerRelationUnknown)
	}()

	// note we don't explicitly let node 3 connect to node 2

	// should eventually both be connected to each other
	connectivity(t, node1Manager, node2.ID(), false)
	connectivity(t, node2Manager, node1.ID(), false)
	// and node 2 and 3
	connectivity(t, node2Manager, node3.ID(), false)
	connectivity(t, node3Manager, node2.ID(), false)

	// connectivity should be protected from getting trimmed
	require.True(t, node1.ConnManager().IsProtected(node2.ID(), p2p.PeerConnectivityProtectionTag))
	require.True(t, node2.ConnManager().IsProtected(node1.ID(), p2p.PeerConnectivityProtectionTag))
	// but not for node 2<->3
	require.False(t, node2.ConnManager().IsProtected(node3.ID(), p2p.PeerConnectivityProtectionTag))
	require.False(t, node3.ConnManager().IsProtected(node2.ID(), p2p.PeerConnectivityProtectionTag))

	// check alias
	node1Manager.Call(node2.ID(), func(peer *p2p.Peer) {
		require.Equal(t, node2AliasOnNode1, peer.Alias)
	})

	// disconnect node 1 from 2
	require.NoError(t, node1Manager.DisconnectPeer(node2.ID()))
	connectivity(t, node1Manager, node2.ID(), true)
	connectivity(t, node2Manager, node1.ID(), true)

	// we instructed node 1 explicitly to disconnect from node 2, therefore
	// node 2 is no longer "protected" from trimming on node 1 onwards
	require.False(t, node1.ConnManager().IsProtected(node2.ID(), p2p.PeerConnectivityProtectionTag))
	// however, for node 2, node 1 is simply disconnected, so it is still considered protected
	require.True(t, node2.ConnManager().IsProtected(node1.ID(), p2p.PeerConnectivityProtectionTag))

	// so eventually, node 2 does a reconnect to node 1
	connectivity(t, node1Manager, node2.ID(), false, 10*time.Second)
	connectivity(t, node2Manager, node1.ID(), false, 10*time.Second)

	// and just for sanity's sake, the protected state should still be the same between the nodes as before:
	require.False(t, node1.ConnManager().IsProtected(node2.ID(), p2p.PeerConnectivityProtectionTag))
	require.True(t, node2.ConnManager().IsProtected(node1.ID(), p2p.PeerConnectivityProtectionTag))

	// if we then tell node 1 to connect to node 2 again explicitly (even if they're already connected),
	// but with a different relation than what node 2 currently is for node 1, it will be updated:
	err = node1Manager.ConnectPeer(node2AddrInfo, p2p.PeerRelationKnown)
	require.ErrorIs(t, err, p2p.ErrPeerInManagerAlready)
	require.True(t, node1.ConnManager().IsProtected(node2.ID(), p2p.PeerConnectivityProtectionTag))

	// connect node 4 to node 2 too
	go func() {
		_ = node2Manager.ConnectPeer(node4AddrInfo, p2p.PeerRelationUnknown)
	}()
	connectivity(t, node2Manager, node4.ID(), false)
	connectivity(t, node4Manager, node2.ID(), false)
	require.False(t, node2.ConnManager().IsProtected(node4.ID(), p2p.PeerConnectivityProtectionTag))

	// lets check that we do have as many connections as we think we should actually have
	connections(t, node1, peer.IDSlice{node2AddrInfo.ID})                                     // to node 2
	connections(t, node2, peer.IDSlice{node1AddrInfo.ID, node3AddrInfo.ID, node4AddrInfo.ID}) // to node 1,3,4
	connections(t, node3, peer.IDSlice{node2AddrInfo.ID})                                     // to node 2
	connections(t, node4, peer.IDSlice{node2AddrInfo.ID})                                     // to node 2

	// if we now trim connections on node 2 which currently is connected to node 1, 3 and 4,
	// then the connection to node 3 or 4 should be closed, as they aren't protected and our "low watermark" is 1.
	node2.ConnManager().TrimOpenConns(context.Background())
	require.Eventually(t, func() bool {
		return (!node2Manager.IsConnected(node3.ID()) && node2Manager.IsConnected(node4.ID())) ||
			(node2Manager.IsConnected(node3.ID()) && !node2Manager.IsConnected(node4.ID()))
	}, 10*time.Second, 100*time.Millisecond)

	// but the connection to node 1 from node 2 is still in tact
	connectivity(t, node1Manager, node2.ID(), false)
	connectivity(t, node2Manager, node1.ID(), false)

	node2Manager.ForEach(func(peer *p2p.Peer) bool {
		require.Equal(t, node1.ID(), peer.ID)

		return true
	}, p2p.PeerRelationKnown)
	node2Manager.ForEach(func(peer *p2p.Peer) bool {
		require.True(t, peer.ID == node3.ID() || peer.ID == node4.ID())

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

func connections(t *testing.T, node host.Host, targets peer.IDSlice) {
	targetsMap := make(map[peer.ID]struct{})
	unconnected := make(map[peer.ID]struct{})

	for _, target := range targets {
		targetsMap[target] = struct{}{}
		unconnected[target] = struct{}{}
	}

	for _, conn := range node.Network().Conns() {
		_, exists := targetsMap[conn.RemotePeer()]
		require.True(t, exists)

		if exists {
			delete(unconnected, conn.RemotePeer())
		}
	}

	require.Empty(t, unconnected)
}

func TestManagerEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := configuration.New()
	err := cfg.Set("logger.disableStacktrace", true)
	require.NoError(t, err)

	// no need to check the error, since the global logger could already be initialized
	_ = logger.InitGlobalLogger(cfg)

	reconnectOpt := p2p.WithManagerReconnectInterval(1*time.Second, 500*time.Millisecond)

	node1 := newNode(t)
	node1Logger := logger.NewLogger(fmt.Sprintf("node1/%s", node1.ID().ShortString()))
	node1Manager := p2p.NewManager(node1, p2p.WithManagerLogger(node1Logger), reconnectOpt)
	go node1Manager.Start(ctx)
	node1AddrInfo := &peer.AddrInfo{ID: node1.ID(), Addrs: node1.Addrs()}
	_ = node1AddrInfo

	node2 := newNode(t)
	node2Logger := logger.NewLogger(fmt.Sprintf("node2/%s", node2.ID().ShortString()))
	node2Manager := p2p.NewManager(node2, p2p.WithManagerLogger(node2Logger), reconnectOpt)
	go node2Manager.Start(ctx)
	node2AddrInfo := &peer.AddrInfo{ID: node2.ID(), Addrs: node2.Addrs()}

	var connectCalled, connectedCalled bool
	node1Manager.Events.Connect.Hook(events.NewClosure(func(_ *p2p.Peer) {
		connectCalled = true
	}))
	node1Manager.Events.Connected.Hook(events.NewClosure(func(_ *p2p.Peer, _ network.Conn) {
		connectedCalled = true
	}))

	go func() {
		_ = node1Manager.ConnectPeer(node2AddrInfo, p2p.PeerRelationUnknown)
	}()
	require.Eventually(t, func() bool {
		return connectCalled
	}, 4*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return connectedCalled
	}, 4*time.Second, 10*time.Millisecond)

	var disconnectCalled, disconnectedCalled bool
	node1Manager.Events.Disconnect.Hook(events.NewClosure(func(_ *p2p.Peer) {
		disconnectCalled = true
	}))
	node1Manager.Events.Disconnected.Hook(events.NewClosure(func(_ *p2p.PeerOptError) {
		disconnectedCalled = true
	}))

	go func() {
		_ = node1Manager.DisconnectPeer(node2.ID())
	}()
	require.Eventually(t, func() bool {
		return disconnectCalled
	}, 4*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return disconnectedCalled
	}, 4*time.Second, 10*time.Millisecond)

	go func() {
		_ = node1Manager.ConnectPeer(node2AddrInfo, p2p.PeerRelationUnknown)
	}()
	connectivity(t, node1Manager, node2.ID(), false)
	connectivity(t, node2Manager, node1.ID(), false)

	var relationUpdatedCalled bool
	var updatedRelation, oldRelation p2p.PeerRelation
	node1Manager.Events.RelationUpdated.Hook(events.NewClosure(func(peer *p2p.Peer, oldRel p2p.PeerRelation) {
		relationUpdatedCalled = true
		updatedRelation = peer.Relation
		oldRelation = oldRel
	}))

	_ = node1Manager.ConnectPeer(node2AddrInfo, p2p.PeerRelationKnown)
	require.True(t, relationUpdatedCalled)
	require.Equal(t, p2p.PeerRelationKnown, updatedRelation)
	require.Equal(t, p2p.PeerRelationUnknown, oldRelation)

	var reconnectingCalled, reconnectedCalled bool
	node1Manager.Events.Reconnecting.Hook(events.NewClosure(func(_ *p2p.Peer) {
		reconnectingCalled = true
	}))
	node1Manager.Events.Reconnected.Hook(events.NewClosure(func(_ *p2p.Peer) {
		reconnectedCalled = true
	}))

	go func() {
		_ = node2Manager.DisconnectPeer(node1.ID())
	}()

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
	node1 := newNode(b)
	node1Manager := p2p.NewManager(node1)
	go node1Manager.Start(ctx)
	time.Sleep(1 * time.Second)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node1Manager.ForEach(func(peer *p2p.Peer) bool {
			return true
		})
	}
}
