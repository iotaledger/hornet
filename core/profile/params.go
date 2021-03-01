package profile

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgNodeProfile is the key to set the profile to use.
	CfgNodeProfile = "node.profile"

	// AutoProfileName is the name of the automatic profile.
	AutoProfileName = "auto"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.StringP(CfgNodeProfile, "p", AutoProfileName, "Sets the profile with which the node runs")
			return fs
		}(),
	},
	Masked: nil,
}
