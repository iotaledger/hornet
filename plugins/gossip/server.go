package gossip

import (
	"net"
	"strconv"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/network"
	"github.com/iotaledger/hive.go/network/tcp"

	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/shutdown"
)

var TCPServer = tcp.NewServer()

func configureServer() {
	TCPServer.Events.Connect.Attach(events.NewClosure(func(conn *network.ManagedConnection) {
		if IsAddrBlacklisted(conn.RemoteAddr()) {
			conn.Close()
			return
		}
		gossipLogger.Infof("handling incoming connection from %s...", conn.RemoteAddr().String())

		protocol := newProtocol(conn)

		// create a new neighbor
		neighbor := NewInboundNeighbor(conn.Conn.RemoteAddr())
		neighbor.SetProtocol(protocol)

		setupNeighborEventHandlers(neighbor)

		go protocol.Init()
	}))

	TCPServer.Events.Error.Attach(events.NewClosure(func(err error) {
		gossipLogger.Fatal(err)
	}))
}

func runServer() {
	gossipBindAddr := config.NodeConfig.GetString(config.CfgNetGossipBindAddress)
	gossipLogger.Infof("Starting TCP Server (%s) ...", gossipBindAddr)

	daemon.BackgroundWorker("Gossip TCP Server", func(shutdownSignal <-chan struct{}) {
		gossipLogger.Infof("Starting TCP Server (%s) ... done", gossipBindAddr)

		addr, portStr, err := net.SplitHostPort(gossipBindAddr)
		if err != nil {
			gossipLogger.Fatalf("'%s' is an invalid bind address: %s", gossipBindAddr, err)
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			gossipLogger.Fatalf("'%s' contains an invalid port: %s", gossipBindAddr, err)
		}

		go TCPServer.Listen(addr, port)
		<-shutdownSignal
		gossipLogger.Info("Stopping TCP Server ...")
		TCPServer.Shutdown()

		gossipLogger.Info("Stopping TCP Server ... done")
	}, shutdown.ShutdownPriorityNeighborTCPServer)
}
