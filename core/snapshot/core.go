package snapshot

import (
	"context"
	"fmt"
	"github.com/iotaledger/hornet/pkg/protocol"
	"os"

	"github.com/labstack/gommon/bytes"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hornet/pkg/daemon"
	"github.com/iotaledger/hornet/pkg/database"
	"github.com/iotaledger/hornet/pkg/metrics"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	"github.com/iotaledger/hornet/pkg/snapshot"
	"github.com/iotaledger/hornet/pkg/tangle"
)

const (
	// SolidEntryPointCheckAdditionalThresholdPast is the additional past cone (to BMD) that is walked to calculate the solid entry points
	SolidEntryPointCheckAdditionalThresholdPast = 5

	// SolidEntryPointCheckAdditionalThresholdFuture is the additional future cone (to BMD) that is needed to calculate solid entry points correctly
	SolidEntryPointCheckAdditionalThresholdFuture = 5

	// AdditionalPruningThreshold is the additional threshold (to BMD), which is needed, because the blocks in the getMilestoneParents call in solidEntryPoints
	// can reference older blocks as well
	AdditionalPruningThreshold = 5
)

const (
	// CfgSnapshotsForceLoadingSnapshot defines the force loading of a snapshot, even if a database already exists
	CfgSnapshotsForceLoadingSnapshot = "forceLoadingSnapshot"
)

func init() {
	_ = flag.CommandLine.MarkHidden(CfgSnapshotsForceLoadingSnapshot)

	CoreComponent = &app.CoreComponent{
		Component: &app.Component{
			Name:           "Snapshot",
			DepsFunc:       func(cDeps dependencies) { deps = cDeps },
			Params:         params,
			InitConfigPars: initConfigPars,
			Provide:        provide,
			Configure:      configure,
			Run:            run,
		},
	}
}

var (
	CoreComponent *app.CoreComponent
	deps          dependencies

	forceLoadingSnapshot = flag.Bool(CfgSnapshotsForceLoadingSnapshot, false, "force loading of a snapshot, even if a database already exists")
)

type dependencies struct {
	dig.In
	Storage              *storage.Storage
	Tangle               *tangle.Tangle
	UTXOManager          *utxo.Manager
	SnapshotManager      *snapshot.SnapshotManager
	DeleteAllFlag        bool   `name:"deleteAll"`
	PruningPruneReceipts bool   `name:"pruneReceipts"`
	SnapshotsFullPath    string `name:"snapshotsFullPath"`
	SnapshotsDeltaPath   string `name:"snapshotsDeltaPath"`
	StorageMetrics       *metrics.StorageMetrics
}

func initConfigPars(c *dig.Container) error {

	type cfgResult struct {
		dig.Out
		PruningPruneReceipts bool   `name:"pruneReceipts"`
		SnapshotsFullPath    string `name:"snapshotsFullPath"`
		SnapshotsDeltaPath   string `name:"snapshotsDeltaPath"`
	}

	return c.Provide(func() cfgResult {
		return cfgResult{
			PruningPruneReceipts: ParamsPruning.PruneReceipts,
			SnapshotsFullPath:    ParamsSnapshots.FullPath,
			SnapshotsDeltaPath:   ParamsSnapshots.DeltaPath,
		}
	})
}

