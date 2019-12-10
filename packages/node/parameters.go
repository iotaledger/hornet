package node

import flag "github.com/spf13/pflag"

func init() {
	flag.Int("node.LogLevel", LOG_LEVEL_INFO, "controls the log types that are shown")
	flag.StringSlice("node.DisablePlugins", nil, "a list of plugins that shall be disabled")
	flag.StringSlice("node.EnablePlugins", nil, "a list of plugins that shall be enabled")
}
