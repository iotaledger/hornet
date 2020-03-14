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
	CfgDashboardBasicAuthPasswordHash = "dashboard.basicauth.passwordhash" // must be lower cased
	// the HTTP basic auth salt used for hashing the password
	CfgDashboardBasicAuthPasswordSalt = "dashboard.basicauth.passwordsalt" // must be lower cased
)

func init() {
	NodeConfig.SetDefault(CfgDashboardBindAddress, "localhost:8081")
	NodeConfig.SetDefault(CfgDashboardDevMode, false)
	NodeConfig.SetDefault(CfgDashboardBasicAuthEnabled, false)
	NodeConfig.SetDefault(CfgDashboardBasicAuthUsername, "")
	NodeConfig.SetDefault(CfgDashboardBasicAuthPasswordHash, "")
	NodeConfig.SetDefault(CfgDashboardBasicAuthPasswordSalt, "")
	NodeConfig.SetDefault(CfgDashboardTheme, "default")
}
