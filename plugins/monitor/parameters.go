package monitor

import (
	flag "github.com/spf13/pflag"
)

func init() {
	flag.String("monitor.TangleMonitorPath", "plugins/monitor/tanglemonitor/frontend", "Path to tanglemonitor frontend files")
	flag.String("monitor.domain", "", "Set the domain on which TangleMonitor is served")
	flag.String("monitor.host", "0.0.0.0", "Set the host to which the TangleMonitor listens")
	flag.Int("monitor.port", 4434, "Set the port to which the TangleMonitor listens")
	flag.Int("monitor.apiport", 4433, "Set the port of TangleMonitor REST API")
}
