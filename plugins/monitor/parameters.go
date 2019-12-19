package monitor

import (
	flag "github.com/spf13/pflag"
)

func init() {
	flag.String("monitor.TangleMonitorPath", "plugins/monitor/tanglemonitor/frontend", "Path to tanglemonitor frontend files")
	flag.String("monitor.domain", "", "Set the domain on which TangleMonitor is served")
	flag.String("monitor.host", "127.0.0.1", "Set the host to which the TangleMonitor listens")
	flag.Int("monitor.port", 4434, "TangleMonitor webserver port (do not change unless you redirect back to 4434)")
	flag.Int("monitor.apiPort", 4433, "TangleMonitor API port (do not change unless you redirect back to 4433)")
}
