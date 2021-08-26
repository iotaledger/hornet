package faucet

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	// the amount of funds the requester receives.
	CfgFaucetAmount = "faucet.amount"
	// the amount of funds the requester receives if the target address has more funds than the faucet amount and less than maximum.
	CfgFaucetSmallAmount = "faucet.smallAmount"
	// the maximum allowed amount of funds on the target address.
	CfgFaucetMaxAddressBalance = "faucet.maxAddressBalance"
	// the maximum output count per faucet message.
	CfgFaucetMaxOutputCount = "faucet.maxOutputCount"
	// the faucet transaction indexation payload.
	CfgFaucetIndexationMessage = "faucet.indexationMessage"
	// the maximum duration for collecting faucet batches.
	CfgFaucetBatchTimeout = "faucet.batchTimeout"
	// the amount of workers used for calculating PoW when issuing faucet messages.
	CfgFaucetPoWWorkerCount = "faucet.powWorkerCount"
	// the bind address on which the faucet website can be accessed from
	CfgFaucetWebsiteBindAddress = "faucet.website.bindAddress"
	// whether to host the faucet website
	CfgFaucetWebsiteEnabled = "faucet.website.enabled"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Int64(CfgFaucetAmount, 10000000, "the amount of funds the requester receives")
			fs.Int64(CfgFaucetSmallAmount, 1000000, "the amount of funds the requester receives if the target address has more funds than the faucet amount and less than maximum")
			fs.Int64(CfgFaucetMaxAddressBalance, 20000000, "the maximum allowed amount of funds on the target address")
			fs.Int(CfgFaucetMaxOutputCount, iotago.MaxOutputsCount, "the maximum output count per faucet message")
			fs.String(CfgFaucetIndexationMessage, "HORNET FAUCET", "the faucet transaction indexation payload")
			fs.Duration(CfgFaucetBatchTimeout, 2*time.Second, "the maximum duration for collecting faucet batches")
			fs.Int(CfgFaucetPoWWorkerCount, 0, "the amount of workers used for calculating PoW when issuing faucet messages")
			fs.String(CfgFaucetWebsiteBindAddress, "localhost:8091", "the bind address on which the faucet website can be accessed from")
			fs.Bool(CfgFaucetWebsiteEnabled, false, "whether to host the faucet website")
			return fs
		}(),
	},
	Masked: nil,
}
