package profile

import (
	"github.com/gohornet/hornet/core/cli"
)

const (
	// CfgProfileUseProfile is the key to set the profile to use.
	CfgProfileUseProfile = "useProfile"

	// AutoProfileName is the name of the automatic profile.
	AutoProfileName = "auto"
)

func init() {
	cli.ConfigFlagSet.StringP(CfgProfileUseProfile, "p", AutoProfileName, "Sets the profile with which the node runs")
}
