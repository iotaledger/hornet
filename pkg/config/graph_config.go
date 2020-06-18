package config

import (
	flag "github.com/spf13/pflag"
)

const (
	// the path to the visualizer web assets
	CfgGraphWebRootPath = "graph.webRootPath"
	// the websocket URI to use (optional)
	CfgGraphWebSocketURI = "graph.webSocket.uri"
	// sets the domain name from which the visualizer is served from
	CfgGraphDomain = "graph.domain"
	// the bind address from which the visualizer can be accessed from
	CfgGraphBindAddress = "graph.bindAddress"
	// the name of the network to be shown on the visualizer site
	CfgGraphNetworkName = "graph.networkName"
	// the explorer transaction link
	CfgGraphExplorerTxLink = "graph.explorerTxLink"
	// the explorer bundle link
	CfgGraphExplorerBundleLink = "graph.explorerBundleLink"
)

func init() {
	flag.String(CfgGraphWebRootPath, "IOTAtangle/webroot", "the path to the visualizer web assets")
	flag.String(CfgGraphWebSocketURI, "", "the websocket URI to use (optional)")
	flag.String(CfgGraphDomain, "", "sets the domain name from which the visualizer is served from")
	flag.String(CfgGraphBindAddress, "localhost:8083", "the bind address from which the visualizer can be accessed from")
	flag.String(CfgGraphNetworkName, "meets HORNET", "the name of the network to be shown on the visualizer site")
	flag.String(CfgGraphExplorerTxLink, "http://localhost:8081/explorer/tx/", "the explorer transaction link")
	flag.String(CfgGraphExplorerBundleLink, "http://localhost:8081/explorer/bundle/", "the explorer bundle link")
}
