package protocfg

import (
	"encoding/json"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/crypto"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	_ = flag.CommandLine.MarkHidden(CfgProtocolPublicKeyRangesJSON)

	CoreComponent = &app.CoreComponent{
		Component: &app.Component{
			Name:           "ProtoCfg",
			Params:         params,
			InitConfigPars: initConfigPars,
		},
	}
}

var (
	CoreComponent *app.CoreComponent

	cooPubKeyRangesFlag = flag.String(CfgProtocolPublicKeyRangesJSON, "", "overwrite public key ranges (JSON)")
)

func initConfigPars(c *dig.Container) error {

	type cfgDeps struct {
		dig.In
		AppConfig *configuration.Configuration `name:"appConfig"`
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
			MilestonePublicKeyCount: deps.AppConfig.Int(CfgProtocolMilestonePublicKeyCount),
			ProtocolParameters: &iotago.ProtocolParameters{
				Version:       byte(deps.AppConfig.Int(CfgProtocolParametersVersion)),
				NetworkName:   deps.AppConfig.String(CfgProtocolParametersNetworkName),
				Bech32HRP:     iotago.NetworkPrefix(deps.AppConfig.String(CfgProtocolParametersBech32HRP)),
				MinPoWScore:   deps.AppConfig.Float64(CfgProtocolParametersMinPoWScore),
				BelowMaxDepth: uint16(deps.AppConfig.Int(CfgProtocolParametersBelowMaxDepth)),
				RentStructure: iotago.RentStructure{
					VByteCost:    uint64(deps.AppConfig.Int64(CfgProtocolParametersRentStructureVByteCost)),
					VBFactorData: iotago.VByteCostFactor(deps.AppConfig.Int64(CfgProtocolParametersRentStructureVByteFactorData)),
					VBFactorKey:  iotago.VByteCostFactor(deps.AppConfig.Int64(CfgProtocolParametersRentStructureVByteFactorKey)),
				},
				TokenSupply: uint64(deps.AppConfig.Int64(CfgProtocolParametersTokenSupply)),
			},
			BaseToken: &BaseToken{
				Name:            deps.AppConfig.String(CfgProtocolBaseTokenName),
				TickerSymbol:    deps.AppConfig.String(CfgProtocolBaseTokenTickerSymbol),
				Unit:            deps.AppConfig.String(CfgProtocolBaseTokenUnit),
				Subunit:         deps.AppConfig.String(CfgProtocolBaseTokenSubunit),
				Decimals:        uint32(deps.AppConfig.Int(CfgProtocolBaseTokenDecimals)),
				UseMetricPrefix: deps.AppConfig.Bool(CfgProtocolBaseTokenUseMetricPrefix),
			},
		}

		keyRanges := ConfigPublicKeyRanges{}
		if *cooPubKeyRangesFlag != "" {
			// load from special CLI flag
			if err := json.Unmarshal([]byte(*cooPubKeyRangesFlag), &keyRanges); err != nil {
				CoreComponent.LogPanic(err)
			}
		} else {
			if err := deps.AppConfig.SetDefault(CfgProtocolPublicKeyRanges, ConfigPublicKeyRanges{
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
				CoreComponent.LogPanic(err)
			}

			// load from config
			if err := deps.AppConfig.Unmarshal(CfgProtocolPublicKeyRanges, &keyRanges); err != nil {
				CoreComponent.LogPanic(err)
			}
		}

		keyManager, err := KeyManagerWithConfigPublicKeyRanges(keyRanges)
		if err != nil {
			CoreComponent.LogPanicf("can't load public key ranges: %s", err)
		}
		res.KeyManager = keyManager
		return res
	}); err != nil {
		CoreComponent.LogPanic(err)
	}

	return nil
}

func KeyManagerWithConfigPublicKeyRanges(coordinatorPublicKeyRanges ConfigPublicKeyRanges) (*keymanager.KeyManager, error) {
	keyManager := keymanager.New()
	for _, keyRange := range coordinatorPublicKeyRanges {
		pubKey, err := crypto.ParseEd25519PublicKeyFromString(keyRange.Key)
		if err != nil {
			return nil, err
		}

		keyManager.AddKeyRange(pubKey, keyRange.StartIndex, keyRange.EndIndex)
	}

	return keyManager, nil
}
