package pruning

import (
	"context"

	"github.com/labstack/gommon/bytes"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/pruning"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	CoreComponent = &app.CoreComponent{
		Component: &app.Component{
			Name:           "Pruning",
			DepsFunc:       func(cDeps dependencies) { deps = cDeps },
			Params:         params,
			InitConfigPars: initConfigPars,
			Provide:        provide,
			Run:            run,
		},
	}
}

var (
	CoreComponent *app.CoreComponent
	deps          dependencies
)

type dependencies struct {
	dig.In
	SnapshotManager *snapshot.Manager
	PruningManager  *pruning.Manager
}

func initConfigPars(c *dig.Container) error {

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
			CoreComponent.LogPanicf("%s has to be specified if %s is enabled", CoreComponent.App().Config().GetParameterPath(&(ParamsPruning.Milestones.MaxMilestonesToKeep)), CoreComponent.App().Config().GetParameterPath(&(ParamsPruning.Milestones.Enabled)))
		}

		pruningSizeEnabled := ParamsPruning.Size.Enabled
		pruningTargetDatabaseSizeBytes, err := bytes.Parse(ParamsPruning.Size.TargetSize)
		if err != nil {
			CoreComponent.LogPanicf("parameter %s invalid", CoreComponent.App().Config().GetParameterPath(&(ParamsPruning.Size.TargetSize)))
		}

		if pruningSizeEnabled && pruningTargetDatabaseSizeBytes == 0 {
			CoreComponent.LogPanicf("%s has to be specified if %s is enabled", CoreComponent.App().Config().GetParameterPath(&(ParamsPruning.Size.TargetSize)), CoreComponent.App().Config().GetParameterPath(&(ParamsPruning.Size.Enabled)))
		}

		return pruning.NewPruningManager(
			CoreComponent.Logger(),
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

	onSnapshotHandledConfirmedMilestoneIndexChanged := events.NewClosure(func(confirmedMilestoneIndex iotago.MilestoneIndex) {
		deps.PruningManager.HandleNewConfirmedMilestoneEvent(CoreComponent.Daemon().ContextStopped(), confirmedMilestoneIndex)
	})

	if err := CoreComponent.Daemon().BackgroundWorker("Pruning", func(ctx context.Context) {
		CoreComponent.LogInfo("Starting pruning background worker ... done")
		deps.SnapshotManager.Events.HandledConfirmedMilestoneIndexChanged.Hook(onSnapshotHandledConfirmedMilestoneIndexChanged)

		<-ctx.Done()

		CoreComponent.LogInfo("Stopping pruning background worker ...")
		deps.SnapshotManager.Events.HandledConfirmedMilestoneIndexChanged.Detach(onSnapshotHandledConfirmedMilestoneIndexChanged)
		CoreComponent.LogInfo("Stopping pruning background worker ... done")
	}, daemon.PriorityPruning); err != nil {
		CoreComponent.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}
