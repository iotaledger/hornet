package autopeering

import (
	"github.com/gohornet/hornet/packages/config"
)

func init() {
	// list of autopeering entry nodes to use
	config.NodeConfig.SetDefault(config.CfgNetAutopeeringEntryNodes, []string{
		"LehlDBPJ6kfcfLOK6kAU4nD7B/BdR7SJhai7yFCbCCM=@enter.hornet.zone:14626",
	})
}
