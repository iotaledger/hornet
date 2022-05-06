package inx

import (
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/app"
)

const (
	// CfgINXBindAddress the bind address on which the INX can be accessed from
	CfgINXBindAddress = "inx.bindAddress"
	// the amount of workers used for calculating PoW when issuing messages via INX
	CfgINXPoWWorkerCount = "inx.powWorkerCount"
)

var params = &app.ComponentParams{
	Params: func(fs *flag.FlagSet) {
		fs.String(CfgINXBindAddress, "localhost:9029", "the bind address on which the INX can be accessed from")
		fs.Int(CfgINXPoWWorkerCount, 0, "the amount of workers used for calculating PoW when issuing messages via INX. (use 0 to use the maximum possible)")
	},
}
