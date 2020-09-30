package config

const (
	// whether to delete old message data from the database
	CfgPruningEnabled = "pruning.enabled"
	// amount of milestone cones to keep in the database
	CfgPruningDelay = "pruning.delay"
)

func init() {
	configFlagSet.Bool(CfgPruningEnabled, true, "whether to delete old message data from the database")
	configFlagSet.Int(CfgPruningDelay, 60480, "amount of milestone cones to keep in the database")
}
