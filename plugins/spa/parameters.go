package spa

import flag "github.com/spf13/pflag"

func init() {
	flag.String("dashboard.host", "0.0.0.0", "Set the host to which the Dashboard listens")
	flag.Int("dashboard.port", 8081, "Set the port to which the Dashboard listens")
	flag.Bool("dashboard.dev", false, "Activate the dashboard dev mode")
	flag.Bool("dashboard.basic_auth.enabled", true, "Whether to use HTTP Basic Auth")
	flag.String("dashboard.basic_auth.username", "hornet", "The HTTP Basic Auth username")
	flag.String("dashboard.basic_auth.password", "hornet", "The HTTP Basic Auth password")
}
