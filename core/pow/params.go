package pow

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgPoWRefreshTipsInterval is the interval for refreshing tips during PoW for spammer messages and messages passed without parents via API.
	CfgPoWRefreshTipsInterval = "pow.refreshTipsInterval"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Duration(CfgPoWRefreshTipsInterval, 5*time.Second, "interval for refreshing tips during PoW for spammer messages and messages passed without parents via API")
			return fs
		}(),
	},
	Masked: nil,
}
