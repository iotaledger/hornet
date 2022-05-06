package pow

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/app"
)

const (
	// CfgPoWRefreshTipsInterval is the interval for refreshing tips during PoW for spammer messages and messages passed without parents via API.
	CfgPoWRefreshTipsInterval = "pow.refreshTipsInterval"
)

var params = &app.ComponentParams{
	Params: func(fs *flag.FlagSet) {
		fs.Duration(CfgPoWRefreshTipsInterval, 5*time.Second, "interval for refreshing tips during PoW for spammer messages and messages passed without parents via API")
	},
	Masked: nil,
}
