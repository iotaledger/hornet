package gossip

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/network"
	"github.com/iotaledger/hive.go/network/tcp"

	"github.com/gohornet/hornet/packages/parameter"
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
	gossipLogger.Infof("Starting TCP Server (port %d) ...", parameter.NodeConfig.GetInt("network.port"))

	daemon.BackgroundWorker("Gossip TCP Server", func(shutdownSignal <-chan struct{}) {
		gossipLogger.Infof("Starting TCP Server (port %d) ... done", parameter.NodeConfig.GetInt("network.port"))

		go TCPServer.Listen(parameter.NodeConfig.GetString("network.bindAddress"), parameter.NodeConfig.GetInt("network.port"))
		<-shutdownSignal
		gossipLogger.Info("Stopping TCP Server ...")
		TCPServer.Shutdown()

		gossipLogger.Info("Stopping TCP Server ... done")
	}, shutdown.ShutdownPriorityNeighborTCPServer)
}
