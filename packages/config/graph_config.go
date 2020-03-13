package config

const (
	// the path to the visualizer web assets
	CfgGraphWebRootPath = "graph.webRootPath"
	// the websocket URI to use
	CfgGraphWebSocketURI = "graph.webSocket.uri"
	// sets the domain name from which the visualizer is served from
	CfgGraphDomain = "graph.domain"
	// the bind address from which the visualizer can be accessed from
	CfgGraphBindAddress = "graph.bindAddress"
	// the name of the network to be shown on the visualizer site
	CfgGraphNetworkName = "graph.networkName"
)

func init() {
	NodeConfig.SetDefault(CfgGraphWebRootPath, "IOTAtangle/webroot")
	NodeConfig.SetDefault(CfgGraphWebSocketURI, "ws://127.0.0.1:8083/ws")
	NodeConfig.SetDefault(CfgGraphDomain, "")
	NodeConfig.SetDefault(CfgGraphBindAddress, "localhost:8083")
	NodeConfig.SetDefault(CfgGraphNetworkName, "meets HORNET")
}
