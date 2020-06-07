package config

import (
	flag "github.com/spf13/pflag"
)

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
	CfgMonitorInitialTransactions = "monitor.initialTransactions"
	// the bind address on which the monitor can be access from
	CfgMonitorWebBindAddress = "monitor.webBindAddress"
	// the bind address on which the API listens on
	CfgMonitorAPIBindAddress = "monitor.apiBindAddress"
)

func init() {
	flag.String(CfgMonitorTangleMonitorPath, "tanglemonitor/frontend", "path to the tanglemonitor web assets")
	flag.String(CfgMonitorDomain, "", "the domain from which the tanglemonitor is served from")
	flag.String(CfgMonitorWebSocketURI, "", "the websocket URI to use (optional)")
	flag.Int(CfgMonitorRemoteAPIPort, 4433, "the remote API port")
	flag.Int(CfgMonitorInitialTransactions, 15000, "the initial amount of tx to load")
	flag.String(CfgMonitorWebBindAddress, "localhost:4434", "the bind address on which the monitor can be access from")
	flag.String(CfgMonitorAPIBindAddress, "localhost:4433", "the bind address on which the API listens on")
}
