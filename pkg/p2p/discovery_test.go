package p2p_test

import (
	"testing"
)

func TestDiscoveryService(t *testing.T) {
	/*
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		shutdownSignal := make(chan struct{})
		defer close(shutdownSignal)

		reconnectOpt := p2p.WithManagerReconnectInterval(2*time.Second, 500*time.Millisecond)

		cfg := configuration.New()
		cfg.Set("logger.disableStacktrace", true)
		require.NoError(t, logger.InitGlobalLogger(cfg))

		node1 := newNode(ctx, t)
		node1Logger := logger.NewLogger(fmt.Sprintf("node1/%s", node1.ID().ShortString()))
		node1Manager := p2p.NewManager(node1, p2p.WithManagerLogger(node1Logger), reconnectOpt)
		go node1Manager.Start(shutdownSignal)
		node1AddrInfo := &peer.AddrInfo{ID: node1.ID(), Addrs: node1.Addrs()}
		node1LoggerDiscSrv := logger.NewLogger(fmt.Sprintf("node1/DS/%s", node1.ID().ShortString()))
		node1DiscSrv := p2p.NewDiscoveryService(node1, node1Manager,
			p2p.WithDiscoveryServiceAdvertiseInterval(1*time.Second),
			p2p.WithDiscoveryServiceRoutingRefreshPeriod(20*time.Second),
			p2p.WithDiscoveryServiceLogger(node1LoggerDiscSrv),
		)
		go node1DiscSrv.Start(shutdownSignal)

		node2 := newNode(ctx, t)
		node2Logger := logger.NewLogger(fmt.Sprintf("node2/%s", node2.ID().ShortString()))
		node2Manager := p2p.NewManager(node2, p2p.WithManagerLogger(node2Logger), reconnectOpt)
		go node2Manager.Start(shutdownSignal)
		node2AddrInfo := &peer.AddrInfo{ID: node2.ID(), Addrs: node2.Addrs()}
		node2LoggerDiscSrv := logger.NewLogger(fmt.Sprintf("node2/DS/%s", node2.ID().ShortString()))
		node2DiscSrv := p2p.NewDiscoveryService(node2, node2Manager,
			p2p.WithDiscoveryServiceAdvertiseInterval(1*time.Second),
			p2p.WithDiscoveryServiceRoutingRefreshPeriod(30*time.Second),
			p2p.WithDiscoveryServiceLogger(node2LoggerDiscSrv),
		)
		go node2DiscSrv.Start(shutdownSignal)

		node3 := newNode(ctx, t)
		node3Logger := logger.NewLogger(fmt.Sprintf("node3/%s", node3.ID().ShortString()))
		node3Manager := p2p.NewManager(node3, p2p.WithManagerLogger(node3Logger), reconnectOpt)
		go node3Manager.Start(shutdownSignal)
		node3AddrInfo := &peer.AddrInfo{ID: node3.ID(), Addrs: node3.Addrs()}
		node3LoggerDiscSrv := logger.NewLogger(fmt.Sprintf("node3/DS/%s", node3.ID().ShortString()))
		node3DiscSrv := p2p.NewDiscoveryService(node3, node3Manager,
			p2p.WithDiscoveryServiceAdvertiseInterval(1*time.Second),
			p2p.WithDiscoveryServiceRoutingRefreshPeriod(30*time.Second),
			p2p.WithDiscoveryServiceLogger(node3LoggerDiscSrv),
		)
		go node3DiscSrv.Start(shutdownSignal)

		fmt.Println("node 1", node1.ID().ShortString())
		fmt.Println("node 2", node2.ID().ShortString())
		fmt.Println("node 3", node3.ID().ShortString())

		time.Sleep(2 * time.Second)

		// connect to each other
		go node1Manager.ConnectPeer(node2AddrInfo, p2p.PeerRelationKnown)
		go node2Manager.ConnectPeer(node1AddrInfo, p2p.PeerRelationKnown)
		go node2Manager.ConnectPeer(node3AddrInfo, p2p.PeerRelationKnown)
		go node3Manager.ConnectPeer(node2AddrInfo, p2p.PeerRelationKnown)

		connectivity(t, node1Manager, node2.ID(), false, 10*time.Second)
		connectivity(t, node2Manager, node1.ID(), false, 10*time.Second)
		connectivity(t, node2Manager, node3.ID(), false, 10*time.Second)
		connectivity(t, node3Manager, node2.ID(), false, 10*time.Second)

		time.Sleep(2 * time.Minute)
	*/
}
