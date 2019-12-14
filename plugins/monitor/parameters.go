package monitor

import (
	flag "github.com/spf13/pflag"
)

func init() {
	flag.String("monitor.TangleMonitorPath", "plugins/monitor/tanglemonitor/frontend", "Path to tanglemonitor frontend files")
	flag.String("monitor.domain", "", "Set the domain on which TangleMonitor is served")
	flag.String("monitor.host", "0.0.0.0", "Set the host to which the TangleMonitor listens")
}
