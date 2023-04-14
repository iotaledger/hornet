package pruning

import (
	"context"

	"github.com/labstack/gommon/bytes"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hornet/v2/pkg/components"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/pruning"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	Component = &app.Component{
		Name:             "Pruning",
		DepsFunc:         func(cDeps dependencies) { deps = cDeps },
		Params:           params,
		InitConfigParams: initConfigParams,
		IsEnabled:        components.IsAutopeeringEntryNodeDisabled, // do not enable in "autopeering entry node" mode
		Provide:          provide,
		Run:              run,
	}
}

var (
	Component *app.Component
	deps      dependencies
)

type dependencies struct {
	dig.In
	SnapshotManager *snapshot.Manager
	PruningManager  *pruning.Manager
}

func initConfigParams(c *dig.Container) error {

	type cfgResult struct {
		dig.Out
		PruningPruneReceipts bool `name:"pruneReceipts"`
	}

	return c.Provide(func() cfgResult {
		return cfgResult{
			PruningPruneReceipts: ParamsPruning.PruneReceipts,
		}
	})
}

func provide(c *dig.Container) error {

	type pruningManagerDeps struct {
		dig.In
		Storage              *storage.Storage
		SyncManager          *syncmanager.SyncManager
		TangleDatabase       *database.Database `name:"tangleDatabase"`
		UTXODatabase         *database.Database `name:"utxoDatabase"`
		SnapshotManager      *snapshot.Manager
		PruningPruneReceipts bool `name:"pruneReceipts"`
	}

	return c.Provide(func(deps pruningManagerDeps) *pruning.Manager {

		pruningMilestonesEnabled := ParamsPruning.Milestones.Enabled
		pruningMilestonesMaxMilestonesToKeep := syncmanager.MilestoneIndexDelta(ParamsPruning.Milestones.MaxMilestonesToKeep)

		if pruningMilestonesEnabled && pruningMilestonesMaxMilestonesToKeep == 0 {
			Component.LogPanicf("%s has to be specified if %s is enabled", Component.App().Config().GetParameterPath(&(ParamsPruning.Milestones.MaxMilestonesToKeep)), Component.App().Config().GetParameterPath(&(ParamsPruning.Milestones.Enabled)))
		}

		pruningSizeEnabled := ParamsPruning.Size.Enabled
		pruningTargetDatabaseSizeBytes, err := bytes.Parse(ParamsPruning.Size.TargetSize)
		if err != nil {
			Component.LogPanicf("parameter %s invalid", Component.App().Config().GetParameterPath(&(ParamsPruning.Size.TargetSize)))
		}

		if pruningSizeEnabled && pruningTargetDatabaseSizeBytes == 0 {
			Component.LogPanicf("%s has to be specified if %s is enabled", Component.App().Config().GetParameterPath(&(ParamsPruning.Size.TargetSize)), Component.App().Config().GetParameterPath(&(ParamsPruning.Size.Enabled)))
		}

		return pruning.NewPruningManager(
			Component.Logger(),
			deps.Storage,
			deps.SyncManager,
			deps.TangleDatabase,
			deps.UTXODatabase,
			deps.SnapshotManager.MinimumMilestoneIndex,
			pruningMilestonesEnabled,
			pruningMilestonesMaxMilestonesToKeep,
			pruningSizeEnabled,
			pruningTargetDatabaseSizeBytes,
			ParamsPruning.Size.ThresholdPercentage,
			ParamsPruning.Size.CooldownTime,
			deps.PruningPruneReceipts,
		)
	})
}

func run() error {
	if err := Component.Daemon().BackgroundWorker("Pruning", func(ctx context.Context) {
		Component.LogInfo("Starting pruning background worker ... done")
		unhookEvent := deps.SnapshotManager.Events.HandledConfirmedMilestoneIndexChanged.Hook(func(confirmedMilestoneIndex iotago.MilestoneIndex) {
			deps.PruningManager.HandleNewConfirmedMilestoneEvent(ctx, confirmedMilestoneIndex)
		}).Unhook
		defer unhookEvent()

		<-ctx.Done()

		Component.LogInfo("Stopping pruning background worker ...")
		Component.LogInfo("Stopping pruning background worker ... done")
	}, daemon.PriorityPruning); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}
