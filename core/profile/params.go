package profile

import (
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/app"
)

const (
	// CfgAppProfile is the key to set the profile to use.
	CfgAppProfile = "app.profile"

	// AutoProfileName is the name of the automatic profile.
	AutoProfileName = "auto"
)

var params = &app.ComponentParams{
	Params: func(fs *flag.FlagSet) {
		fs.StringP(CfgAppProfile, "p", AutoProfileName, "the profile the node runs with")
	},
	Masked: nil,
}
