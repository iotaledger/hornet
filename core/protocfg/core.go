package protocfg

import (
	"encoding/json"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
	iotago "github.com/iotaledger/iota.go/v3"
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
		KeyManager              *keymanager.KeyManager
		MilestonePublicKeyCount int `name:"milestonePublicKeyCount"`
		ProtocolParameters      *iotago.ProtocolParameters
		BaseToken               *BaseToken
	}

	if err := c.Provide(func(deps cfgDeps) cfgResult {

		res := cfgResult{
			MilestonePublicKeyCount: deps.NodeConfig.Int(CfgProtocolMilestonePublicKeyCount),
			ProtocolParameters: &iotago.ProtocolParameters{
				Version:     byte(deps.NodeConfig.Int(CfgProtocolParametersVersion)),
				NetworkName: deps.NodeConfig.String(CfgProtocolParametersNetworkName),
				Bech32HRP:   iotago.NetworkPrefix(deps.NodeConfig.String(CfgProtocolParametersBech32HRP)),
				MinPoWScore: deps.NodeConfig.Float64(CfgProtocolParametersMinPoWScore),
				RentStructure: iotago.RentStructure{
					VByteCost:    uint64(deps.NodeConfig.Int64(CfgProtocolParametersRentStructureVByteCost)),
					VBFactorData: iotago.VByteCostFactor(deps.NodeConfig.Int64(CfgProtocolParametersRentStructureVByteFactorData)),
					VBFactorKey:  iotago.VByteCostFactor(deps.NodeConfig.Int64(CfgProtocolParametersRentStructureVByteFactorKey)),
				},
				TokenSupply: uint64(deps.NodeConfig.Int64(CfgProtocolParametersTokenSupply)),
			},
			BaseToken: &BaseToken{
				Name:            deps.NodeConfig.String(CfgProtocolBaseTokenName),
				TickerSymbol:    deps.NodeConfig.String(CfgProtocolBaseTokenTickerSymbol),
				Unit:            deps.NodeConfig.String(CfgProtocolBaseTokenUnit),
				Subunit:         deps.NodeConfig.String(CfgProtocolBaseTokenSubunit),
				Decimals:        uint32(deps.NodeConfig.Int(CfgProtocolBaseTokenDecimals)),
				UseMetricPrefix: deps.NodeConfig.Bool(CfgProtocolBaseTokenUseMetricPrefix),
			},
		}

		keyRanges := ConfigPublicKeyRanges{}
		if *cooPubKeyRangesFlag != "" {
			// load from special CLI flag
			if err := json.Unmarshal([]byte(*cooPubKeyRangesFlag), &keyRanges); err != nil {
				CorePlugin.LogPanic(err)
			}
		} else {
			if err := deps.NodeConfig.SetDefault(CfgProtocolPublicKeyRanges, ConfigPublicKeyRanges{
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
					StartIndex: 2108160,
					EndIndex:   3359999,
				}, {
					Key:        "edd9c639a719325e465346b84133bf94740b7d476dd87fc949c0e8df516f9954",
					StartIndex: 2888660,
					EndIndex:   3359999,
				}, {
					Key:        "47a5098c696e0fb53e6339edac574be4172cb4701a8210c2ae7469b536fd2c59",
					StartIndex: 3360000,
					EndIndex:   0,
				}, {
					Key:        "ae4e03072b4869e87dd4cd59315291a034493a8c202b43b257f9c07bc86a2f3e",
					StartIndex: 3360000,
					EndIndex:   0,
				},
			}); err != nil {
				CorePlugin.LogPanic(err)
			}

			// load from config
			if err := deps.NodeConfig.Unmarshal(CfgProtocolPublicKeyRanges, &keyRanges); err != nil {
				CorePlugin.LogPanic(err)
			}
		}

		keyManager, err := KeyManagerWithConfigPublicKeyRanges(keyRanges)
		if err != nil {
			CorePlugin.LogPanicf("can't load public key ranges: %s", err)
		}
		res.KeyManager = keyManager
		return res
	}); err != nil {
		CorePlugin.LogPanic(err)
	}
}

func KeyManagerWithConfigPublicKeyRanges(coordinatorPublicKeyRanges ConfigPublicKeyRanges) (*keymanager.KeyManager, error) {
	keyManager := keymanager.New()
	for _, keyRange := range coordinatorPublicKeyRanges {
		pubKey, err := utils.ParseEd25519PublicKeyFromString(keyRange.Key)
		if err != nil {
			return nil, err
		}

		keyManager.AddKeyRange(pubKey, keyRange.StartIndex, keyRange.EndIndex)
	}

	return keyManager, nil
}
