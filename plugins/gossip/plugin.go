package gossip

import (
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

var (
	PLUGIN       = node.NewPlugin("Gossip", node.Enabled, configure, run)
	gossipLogger *logger.Logger
)

func configure(plugin *node.Plugin) {
	gossipLogger = logger.NewLogger(plugin.Name)

	configureProtocol()
	configureAutopeering()
	configureNeighbors()
	configureReconnectPool()
	configureServer()
	configureBroadcastQueue()
	configurePacketProcessor()
	configureSTINGRequestsProcessor()
	configureConfigObserver()
}

func run(plugin *node.Plugin) {
	runReconnectPool()
	runServer()
	runBroadcastQueue()
	runPacketProcessor()
	runSTINGRequestsProcessor()
	runConfigObserver()
}
