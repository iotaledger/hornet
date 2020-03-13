package graph

import (
	"github.com/gohornet/hornet/packages/config"
)

func init() {

	// the path to the visualizer web assets
	config.NodeConfig.SetDefault(config.CfgGraphWebRootPath, "IOTAtangle/webroot")

	// the websocket URI to use
	config.NodeConfig.SetDefault(config.CfgGraphWebSocketURI, "ws://127.0.0.1:8083/ws")

	// sets the domain name from which the visualizer is served from
	config.NodeConfig.SetDefault(config.CfgGraphDomain, "")

	// the bind address from which the visualizer can be accessed from
	config.NodeConfig.SetDefault(config.CfgGraphBindAddress, "localhost:8083")

	// the name of the network to be shown on the visualizer site
	config.NodeConfig.SetDefault(config.CfgGraphNetworkName, "meets HORNET")
}
