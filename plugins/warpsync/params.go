package warpsync

import (
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/app"
)

const (
	// the used advancement range per warpsync checkpoint
	CfgWarpSyncAdvancementRange = "warpsync.advancementRange"
)

var params = &app.ComponentParams{
	Params: func(fs *flag.FlagSet) {
		fs.Int(CfgWarpSyncAdvancementRange, 150, "the used advancement range per warpsync checkpoint")
	},
	Masked: nil,
}
