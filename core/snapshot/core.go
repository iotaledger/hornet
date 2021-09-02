package snapshot

import (
	"os"

	"github.com/labstack/gommon/bytes"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
)

const (
	// SolidEntryPointCheckAdditionalThresholdPast is the additional past cone (to BMD) that is walked to calculate the solid entry points
	SolidEntryPointCheckAdditionalThresholdPast = 5

	// SolidEntryPointCheckAdditionalThresholdFuture is the additional future cone (to BMD) that is needed to calculate solid entry points correctly
	SolidEntryPointCheckAdditionalThresholdFuture = 5

	// AdditionalPruningThreshold is the additional threshold (to BMD), which is needed, because the messages in the getMilestoneParents call in solidEntryPoints
	// can reference older messages as well
	AdditionalPruningThreshold = 5
)

const (
	// force loading of a snapshot, even if a database already exists
	CfgSnapshotsForceLoadingSnapshot = "forceLoadingSnapshot"
)

func init() {
	_ = flag.CommandLine.MarkHidden(CfgSnapshotsForceLoadingSnapshot)

	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
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
	CorePlugin *node.CorePlugin
	deps       dependencies

	forceLoadingSnapshot = flag.Bool(CfgSnapshotsForceLoadingSnapshot, false, "force loading of a snapshot, even if a database already exists")
)

type dependencies struct {
	dig.In
	Storage              *storage.Storage
	Tangle               *tangle.Tangle
	UTXO                 *utxo.Manager
	Snapshot             *snapshot.Snapshot
	NodeConfig           *configuration.Configuration `name:"nodeConfig"`
	NetworkID            uint64                       `name:"networkId"`
	DeleteAllFlag        bool                         `name:"deleteAll"`
	PruningPruneReceipts bool                         `name:"pruneReceipts"`
	SnapshotsFullPath    string                       `name:"snapshotsFullPath"`
	SnapshotsDeltaPath   string                       `name:"snapshotsDeltaPath"`
	StorageMetrics       *metrics.StorageMetrics
}

func initConfigPars(c *dig.Container) {

	type cfgDeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	type cfgResult struct {
		dig.Out
		PruningPruneReceipts bool   `name:"pruneReceipts"`
		SnapshotsFullPath    string `name:"snapshotsFullPath"`
		SnapshotsDeltaPath   string `name:"snapshotsDeltaPath"`
	}

	if err := c.Provide(func(deps cfgDeps) cfgResult {
		return cfgResult{
			PruningPruneReceipts: deps.NodeConfig.Bool(CfgPruningPruneReceipts),
			SnapshotsFullPath:    deps.NodeConfig.String(CfgSnapshotsFullPath),
			SnapshotsDeltaPath:   deps.NodeConfig.String(CfgSnapshotsDeltaPath),
		}
	}); err != nil {
		CorePlugin.Panic(err)
	}
}

