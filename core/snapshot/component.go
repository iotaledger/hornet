package snapshot

import (
	"context"
	"os"

	"github.com/labstack/gommon/bytes"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/protocol"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// SolidEntryPointCheckAdditionalThresholdPast is the additional past cone (to BMD) that is walked to calculate the solid entry points.
	SolidEntryPointCheckAdditionalThresholdPast = 5

	// SolidEntryPointCheckAdditionalThresholdFuture is the additional future cone (to BMD) that is needed to calculate solid entry points correctly.
	SolidEntryPointCheckAdditionalThresholdFuture = 5
)

const (
	// CfgSnapshotsForceLoadingSnapshot defines the force loading of a snapshot, even if a database already exists.
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
	Storage            *storage.Storage
	Tangle             *tangle.Tangle
	UTXOManager        *utxo.Manager
	SnapshotImporter   *snapshot.Importer
	SnapshotManager    *snapshot.Manager
	SnapshotsFullPath  string `name:"snapshotsFullPath"`
	SnapshotsDeltaPath string `name:"snapshotsDeltaPath"`
	StorageMetrics     *metrics.StorageMetrics
}

func initConfigPars(c *dig.Container) error {

	type cfgResult struct {
		dig.Out
		SnapshotsFullPath  string `name:"snapshotsFullPath"`
		SnapshotsDeltaPath string `name:"snapshotsDeltaPath"`
	}

	return c.Provide(func() cfgResult {
		return cfgResult{
			SnapshotsFullPath:  ParamsSnapshots.FullPath,
			SnapshotsDeltaPath: ParamsSnapshots.DeltaPath,
		}
	})
}

func provide(c *dig.Container) error {

	type snapshotImporterDeps struct {
		dig.In
		DeleteAllFlag        bool `name:"deleteAll"`
		PruningPruneReceipts bool `name:"pruneReceipts"`
		Storage              *storage.Storage
		SnapshotsFullPath    string `name:"snapshotsFullPath"`
		SnapshotsDeltaPath   string `name:"snapshotsDeltaPath"`
		TargetNetworkName    string `name:"targetNetworkName"`
	}

	if err := c.Provide(func(deps snapshotImporterDeps) *snapshot.Importer {

		if deps.DeleteAllFlag {
			// delete old snapshot files
			if err := os.Remove(deps.SnapshotsFullPath); err != nil && !os.IsNotExist(err) {
				CoreComponent.LogErrorfAndExit("deleting full snapshot file failed: %s", err)
			}

			if err := os.Remove(deps.SnapshotsDeltaPath); err != nil && !os.IsNotExist(err) {
				CoreComponent.LogErrorfAndExit("deleting delta snapshot file failed: %s", err)
			}
		}

		importer := snapshot.NewSnapshotImporter(
			CoreComponent.Logger(),
			deps.Storage,
			deps.SnapshotsFullPath,
			deps.SnapshotsDeltaPath,
			deps.TargetNetworkName,
			ParamsSnapshots.DownloadURLs,
		)

		switch {
		case deps.Storage.SnapshotInfo() != nil && !*forceLoadingSnapshot:
			// snapshot already exists, no need to load it
		default:
			// import the initial snapshot
			if err := importer.ImportSnapshots(CoreComponent.Daemon().ContextStopped()); err != nil {
				CoreComponent.LogErrorAndExit(err)
			}
		}

		return importer
	}); err != nil {
		return err
	}

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

	return c.Provide(func(deps snapshotDeps) *snapshot.Manager {
		deltaSnapshotSizeThresholdMinSizeBytes, err := bytes.Parse(ParamsSnapshots.DeltaSizeThresholdMinSize)
		if err != nil {
			CoreComponent.LogPanicf("parameter %s invalid", CoreComponent.App().Config().GetParameterPath(&(ParamsSnapshots.DeltaSizeThresholdMinSize)))
		}

		solidEntryPointCheckThresholdPast := syncmanager.MilestoneIndexDelta(deps.ProtocolManager.Current().BelowMaxDepth + SolidEntryPointCheckAdditionalThresholdPast)
		solidEntryPointCheckThresholdFuture := syncmanager.MilestoneIndexDelta(deps.ProtocolManager.Current().BelowMaxDepth + SolidEntryPointCheckAdditionalThresholdFuture)

		snapshotDepth := syncmanager.MilestoneIndexDelta(ParamsSnapshots.Depth)
		if snapshotDepth < solidEntryPointCheckThresholdFuture {
			CoreComponent.LogWarnf("parameter '%s' is too small (%d). value was changed to %d", CoreComponent.App().Config().GetParameterPath(&(ParamsSnapshots.Depth)), snapshotDepth, solidEntryPointCheckThresholdFuture)
			snapshotDepth = solidEntryPointCheckThresholdFuture
		}

		return snapshot.NewSnapshotManager(
			CoreComponent.Logger(),
			deps.Storage,
			deps.SyncManager,
			deps.UTXOManager,
			ParamsSnapshots.Enabled,
			deps.SnapshotsFullPath,
			deps.SnapshotsDeltaPath,
			ParamsSnapshots.DeltaSizeThresholdPercentage,
			deltaSnapshotSizeThresholdMinSizeBytes,
			solidEntryPointCheckThresholdPast,
			solidEntryPointCheckThresholdFuture,
			snapshotDepth,
			syncmanager.MilestoneIndexDelta(ParamsSnapshots.Interval),
		)
	})
}

func run() error {

	newConfirmedMilestoneSignal := make(chan iotago.MilestoneIndex)
	onConfirmedMilestoneIndexChanged := events.NewClosure(func(msIndex iotago.MilestoneIndex) {
		select {
		case newConfirmedMilestoneSignal <- msIndex:
		default:
		}
	})

	if err := CoreComponent.Daemon().BackgroundWorker("Snapshots", func(ctx context.Context) {
		CoreComponent.LogInfo("Starting snapshot background worker ... done")

		deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Hook(onConfirmedMilestoneIndexChanged)
		defer deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Detach(onConfirmedMilestoneIndexChanged)

		for {
			select {
			case <-ctx.Done():
				CoreComponent.LogInfo("Stopping snapshot background worker ...")
				CoreComponent.LogInfo("Stopping snapshot background worker ... done")

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
