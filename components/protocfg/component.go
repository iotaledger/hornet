package protocfg

import (
	"encoding/json"
	"fmt"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/app/shutdown"
	"github.com/iotaledger/hornet/v2/pkg/components"
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

	Component = &app.Component{
		Name:             "ProtoCfg",
		DepsFunc:         func(cDeps dependencies) { deps = cDeps },
		Params:           params,
		InitConfigParams: initConfigParams,
		IsEnabled:        components.IsAutopeeringEntryNodeDisabled, // do not enable in "autopeering entry node" mode
		Provide:          provide,
		Configure:        configure,
	}
}

var (
	Component           *app.Component
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

func initConfigParams(c *dig.Container) error {
	if err := c.Provide(func() string {
		return ParamsProtocol.TargetNetworkName
	}, dig.Name("targetNetworkName")); err != nil {
		Component.LogPanic(err)
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
				Component.LogPanic(err)
			}
		}

		keyManager, err := KeyManagerWithConfigPublicKeyRanges(keyRanges)
		if err != nil {
			Component.LogPanicf("can't load public key ranges: %s", err)
		}
		res.KeyManager = keyManager

		return res
	}); err != nil {
		Component.LogPanic(err)
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
			Component.LogPanicf("can't initialize sync manager: %s", err)
		}

		protocolManager, err := protocol.NewManager(deps.Storage, ledgerIndex)
		if err != nil {
			Component.LogPanic(err)
		}

		return protocolManager
	}); err != nil {
		Component.LogPanic(err)
	}

	return nil
}

func configure() error {
	deps.Tangle.Events.ConfirmedMilestoneChanged.Hook(func(cachedMilestone *storage.CachedMilestone) {
		defer cachedMilestone.Release(true) // milestone -1

		deps.ProtocolManager.HandleConfirmedMilestone(cachedMilestone.Milestone().Milestone())
	})

	deps.ProtocolManager.Events.NextMilestoneUnsupported.Hook(func(unsupportedProtoParamsMsOption *iotago.ProtocolParamsMilestoneOpt) {
		unsupportedVersion := unsupportedProtoParamsMsOption.ProtocolVersion
		Component.LogWarnf("next milestone will run under unsupported protocol version %d!", unsupportedVersion)
	})

	deps.ProtocolManager.Events.CriticalErrors.Hook(func(err error) {
		deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("protocol manager hit a critical error: %s", err), true)
	})

	return nil
}
