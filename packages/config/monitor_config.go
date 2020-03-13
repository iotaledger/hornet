package config

const (
	// path to the tanglemonitor web assets
	CfgMonitorTangleMonitorPath = "monitor.tangleMonitorPath"
	// the domain from which the tanglemonitor is served from
	CfgMonitorDomain = "monitor.domain"
	// the bind address on which the monitor can be access from
	CfgMonitorWebBindAddress = "monitor.webBindAddress"
	// the bind address on which the API listens on
	CfgMonitorAPIBindAddress = "monitor.apiBindAddress"
)

func init() {
	NodeConfig.SetDefault(CfgMonitorTangleMonitorPath, "tanglemonitor/frontend")
	NodeConfig.SetDefault(CfgMonitorDomain, "")
	NodeConfig.SetDefault(CfgMonitorWebBindAddress, "localhost:4434")
	NodeConfig.SetDefault(CfgMonitorAPIBindAddress, "localhost:4433")
}
