package protocfg

import (
	"encoding/json"
	"fmt"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/hive.go/core/app/pkg/shutdown"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/protocol"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
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
		return ParamsProtocol.TargetNetworkName
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

		// even if not used directly, this dependency is needed to make sure
		// the snapshot is already loaded before initializing the protocol manager
		SnapshotImporter *snapshot.Importer
	}

	if err := c.Provide(func(deps protocolManagerDeps) *protocol.Manager {
		ledgerIndex, err := deps.UTXOManager.ReadLedgerIndex()
		if err != nil {
			CoreComponent.LogPanicf("can't initialize sync manager: %s", err)
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

	onConfirmedMilestoneChanged := events.NewClosure(func(cachedMilestone *storage.CachedMilestone) {
		defer cachedMilestone.Release(true) // milestone -1

		deps.ProtocolManager.HandleConfirmedMilestone(cachedMilestone.Milestone().Milestone())
	})
	deps.Tangle.Events.ConfirmedMilestoneChanged.Hook(onConfirmedMilestoneChanged)

	onNextMilestoneUnsupported := events.NewClosure(func(unsupportedProtoParamsMsOption *iotago.ProtocolParamsMilestoneOpt) {
		unsupportedVersion := unsupportedProtoParamsMsOption.ProtocolVersion
		CoreComponent.LogWarnf("next milestone will run under unsupported protocol version %d!", unsupportedVersion)
	})
	deps.ProtocolManager.Events.NextMilestoneUnsupported.Hook(onNextMilestoneUnsupported)

	deps.ProtocolManager.Events.CriticalErrors.Hook(events.NewClosure(func(err error) {
		deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("protocol manager hit a critical error: %s", err), true)
	}))

	return nil
}
