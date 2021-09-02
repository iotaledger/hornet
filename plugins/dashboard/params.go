package dashboard

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgNodeAlias set an alias to identify a node
	CfgNodeAlias = "node.alias"
	// the bind address on which the dashboard can be accessed from
	CfgDashboardBindAddress = "dashboard.bindAddress"
	// whether to run the dashboard in dev mode
	CfgDashboardDevMode = "dashboard.dev"
	// how long the auth session should last before expiring
	CfgDashboardAuthSessionTimeout = "dashboard.auth.sessionTimeout"
	// the auth username
	CfgDashboardAuthUsername = "dashboard.auth.username"
	// the auth password+salt as a scrypt hash
	CfgDashboardAuthPasswordHash = "dashboard.auth.passwordHash"
	// the auth salt used for hashing the password
	CfgDashboardAuthPasswordSalt = "dashboard.auth.passwordSalt"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgNodeAlias, "HORNET node", "set an alias to identify a node")
			fs.String(CfgDashboardBindAddress, "localhost:8081", "the bind address on which the dashboard can be accessed from")
			fs.Bool(CfgDashboardDevMode, false, "whether to run the dashboard in dev mode")
			fs.Duration(CfgDashboardAuthSessionTimeout, 72*time.Hour, "how long the auth session should last before expiring")
			fs.String(CfgDashboardAuthUsername, "admin", "the auth username")
			fs.String(CfgDashboardAuthPasswordHash, "0000000000000000000000000000000000000000000000000000000000000000", "the auth password+salt as a scrypt hash")
			fs.String(CfgDashboardAuthPasswordSalt, "0000000000000000000000000000000000000000000000000000000000000000", "the auth salt used for hashing the password")
			return fs
		}(),
	},
	Masked: []string{CfgDashboardAuthPasswordHash, CfgDashboardAuthPasswordSalt},
}
