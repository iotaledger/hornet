package config

import (
	flag "github.com/spf13/pflag"
)

const (
	// the used advancement range per warpsync checkpoint
	CfgWarpSyncAdvancementRange = "warpsync.advancementRange"
)

func init() {
	flag.Int(CfgWarpSyncAdvancementRange, 200, "the used advancement range per warpsync checkpoint")
}
