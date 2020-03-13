package spa

import (
	"github.com/gohornet/hornet/packages/config"
)

func init() {
	// the bind address on which the dashboard can be access from
	config.NodeConfig.SetDefault(config.CfgDashboardBindAddress, "localhost:8081")

	// whether to run the dashboard in dev mode
	config.NodeConfig.SetDefault(config.CfgDashboardDevMode, false)

	// whether to use HTTP basic auth
	config.NodeConfig.SetDefault(config.CfgDashboardBasicAuthEnabled, true)

	// the HTTP basic auth username
	config.NodeConfig.SetDefault(config.CfgDashboardBasicAuthUsername, "hornet")

	// the HTTP basic auth password
	config.NodeConfig.SetDefault(config.CfgDashboardBasicAuthPassword, "hornet")

	// the theme for the dashboard to use (default or dark)
	config.NodeConfig.SetDefault(config.CfgDashboardTheme, "default")
}