func provide(c *dig.Container) error {

	type snapshotDeps struct {
		dig.In
		TangleDatabase       *database.Database `name:"tangleDatabase"`
		UTXODatabase         *database.Database `name:"utxoDatabase"`
		Storage              *storage.Storage
		SyncManager          *syncmanager.SyncManager
		UTXOManager          *utxo.Manager
		ProtocolManager      *protocol.Manager
		PruningPruneReceipts bool   `name:"pruneReceipts"`
		SnapshotsFullPath    string `name:"snapshotsFullPath"`
		SnapshotsDeltaPath   string `name:"snapshotsDeltaPath"`
	}

	return c.Provide(func(deps snapshotDeps) *snapshot.SnapshotManager {

		solidEntryPointCheckThresholdPast := milestone.Index(deps.ProtocolManager.Current().BelowMaxDepth + SolidEntryPointCheckAdditionalThresholdPast)
		solidEntryPointCheckThresholdFuture := milestone.Index(deps.ProtocolManager.Current().BelowMaxDepth + SolidEntryPointCheckAdditionalThresholdFuture)
		pruningThreshold := milestone.Index(deps.ProtocolManager.Current().BelowMaxDepth + AdditionalPruningThreshold)

		snapshotDepth := milestone.Index(ParamsSnapshots.Depth)
		if snapshotDepth < solidEntryPointCheckThresholdFuture {
			CoreComponent.LogWarnf("parameter '%s' is too small (%d). value was changed to %d", CoreComponent.App.Config().GetParameterPath(&(ParamsSnapshots.Depth)), snapshotDepth, solidEntryPointCheckThresholdFuture)
			snapshotDepth = solidEntryPointCheckThresholdFuture
		}

		pruningMilestonesEnabled := ParamsPruning.Milestones.Enabled
		pruningMilestonesMaxMilestonesToKeep := milestone.Index(ParamsPruning.Milestones.MaxMilestonesToKeep)
		pruningMilestonesMaxMilestonesToKeepMin := snapshotDepth + solidEntryPointCheckThresholdPast + pruningThreshold + 1
		if pruningMilestonesMaxMilestonesToKeep != 0 && pruningMilestonesMaxMilestonesToKeep < pruningMilestonesMaxMilestonesToKeepMin {
			CoreComponent.LogWarnf("parameter '%s' is too small (%d). value was changed to %d", CoreComponent.App.Config().GetParameterPath(&(ParamsPruning.Milestones.MaxMilestonesToKeep)), pruningMilestonesMaxMilestonesToKeep, pruningMilestonesMaxMilestonesToKeepMin)
			pruningMilestonesMaxMilestonesToKeep = pruningMilestonesMaxMilestonesToKeepMin
		}

		if pruningMilestonesEnabled && pruningMilestonesMaxMilestonesToKeep == 0 {
			CoreComponent.LogPanicf("%s has to be specified if %s is enabled", CoreComponent.App.Config().GetParameterPath(&(ParamsPruning.Milestones.MaxMilestonesToKeep)), CoreComponent.App.Config().GetParameterPath(&(ParamsPruning.Milestones.Enabled)))
		}

		pruningSizeEnabled := ParamsPruning.Size.Enabled
		pruningTargetDatabaseSizeBytes, err := bytes.Parse(ParamsPruning.Size.TargetSize)
		if err != nil {
			CoreComponent.LogPanicf("parameter %s invalid", CoreComponent.App.Config().GetParameterPath(&(ParamsPruning.Size.TargetSize)))
		}

		if pruningSizeEnabled && pruningTargetDatabaseSizeBytes == 0 {
			CoreComponent.LogPanicf("%s has to be specified if %s is enabled", CoreComponent.App.Config().GetParameterPath(&(ParamsPruning.Size.TargetSize)), CoreComponent.App.Config().GetParameterPath(&(ParamsPruning.Size.Enabled)))
		}

		return snapshot.NewSnapshotManager(
			CoreComponent.Logger(),
			deps.TangleDatabase,
			deps.UTXODatabase,
			deps.Storage,
			deps.SyncManager,
			deps.UTXOManager,
			deps.ProtocolManager,
			deps.SnapshotsFullPath,
			deps.SnapshotsDeltaPath,
			ParamsSnapshots.DeltaSizeThresholdPercentage,
			ParamsSnapshots.DownloadURLs,
			solidEntryPointCheckThresholdPast,
			solidEntryPointCheckThresholdFuture,
			pruningThreshold,
			snapshotDepth,
			milestone.Index(ParamsSnapshots.Interval),
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

func configure() error {

	if deps.DeleteAllFlag {
		// delete old snapshot files
		if err := os.Remove(deps.SnapshotsFullPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("deleting full snapshot file failed: %w", err)
		}

		if err := os.Remove(deps.SnapshotsDeltaPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("deleting delta snapshot file failed: %w", err)
		}
	}

	snapshotInfo := deps.Storage.SnapshotInfo()

	switch {
	case snapshotInfo != nil && !*forceLoadingSnapshot:
		if err := deps.SnapshotManager.CheckCurrentSnapshot(snapshotInfo); err != nil {
			return err
		}
	default:
		if err := deps.SnapshotManager.ImportSnapshots(CoreComponent.Daemon().ContextStopped()); err != nil {
			return err
		}
	}

	return nil
}

func run() error {

	newConfirmedMilestoneSignal := make(chan milestone.Index)
	onConfirmedMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		select {
		case newConfirmedMilestoneSignal <- msIndex:
		default:
		}
	})

	if err := CoreComponent.Daemon().BackgroundWorker("Snapshots", func(ctx context.Context) {
		CoreComponent.LogInfo("Starting Snapshots ... done")

		deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Attach(onConfirmedMilestoneIndexChanged)
		defer deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Detach(onConfirmedMilestoneIndexChanged)

		for {
			select {
			case <-ctx.Done():
				CoreComponent.LogInfo("Stopping Snapshots...")
				CoreComponent.LogInfo("Stopping Snapshots... done")
				return

			case confirmedMilestoneIndex := <-newConfirmedMilestoneSignal:
				deps.SnapshotManager.HandleNewConfirmedMilestoneEvent(ctx, confirmedMilestoneIndex)
			}
		}
	}, daemon.PrioritySnapshots); err != nil {
		CoreComponent.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}
