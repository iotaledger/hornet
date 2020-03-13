package monitor

import (
	"github.com/gohornet/hornet/packages/config"
)

func init() {
	// path to the tanglemonitor web assets
	config.NodeConfig.SetDefault(config.CfgMonitorTangleMonitorPath, "tanglemonitor/frontend")

	// the domain from which the tanglemonitor is served from
	config.NodeConfig.SetDefault(config.CfgMonitorDomain, "")

	// the bind address on which the monitor can be access from
	config.NodeConfig.SetDefault(config.CfgMonitorWebBindAddress, "localhost:4434")

	// the bind address on which the API listens on
	config.NodeConfig.SetDefault(config.CfgMonitorAPIBindAddress, "localhost:4433")
}
