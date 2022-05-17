package dashboard

import (
	"time"

	"github.com/iotaledger/hive.go/app"
)

const (
	maxDashboardAuthUsernameSize = 25
)

// ParametersNode contains the definition of the parameters used by WarpSync.
type ParametersNode struct {
	// CfgAppAlias set an alias to identify a node
	Alias string `default:"HORNET node" usage:"set an alias to identify a node"`
}

// ParametersDashboard contains the definition of the parameters used by WarpSync.
type ParametersDashboard struct {
	// the bind address on which the dashboard can be accessed from
	BindAddress string `default:"localhost:8081" usage:"the bind address on which the dashboard can be accessed from"`
	// whether to run the dashboard in dev mode
	DevMode bool `name:"dev" default:"false" usage:"whether to run the dashboard in dev mode"`

	Auth struct {
		// how long the auth session should last before expiring
		SessionTimeout time.Duration `default:"72h" usage:"how long the auth session should last before expiring"`
		// the auth username
		Username string `default:"admin" usage:"the auth username (max 25 chars)"`
		// the auth password+salt as a scrypt hash
		PasswordHash string `default:"0000000000000000000000000000000000000000000000000000000000000000" usage:"the auth password+salt as a scrypt hash"`
		// the auth salt used for hashing the password
		PasswordSalt string `default:"0000000000000000000000000000000000000000000000000000000000000000" usage:"the auth salt used for hashing the password"`
	}
}

var ParamsNode = &ParametersNode{}
var ParamsDashboard = &ParametersDashboard{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"node":      ParamsNode,
		"dashboard": ParamsDashboard,
	},
	Masked: []string{"dashboard.auth.passwordHash", "dashboard.auth.passwordSalt"},
}
