package warpsync

import (
	"github.com/gohornet/hornet/core/cli"
)

const (
	// the used advancement range per warpsync checkpoint
	CfgWarpSyncAdvancementRange = "warpsync.advancementRange"
)

func init() {
	cli.ConfigFlagSet.Int(CfgWarpSyncAdvancementRange, 50, "the used advancement range per warpsync checkpoint")
}
