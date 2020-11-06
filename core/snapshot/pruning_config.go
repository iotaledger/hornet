package snapshot

import (
	"github.com/gohornet/hornet/core/cli"
)

const (
	// whether to delete old message data from the database
	CfgPruningEnabled = "pruning.enabled"
	// amount of milestone cones to keep in the database
	CfgPruningDelay = "pruning.delay"
)

func init() {
	cli.ConfigFlagSet.Bool(CfgPruningEnabled, true, "whether to delete old message data from the database")
	cli.ConfigFlagSet.Int(CfgPruningDelay, 60480, "amount of milestone cones to keep in the database")
}
