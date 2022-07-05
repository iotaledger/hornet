package protocfg

import (
	"encoding/json"
	"fmt"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/app/core/shutdown"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	"github.com/iotaledger/hornet/pkg/protocol"
	"github.com/iotaledger/hornet/pkg/tangle"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/keymanager"
)

func init() {
	_ = flag.CommandLine.MarkHidden(CfgProtocolPublicKeyRangesJSON)

	CoreComponent = &app.CoreComponent{
		Component: &app.Component{
			Name:           "ProtoCfg",
			DepsFunc:       func(cDeps dependencies) { deps = cDeps },
			Params:         params,
			InitConfigPars: initConfigPars,
			Provide:        provide,
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

	if err := c.Provide(func() string {
		return ParamsProtocol.Parameters.NetworkName
	}, dig.Name("targetNetworkName")); err != nil {
		CoreComponent.LogPanic(err)
	}

	type cfgDeps struct {
		dig.In
		Storage *storage.Storage `optional:"true"` // optional because of entry-node mode
	}

	type cfgResult struct {
		dig.Out
		KeyManager              *keymanager.KeyManager
		MilestonePublicKeyCount int `name:"milestonePublicKeyCount"`
		BaseToken               *BaseToken
	}

	if err := c.Provide(func(deps cfgDeps) cfgResult {

		res := cfgResult{
			MilestonePublicKeyCount: ParamsProtocol.MilestonePublicKeyCount,
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

func provide(c *dig.Container) error {

	type protocolManagerDeps struct {
		dig.In
		Storage     *storage.Storage
		UTXOManager *utxo.Manager
	}

	if err := c.Provide(func(deps protocolManagerDeps) *protocol.Manager {

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

		protoParasBytes, err := protoParas.Serialize(serializer.DeSeriModeNoValidation, nil)
		if err != nil {
			CoreComponent.LogPanic(err)
		}

		// store the initial protocol parameters as milestone 0 for now
		if err := deps.Storage.StoreProtocolParameters(&iotago.ProtocolParamsMilestoneOpt{
			TargetMilestoneIndex: 0,
			ProtocolVersion:      protoParas.Version,
			Params:               protoParasBytes,
		}); err != nil {
			CoreComponent.LogPanic(err)
		}

		ledgerIndex, err := deps.UTXOManager.ReadLedgerIndex()
		if err != nil {
			CoreComponent.LogPanicf("can't initialize protocol manager: %s", err)
		}

		protocolManager, err := protocol.NewManager(deps.Storage, ledgerIndex)
		if err != nil {
			CoreComponent.LogPanic(err)
		}
		return protocolManager
	}); err != nil {
		CoreComponent.LogPanic(err)
	}

	return nil
}

func configure() error {
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
