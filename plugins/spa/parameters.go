package spa

import (
	"github.com/gohornet/hornet/packages/parameter"
)

func init() {
	// "Set the host to which the Dashboard listens"
	parameter.NodeConfig.SetDefault("dashboard.host", "127.0.0.1")

	// "Set the port to which the Dashboard listens"
	parameter.NodeConfig.SetDefault("dashboard.port", 8081)

	// "Activate the dashboard dev mode"
	parameter.NodeConfig.SetDefault("dashboard.dev", false)

	// "Whether to use HTTP Basic Auth"
	parameter.NodeConfig.SetDefault("dashboard.basic_auth.enabled", true)

	// "The HTTP Basic Auth username"
	parameter.NodeConfig.SetDefault("dashboard.basic_auth.username", "hornet")

	// "The HTTP Basic Auth password"
	parameter.NodeConfig.SetDefault("dashboard.basic_auth.password", "hornet")
}
