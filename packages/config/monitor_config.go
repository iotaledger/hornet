package config

const (
	// path to the tanglemonitor web assets
	CfgMonitorTangleMonitorPath = "monitor.tangleMonitorPath"
	// the domain from which the tanglemonitor is served from
	CfgMonitorDomain = "monitor.domain"
	// the websocket URI to use (optional)
	CfgMonitorWebSocketURI = "monitor.webSocket.uri"
	// the remote API port
	CfgMonitorRemoteAPIPort = "monitor.remoteAPIPort"
	// the initial amount of tx to load
	CfgMonitorInitialTransactionCount = "monitor.initialTransactionCount"
	// the bind address on which the monitor can be access from
	CfgMonitorWebBindAddress = "monitor.webBindAddress"
	// the bind address on which the API listens on
	CfgMonitorAPIBindAddress = "monitor.apiBindAddress"
)

func init() {
	NodeConfig.SetDefault(CfgMonitorTangleMonitorPath, "tanglemonitor/frontend")
	NodeConfig.SetDefault(CfgMonitorDomain, "")
	NodeConfig.SetDefault(CfgMonitorWebSocketURI, "")
	NodeConfig.SetDefault(CfgMonitorRemoteAPIPort, 4433)
	NodeConfig.SetDefault(CfgMonitorInitialTransactionCount, 15000)
	NodeConfig.SetDefault(CfgMonitorWebBindAddress, "localhost:4434")
	NodeConfig.SetDefault(CfgMonitorAPIBindAddress, "localhost:4433")
}
