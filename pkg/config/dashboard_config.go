package config

const (
	// the bind address on which the dashboard can be access from
	CfgDashboardBindAddress = "dashboard.bindAddress"
	// whether to run the dashboard in dev mode
	CfgDashboardDevMode = "dashboard.dev"
	// the theme for the dashboard to use (default or dark)
	CfgDashboardTheme = "dashboard.theme"
	// whether to use HTTP basic auth
	CfgDashboardBasicAuthEnabled = "dashboard.basicAuth.enabled"
	// the HTTP basic auth username
	CfgDashboardBasicAuthUsername = "dashboard.basicAuth.username"
	// the HTTP basic auth password+salt as a sha256 hash
	CfgDashboardBasicAuthPasswordHash = "dashboard.basicauth.passwordhash" // config key must be lower cased (for hiding passwords in PrintConfig)
	// the HTTP basic auth salt used for hashing the password
	CfgDashboardBasicAuthPasswordSalt = "dashboard.basicauth.passwordsalt" // config key must be lower cased (for hiding passwords in PrintConfig)
)

func init() {
	configFlagSet.String(CfgDashboardBindAddress, "localhost:8081", "the bind address on which the dashboard can be access from")
	configFlagSet.Bool(CfgDashboardDevMode, false, "whether to run the dashboard in dev mode")
	configFlagSet.Bool(CfgDashboardBasicAuthEnabled, false, "whether to use HTTP basic auth")
	configFlagSet.String(CfgDashboardBasicAuthUsername, "", "the HTTP basic auth username")
	configFlagSet.String(CfgDashboardBasicAuthPasswordHash, "", "the HTTP basic auth username")
	configFlagSet.String(CfgDashboardBasicAuthPasswordSalt, "", "the HTTP basic auth password+salt as a sha256 hash")
	configFlagSet.String(CfgDashboardTheme, "default", "the theme for the dashboard to use (default or dark)")
}
