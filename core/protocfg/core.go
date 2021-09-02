package protocfg

import (
	"encoding/json"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/configuration"
	iotago "github.com/iotaledger/iota.go/v2"
)

func init() {
	_ = flag.CommandLine.MarkHidden(CfgProtocolPublicKeyRangesJSON)

	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
			Name:           "ProtoCfg",
			Params:         params,
			InitConfigPars: initConfigPars,
		},
	}
}

var (
	CorePlugin *node.CorePlugin

	cooPubKeyRangesFlag = flag.String(CfgProtocolPublicKeyRangesJSON, "", "overwrite public key ranges (JSON)")
)

func initConfigPars(c *dig.Container) {

	type cfgDeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	type cfgResult struct {
		dig.Out
		PublicKeyRanges         coordinator.PublicKeyRanges
		NetworkID               uint64               `name:"networkId"`
		NetworkIDName           string               `name:"networkIdName"`
		Bech32HRP               iotago.NetworkPrefix `name:"bech32HRP"`
		MinPoWScore             float64              `name:"minPoWScore"`
		MilestonePublicKeyCount int                  `name:"milestonePublicKeyCount"`
	}

	if err := c.Provide(func(deps cfgDeps) cfgResult {

		res := cfgResult{
			NetworkID:               iotago.NetworkIDFromString(deps.NodeConfig.String(CfgProtocolNetworkIDName)),
			NetworkIDName:           deps.NodeConfig.String(CfgProtocolNetworkIDName),
			Bech32HRP:               iotago.NetworkPrefix(deps.NodeConfig.String(CfgProtocolBech32HRP)),
			MinPoWScore:             deps.NodeConfig.Float64(CfgProtocolMinPoWScore),
			MilestonePublicKeyCount: deps.NodeConfig.Int(CfgProtocolMilestonePublicKeyCount),
		}

		if *cooPubKeyRangesFlag != "" {
			// load from special CLI flag
			if err := json.Unmarshal([]byte(*cooPubKeyRangesFlag), &res.PublicKeyRanges); err != nil {
				CorePlugin.Panic(err)
			}
			return res
		}

		if err := deps.NodeConfig.SetDefault(CfgProtocolPublicKeyRanges, coordinator.PublicKeyRanges{
			{
				Key:        "a9b46fe743df783dedd00c954612428b34241f5913cf249d75bed3aafd65e4cd",
				StartIndex: 0,
				EndIndex:   777600,
			}, {
				Key:        "365fb85e7568b9b32f7359d6cbafa9814472ad0ecbad32d77beaf5dd9e84c6ba",
				StartIndex: 0,
				EndIndex:   1555200,
			}, {
				Key:        "ba6d07d1a1aea969e7e435f9f7d1b736ea9e0fcb8de400bf855dba7f2a57e947",
				StartIndex: 552960,
				EndIndex:   2108160,
			}, {
				Key:        "760d88e112c0fd210cf16a3dce3443ecf7e18c456c2fb9646cabb2e13e367569",
				StartIndex: 1333460,
				EndIndex:   2888660,
			}, {
				Key:        "7bac2209b576ea2235539358c7df8ca4d2f2fc35a663c760449e65eba9f8a6e7",
				StartIndex: 2111060,
				EndIndex:   3666260,
			}, {
				Key:        "edd9c639a719325e465346b84133bf94740b7d476dd87fc949c0e8df516f9954",
				StartIndex: 2888660,
				EndIndex:   4443860,
			},
		}); err != nil {
			CorePlugin.Panic(err)
		}

		// load from config
		if err := deps.NodeConfig.Unmarshal(CfgProtocolPublicKeyRanges, &res.PublicKeyRanges); err != nil {
			CorePlugin.Panic(err)
		}

		return res
	}); err != nil {
		CorePlugin.Panic(err)
	}
}
