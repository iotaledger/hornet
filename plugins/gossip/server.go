package gossip

import (
	daemon "github.com/iotaledger/hive.go/daemon/ordered"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/parameter"
	"github.com/gohornet/hornet/packages/network"
	"github.com/gohornet/hornet/packages/network/tcp"
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
}

func runServer() {
	gossipLogger.Infof("Starting TCP Server (port %d) ...", parameter.NodeConfig.GetInt("network.port"))

	daemon.BackgroundWorker("Gossip TCP Server", func(shutdownSignal <-chan struct{}) {
		gossipLogger.Infof("Starting TCP Server (port %d) ... done", parameter.NodeConfig.GetInt("network.port"))

		go TCPServer.Listen(parameter.NodeConfig.GetString("network.address"), parameter.NodeConfig.GetInt("network.port"))
		<-shutdownSignal
		TCPServer.Shutdown()

		gossipLogger.Info("Stopping TCP Server ... done")
	}, shutdown.ShutdownPriorityNeighborTCPServer)
}
