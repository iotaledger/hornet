package config

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
	NodeConfig.SetDefault(CfgGraphWebRootPath, "IOTAtangle/webroot")
	NodeConfig.SetDefault(CfgGraphWebSocketURI, "")
	NodeConfig.SetDefault(CfgGraphDomain, "")
	NodeConfig.SetDefault(CfgGraphBindAddress, "localhost:8083")
	NodeConfig.SetDefault(CfgGraphNetworkName, "meets HORNET")
	NodeConfig.SetDefault(CfgGraphExplorerTxLink, "http://localhost:8081/explorer/tx/")
	NodeConfig.SetDefault(CfgGraphExplorerBundleLink, "http://localhost:8081/explorer/bundle/")
}
