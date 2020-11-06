package profile

import (
	"github.com/gohornet/hornet/pkg/node"
	flag "github.com/spf13/pflag"
)

const (
	// CfgProfileUseProfile is the key to set the profile to use.
	CfgProfileUseProfile = "useProfile"

	// AutoProfileName is the name of the automatic profile.
	AutoProfileName = "auto"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.StringP(CfgProfileUseProfile, "p", AutoProfileName, "Sets the profile with which the node runs")
			return fs
		}(),
	},
	Masked: nil,
}
