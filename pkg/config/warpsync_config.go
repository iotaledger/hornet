package config

const (
	// the used advancement range per warpsync checkpoint
	CfgWarpSyncAdvancementRange = "warpsync.advancementRange"
)

func init() {
	configFlagSet.Int(CfgWarpSyncAdvancementRange, 50, "the used advancement range per warpsync checkpoint")
}
