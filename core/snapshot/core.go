package snapshot

import (
	"os"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/core/protocfg"
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
	"github.com/iotaledger/hive.go/logger"
)

const (
	// SolidEntryPointCheckAdditionalThresholdPast is the additional past cone (to BMD) that is walked to calculate the solid entry points
	SolidEntryPointCheckAdditionalThresholdPast = 5

	// SolidEntryPointCheckAdditionalThresholdFuture is the additional future cone (to BMD) that is needed to calculate solid entry points correctly
	SolidEntryPointCheckAdditionalThresholdFuture = 5

	// AdditionalPruningThreshold is the additional threshold (to BMD), which is needed, because the messages in the getMilestoneParents call in getSolidEntryPoints
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
			Name:      "Snapshot",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	CorePlugin           *node.CorePlugin
	log                  *logger.Logger
	forceLoadingSnapshot = flag.Bool(CfgSnapshotsForceLoadingSnapshot, false, "force loading of a snapshot, even if a database already exists")
	deps                 dependencies
)

type dependencies struct {
	dig.In
	Storage        *storage.Storage
	Tangle         *tangle.Tangle
	UTXO           *utxo.Manager
	Snapshot       *snapshot.Snapshot
	NodeConfig     *configuration.Configuration `name:"nodeConfig"`
	NetworkID      uint64                       `name:"networkId"`
	DeleteAllFlag  bool                         `name:"deleteAll"`
	StorageMetrics *metrics.StorageMetrics
}

func provide(c *dig.Container) {
	log = logger.NewLogger(CorePlugin.Name)

	type snapshotdeps struct {
		dig.In
		Storage       *storage.Storage
		UTXO          *utxo.Manager
		NodeConfig    *configuration.Configuration `name:"nodeConfig"`
		BelowMaxDepth int                          `name:"belowMaxDepth"`
		NetworkID     uint64                       `name:"networkId"`
	}

	if err := c.Provide(func(deps snapshotdeps) *snapshot.Snapshot {

		solidEntryPointCheckThresholdPast := milestone.Index(deps.BelowMaxDepth + SolidEntryPointCheckAdditionalThresholdPast)
		solidEntryPointCheckThresholdFuture := milestone.Index(deps.BelowMaxDepth + SolidEntryPointCheckAdditionalThresholdFuture)
		pruningThreshold := milestone.Index(deps.BelowMaxDepth + AdditionalPruningThreshold)

		snapshotDepth := milestone.Index(deps.NodeConfig.Int(CfgSnapshotsDepth))
		if snapshotDepth < solidEntryPointCheckThresholdFuture {
			log.Warnf("Parameter '%s' is too small (%d). Value was changed to %d", CfgSnapshotsDepth, snapshotDepth, solidEntryPointCheckThresholdFuture)
			snapshotDepth = solidEntryPointCheckThresholdFuture
		}

		pruningDelay := milestone.Index(deps.NodeConfig.Int(CfgPruningDelay))
		pruningDelayMin := snapshotDepth + solidEntryPointCheckThresholdPast + pruningThreshold + 1
		if pruningDelay < pruningDelayMin {
			log.Warnf("Parameter '%s' is too small (%d). Value was changed to %d", CfgPruningDelay, pruningDelay, pruningDelayMin)
			pruningDelay = pruningDelayMin
		}

		if err := deps.NodeConfig.SetDefault(CfgSnapshotsDownloadURLs, []snapshot.DownloadTarget{}); err != nil {
			panic(err)
		}

		networkIDSource := deps.NodeConfig.String(protocfg.CfgProtocolNetworkIDName)

		var downloadTargets []snapshot.DownloadTarget
		if err := deps.NodeConfig.Unmarshal(CfgSnapshotsDownloadURLs, &downloadTargets); err != nil {
			panic(err)
		}

		return snapshot.New(CorePlugin.Daemon().ContextStopped(),
			log,
			deps.Storage,
			deps.UTXO,
			deps.NetworkID,
			networkIDSource,
			deps.NodeConfig.String(CfgSnapshotsFullPath),
			deps.NodeConfig.String(CfgSnapshotsDeltaPath),
			downloadTargets,
			solidEntryPointCheckThresholdPast,
			solidEntryPointCheckThresholdFuture,
			pruningThreshold,
			snapshotDepth,
			milestone.Index(deps.NodeConfig.Int(CfgSnapshotsInterval)),
			deps.NodeConfig.Bool(CfgPruningEnabled),
			pruningDelay,
			deps.NodeConfig.Bool(CfgPruningPruneReceipts),
		)
	}); err != nil {
		panic(err)
	}
}

func configure() {

	if deps.DeleteAllFlag {
		// delete old snapshot files
		if err := os.Remove(deps.NodeConfig.String(CfgSnapshotsFullPath)); err != nil && !os.IsNotExist(err) {
			log.Panicf("deleting full snapshot file failed: %s", err)
		}

		if err := os.Remove(deps.NodeConfig.String(CfgSnapshotsDeltaPath)); err != nil && !os.IsNotExist(err) {
			log.Panicf("deleting delta snapshot file failed: %s", err)
		}
	}

	snapshotInfo := deps.Storage.GetSnapshotInfo()

	switch {
	case snapshotInfo != nil && !*forceLoadingSnapshot:
		if err := deps.Snapshot.CheckCurrentSnapshot(snapshotInfo); err != nil {
			log.Panic(err.Error())
		}
	default:
		if err := deps.Snapshot.ImportSnapshots(); err != nil {
			log.Panic(err.Error())
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

	_ = CorePlugin.Daemon().BackgroundWorker("Snapshots", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Snapshots ... done")

		deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Attach(onConfirmedMilestoneIndexChanged)
		defer deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Detach(onConfirmedMilestoneIndexChanged)

		for {
			select {
			case <-shutdownSignal:
				log.Info("Stopping Snapshots...")
				log.Info("Stopping Snapshots... done")
				return

			case confirmedMilestoneIndex := <-newConfirmedMilestoneSignal:
				deps.Snapshot.HandleNewConfirmedMilestoneEvent(confirmedMilestoneIndex, shutdownSignal)
			}
		}
	}, shutdown.PrioritySnapshots)
}
