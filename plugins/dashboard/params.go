package dashboard

import (
	"fmt"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/app"
)

const (
	// CfgAppAlias set an alias to identify a node
	CfgAppAlias = "app.alias"
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

	maxDashboardAuthUsernameSize = 25
)

var params = &app.ComponentParams{
	Params: func(fs *flag.FlagSet) {
		fs.String(CfgAppAlias, "HORNET node", "set an alias to identify a node")
		fs.String(CfgDashboardBindAddress, "localhost:8081", "the bind address on which the dashboard can be accessed from")
		fs.Bool(CfgDashboardDevMode, false, "whether to run the dashboard in dev mode")
		fs.Duration(CfgDashboardAuthSessionTimeout, 72*time.Hour, "how long the auth session should last before expiring")
		fs.String(CfgDashboardAuthUsername, "admin", fmt.Sprintf("the auth username (max %d chars)", maxDashboardAuthUsernameSize))
		fs.String(CfgDashboardAuthPasswordHash, "0000000000000000000000000000000000000000000000000000000000000000", "the auth password+salt as a scrypt hash")
		fs.String(CfgDashboardAuthPasswordSalt, "0000000000000000000000000000000000000000000000000000000000000000", "the auth salt used for hashing the password")
	},
	Masked: []string{CfgDashboardAuthPasswordHash, CfgDashboardAuthPasswordSalt},
}
