package protocfg

import (
	"encoding/json"
	"fmt"
	"github.com/iotaledger/hive.go/app/core/shutdown"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/pkg/protocol"
	"github.com/iotaledger/hornet/pkg/tangle"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/crypto"
	"github.com/iotaledger/hornet/pkg/keymanager"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	_ = flag.CommandLine.MarkHidden(CfgProtocolPublicKeyRangesJSON)

	CoreComponent = &app.CoreComponent{
		Component: &app.Component{
			Name:           "ProtoCfg",
			DepsFunc:       func(cDeps dependencies) { deps = cDeps },
			Params:         params,
			InitConfigPars: initConfigPars,
			Configure:      configure,
		},
	}
}

var (
	CoreComponent       *app.CoreComponent
	deps                dependencies
	cooPubKeyRangesFlag = flag.String(CfgProtocolPublicKeyRangesJSON, "", "overwrite public key ranges (JSON)")
)

type dependencies struct {
	dig.In
	Tangle          *tangle.Tangle
	ProtocolManager *protocol.Manager
	SyncManager     *syncmanager.SyncManager
	ShutdownHandler *shutdown.ShutdownHandler
}

func initConfigPars(c *dig.Container) error {

	type cfgDeps struct {
		dig.In
		Storage *storage.Storage `optional:"true"`
	}

	type cfgResult struct {
		dig.Out
		KeyManager              *keymanager.KeyManager
		MilestonePublicKeyCount int `name:"milestonePublicKeyCount"`
		ProtocolManager         *protocol.Manager
		BaseToken               *BaseToken
	}

	if err := c.Provide(func(deps cfgDeps) cfgResult {

		protoParas := &iotago.ProtocolParameters{
			Version:       ParamsProtocol.Parameters.Version,
			NetworkName:   ParamsProtocol.Parameters.NetworkName,
			Bech32HRP:     iotago.NetworkPrefix(ParamsProtocol.Parameters.Bech32HRP),
			MinPoWScore:   ParamsProtocol.Parameters.MinPoWScore,
			BelowMaxDepth: ParamsProtocol.Parameters.BelowMaxDepth,
			RentStructure: iotago.RentStructure{
				VByteCost:    ParamsProtocol.Parameters.RentStructureVByteCost,
				VBFactorData: iotago.VByteCostFactor(ParamsProtocol.Parameters.RentStructureVByteFactorData),
				VBFactorKey:  iotago.VByteCostFactor(ParamsProtocol.Parameters.RentStructureVByteFactorKey),
			},
			TokenSupply: ParamsProtocol.Parameters.TokenSupply,
		}

		res := cfgResult{
			MilestonePublicKeyCount: ParamsProtocol.MilestonePublicKeyCount,
			ProtocolManager:         protocol.NewManager(deps.Storage, protoParas),
			BaseToken: &BaseToken{
				Name:            ParamsProtocol.BaseToken.Name,
				TickerSymbol:    ParamsProtocol.BaseToken.TickerSymbol,
				Unit:            ParamsProtocol.BaseToken.Unit,
				Subunit:         ParamsProtocol.BaseToken.Subunit,
				Decimals:        ParamsProtocol.BaseToken.Decimals,
				UseMetricPrefix: ParamsProtocol.BaseToken.UseMetricPrefix,
			},
		}

		// load from config
		keyRanges := ParamsProtocol.PublicKeyRanges
		if *cooPubKeyRangesFlag != "" {
			// load from special CLI flag
			if err := json.Unmarshal([]byte(*cooPubKeyRangesFlag), &keyRanges); err != nil {
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

		keyManager.AddKeyRange(pubKey, milestone.Index(keyRange.StartIndex), milestone.Index(keyRange.EndIndex))
	}

	return keyManager, nil
}

func configure() error {
	deps.ProtocolManager.LoadPending(deps.SyncManager)

	deps.Tangle.Events.ConfirmedMilestoneChanged.Attach(events.NewClosure(deps.ProtocolManager.HandleConfirmedMilestone))

	unsuppProtoParasClosure := events.NewClosure(func(unsupportedProtocolParas *iotago.ProtocolParamsMilestoneOpt) {
		unsupportedVersion := unsupportedProtocolParas.ProtocolVersion
		CoreComponent.LogWarnf("next milestone will run under unsupported protocol version %d!", unsupportedVersion)
	})
	deps.ProtocolManager.Events.NextMilestoneUnsupported.Attach(unsuppProtoParasClosure)
	deps.ProtocolManager.Events.CriticalErrors.Attach(events.NewClosure(func(err error) {
		deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("protocol manager hit a critical error: %s", err), true)
	}))

	return nil
}