func provide(c *dig.Container) {

	type snapshotDeps struct {
		dig.In
		Database             *database.Database
		Storage              *storage.Storage
		UTXO                 *utxo.Manager
		NodeConfig           *configuration.Configuration `name:"nodeConfig"`
		BelowMaxDepth        int                          `name:"belowMaxDepth"`
		NetworkID            uint64                       `name:"networkId"`
		NetworkIDName        string                       `name:"networkIdName"`
		PruningPruneReceipts bool                         `name:"pruneReceipts"`
		SnapshotsFullPath    string                       `name:"snapshotsFullPath"`
		SnapshotsDeltaPath   string                       `name:"snapshotsDeltaPath"`
	}

	if err := c.Provide(func(deps snapshotDeps) *snapshot.Snapshot {

		networkIDSource := deps.NetworkIDName

		if err := deps.NodeConfig.SetDefault(CfgSnapshotsDownloadURLs, []snapshot.DownloadTarget{
			{
				Full:  "https://chrysalis-dbfiles.iota.org/snapshots/hornet/latest-full_snapshot.bin",
				Delta: "https://chrysalis-dbfiles.iota.org/snapshots/hornet/latest-delta_snapshot.bin",
			},
			{
				Full:  "https://cdn.tanglebay.com/snapshots/mainnet/full_snapshot.bin",
				Delta: "https://cdn.tanglebay.com/snapshots/mainnet/delta_snapshot.bin",
			},
		}); err != nil {
			CorePlugin.Panic(err)
		}

		var downloadTargets []*snapshot.DownloadTarget
		if err := deps.NodeConfig.Unmarshal(CfgSnapshotsDownloadURLs, &downloadTargets); err != nil {
			CorePlugin.Panic(err)
		}

		solidEntryPointCheckThresholdPast := milestone.Index(deps.BelowMaxDepth + SolidEntryPointCheckAdditionalThresholdPast)
		solidEntryPointCheckThresholdFuture := milestone.Index(deps.BelowMaxDepth + SolidEntryPointCheckAdditionalThresholdFuture)
		pruningThreshold := milestone.Index(deps.BelowMaxDepth + AdditionalPruningThreshold)

		snapshotDepth := milestone.Index(deps.NodeConfig.Int(CfgSnapshotsDepth))
		if snapshotDepth < solidEntryPointCheckThresholdFuture {
			CorePlugin.LogWarnf("parameter '%s' is too small (%d). value was changed to %d", CfgSnapshotsDepth, snapshotDepth, solidEntryPointCheckThresholdFuture)
			snapshotDepth = solidEntryPointCheckThresholdFuture
		}

		pruningMilestonesEnabled := deps.NodeConfig.Bool(CfgPruningMilestonesEnabled)
		pruningMilestonesMaxMilestonesToKeep := milestone.Index(deps.NodeConfig.Int(CfgPruningMilestonesMaxMilestonesToKeep))
		pruningMilestonesMaxMilestonesToKeepMin := snapshotDepth + solidEntryPointCheckThresholdPast + pruningThreshold + 1
		if pruningMilestonesMaxMilestonesToKeep != 0 && pruningMilestonesMaxMilestonesToKeep < pruningMilestonesMaxMilestonesToKeepMin {
			CorePlugin.LogWarnf("parameter '%s' is too small (%d). value was changed to %d", CfgPruningMilestonesMaxMilestonesToKeep, pruningMilestonesMaxMilestonesToKeep, pruningMilestonesMaxMilestonesToKeepMin)
			pruningMilestonesMaxMilestonesToKeep = pruningMilestonesMaxMilestonesToKeepMin
		}

		if pruningMilestonesEnabled && pruningMilestonesMaxMilestonesToKeep == 0 {
			CorePlugin.Panicf("%s has to be specified if %s is enabled", CfgPruningMilestonesMaxMilestonesToKeep, CfgPruningMilestonesEnabled)
		}

		pruningSizeEnabled := deps.NodeConfig.Bool(CfgPruningSizeEnabled)
		pruningTargetDatabaseSizeBytes, err := bytes.Parse(deps.NodeConfig.String(CfgPruningSizeTargetSize))
		if err != nil {
			CorePlugin.Panicf("parameter %s invalid", CfgPruningSizeTargetSize)
		}

		if pruningSizeEnabled && pruningTargetDatabaseSizeBytes == 0 {
			CorePlugin.Panicf("%s has to be specified if %s is enabled", CfgPruningSizeTargetSize, CfgPruningSizeEnabled)
		}

		return snapshot.New(CorePlugin.Daemon().ContextStopped(),
			CorePlugin.Logger(),
			deps.Database,
			deps.Storage,
			deps.UTXO,
			deps.NetworkID,
			networkIDSource,
			deps.SnapshotsFullPath,
			deps.SnapshotsDeltaPath,
			deps.NodeConfig.Float64(CfgSnapshotsDeltaSizeThresholdPercentage),
			downloadTargets,
			solidEntryPointCheckThresholdPast,
			solidEntryPointCheckThresholdFuture,
			pruningThreshold,
			snapshotDepth,
			milestone.Index(deps.NodeConfig.Int(CfgSnapshotsInterval)),
			pruningMilestonesEnabled,
			pruningMilestonesMaxMilestonesToKeep,
			pruningSizeEnabled,
			pruningTargetDatabaseSizeBytes,
			deps.NodeConfig.Float64(CfgPruningSizeThresholdPercentage),
			deps.NodeConfig.Duration(CfgPruningSizeCooldownTime),
			deps.PruningPruneReceipts,
		)
	}); err != nil {
		CorePlugin.Panic(err)
	}
}

func configure() {

	if deps.DeleteAllFlag {
		// delete old snapshot files
		if err := os.Remove(deps.SnapshotsFullPath); err != nil && !os.IsNotExist(err) {
			CorePlugin.Panicf("deleting full snapshot file failed: %s", err)
		}

		if err := os.Remove(deps.SnapshotsDeltaPath); err != nil && !os.IsNotExist(err) {
			CorePlugin.Panicf("deleting delta snapshot file failed: %s", err)
		}
	}

	snapshotInfo := deps.Storage.SnapshotInfo()

	switch {
	case snapshotInfo != nil && !*forceLoadingSnapshot:
		if err := deps.Snapshot.CheckCurrentSnapshot(snapshotInfo); err != nil {
			CorePlugin.Panic(err)
		}
	default:
		if err := deps.Snapshot.ImportSnapshots(); err != nil {
			CorePlugin.Panic(err)
		}
	}

}

func run() {

	newConfirmedMilestoneSignal := make(chan milestone.Index)
	onConfirmedMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		select {
		case newConfirmedMilestoneSignal <- msIndex:
		default:
		}
	})

	if err := CorePlugin.Daemon().BackgroundWorker("Snapshots", func(shutdownSignal <-chan struct{}) {
		CorePlugin.LogInfo("Starting Snapshots ... done")

		deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Attach(onConfirmedMilestoneIndexChanged)
		defer deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Detach(onConfirmedMilestoneIndexChanged)

		for {
			select {
			case <-shutdownSignal:
				CorePlugin.LogInfo("Stopping Snapshots...")
				CorePlugin.LogInfo("Stopping Snapshots... done")
				return

			case confirmedMilestoneIndex := <-newConfirmedMilestoneSignal:
				deps.Snapshot.HandleNewConfirmedMilestoneEvent(confirmedMilestoneIndex, shutdownSignal)
			}
		}
	}, shutdown.PrioritySnapshots); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}
}
