package monitor

import (
	"github.com/gohornet/hornet/packages/parameter"
)

func init() {
	// "Path to tanglemonitor frontend files"
	parameter.NodeConfig.SetDefault("monitor.TangleMonitorPath", "tanglemonitor/frontend")

	// "Set the domain on which TangleMonitor is served"
	parameter.NodeConfig.SetDefault("monitor.domain", "")

	// "Set the host to which the TangleMonitor listens"
	parameter.NodeConfig.SetDefault("monitor.bindAddress", "127.0.0.1")

	// "TangleMonitor webserver port (do not change unless you redirect back to 4434)"
	parameter.NodeConfig.SetDefault("monitor.port", 4434)

	// "TangleMonitor API port (do not change unless you redirect back to 4433)"
	parameter.NodeConfig.SetDefault("monitor.apiPort", 4433)
}
