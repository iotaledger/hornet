package config

const (
	// CfgProfileUseProfile is the key to set the profile to use.
	CfgProfileUseProfile = "useProfile"

	// AutoProfileName is the name of the automatic profile.
	AutoProfileName = "auto"
)

func init() {
	configFlagSet.StringP(CfgProfileUseProfile, "p", AutoProfileName, "Sets the profile with which the node runs")
}
