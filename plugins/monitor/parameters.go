package monitor

import (
	flag "github.com/spf13/pflag"
)

func init() {
	flag.String("monitor.TangleMonitorPath", "plugins/monitor/tanglemonitor/frontend", "Path to tanglemonitor frontend files")
}
