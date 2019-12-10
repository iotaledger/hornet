package gossip

import (
	"github.com/gohornet/hornet/packages/logger"
	"github.com/gohornet/hornet/packages/node"
)

var PLUGIN = node.NewPlugin("Gossip", node.Enabled, configure, run)
var gossipLogger = logger.NewLogger("Gossip")

func configure(plugin *node.Plugin) {
	configureProtocol()
	configureNeighbors()
	configureReconnectPool()
	configureServer()
	configureBroadcastQueue()
	configurePacketProcessor()
	configureSTINGRequestsProcessor()
}

func run(plugin *node.Plugin) {
	runReconnectPool()
	runServer()
	runBroadcastQueue()
	runPacketProcessor()
	runSTINGRequestsProcessor()
}
