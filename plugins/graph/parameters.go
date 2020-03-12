package graph

import (
	"github.com/gohornet/hornet/packages/parameter"
)

func init() {

	// "Path to IOTA Tangle Visualiser webroot files"
	parameter.NodeConfig.SetDefault("graph.webrootPath", "IOTAtangle/webroot")

	// "Set the websocket URI"
	parameter.NodeConfig.SetDefault("graph.websocket.uri", "ws://127.0.0.1:8083/ws")

	// "Set the domain on which IOTA Tangle Visualiser is served"
	parameter.NodeConfig.SetDefault("graph.domain", "")

	// "Set the host to which the IOTA Tangle Visualiser listens"
	parameter.NodeConfig.SetDefault("graph.bindAddress", "127.0.0.1")

	// "IOTA Tangle Visualiser webserver port"
	parameter.NodeConfig.SetDefault("graph.port", 8083)

	// "Name of the network shown in IOTA Tangle Visualiser"
	parameter.NodeConfig.SetDefault("graph.networkName", "meets HORNET")
}
