package snapshot

import (
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"

	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/gohornet/hornet/pkg/tangle"
)

const (
	// SolidEntryPointCheckThresholdPast is the past cone that is walked to calculate the solid entry points
	SolidEntryPointCheckThresholdPast = 50

	// SolidEntryPointCheckThresholdFuture is the future cone that is needed to calculate solid entry points correctly
	SolidEntryPointCheckThresholdFuture = 50

	// AdditionalPruningThreshold is needed, because the messages in the getMilestoneParents call in getSolidEntryPoints
	// can reference older messages as well
	AdditionalPruningThreshold = 50
)

const (
	// force loading of a snapshot, even if a database already exists
	CfgSnapshotsForceLoadingSnapshot = "forceLoadingSnapshot"
)

func init() {
	flag.CommandLine.MarkHidden(CfgSnapshotsForceLoadingSnapshot)

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
	CorePlugin *node.CorePlugin
	log        *logger.Logger

	forceLoadingSnapshot = flag.Bool(CfgSnapshotsForceLoadingSnapshot, false, "force loading of a snapshot, even if a database already exists")

	newSolidMilestoneSignal = make(chan milestone.Index)

	pruningEnabled bool
	pruningDelay   milestone.Index

	deps dependencies
)

type dependencies struct {
	dig.In
	Storage    *storage.Storage
	Tangle     *tangle.Tangle
	UTXO       *utxo.Manager
	Snapshot   *snapshot.Snapshot
	NodeConfig *configuration.Configuration `name:"nodeConfig"`
	NetworkID  uint64                       `name:"networkId"`
}

func provide(c *dig.Container) {
	log = logger.NewLogger(CorePlugin.Name)

	type snapshotdeps struct {
		dig.In
		Storage    *storage.Storage
		Tangle     *tangle.Tangle
		UTXO       *utxo.Manager
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps snapshotdeps) *snapshot.Snapshot {

		snapshotDepth := milestone.Index(deps.NodeConfig.Int(CfgSnapshotsDepth))
		if snapshotDepth < SolidEntryPointCheckThresholdFuture {
			log.Warnf("Parameter '%s' is too small (%d). Value was changed to %d", CfgSnapshotsDepth, snapshotDepth, SolidEntryPointCheckThresholdFuture)
			snapshotDepth = SolidEntryPointCheckThresholdFuture
		}
		snapshotIntervalSynced := milestone.Index(deps.NodeConfig.Int(CfgSnapshotsIntervalSynced))
		snapshotIntervalUnsynced := milestone.Index(deps.NodeConfig.Int(CfgSnapshotsIntervalUnsynced))

		pruningEnabled = deps.NodeConfig.Bool(CfgPruningEnabled)
		pruningDelay = milestone.Index(deps.NodeConfig.Int(CfgPruningDelay))
		pruningDelayMin := snapshotDepth + SolidEntryPointCheckThresholdPast + AdditionalPruningThreshold + 1
		if pruningDelay < pruningDelayMin {
			log.Warnf("Parameter '%s' is too small (%d). Value was changed to %d", CfgPruningDelay, pruningDelay, pruningDelayMin)
			pruningDelay = pruningDelayMin
		}

		return snapshot.New(CorePlugin.Daemon().ContextStopped(),
			log,
			deps.Storage,
			deps.Tangle,
			deps.UTXO,
			deps.NodeConfig.String(CfgSnapshotsPath),
			SolidEntryPointCheckThresholdPast,
			SolidEntryPointCheckThresholdFuture,
			AdditionalPruningThreshold,
			snapshotDepth,
			snapshotIntervalSynced,
			snapshotIntervalUnsynced,
			pruningEnabled,
			pruningDelay,
		)
	}); err != nil {
		panic(err)
	}
}

func configure() {

	snapshotInfo := deps.Storage.GetSnapshotInfo()
	if snapshotInfo != nil {
		if !*forceLoadingSnapshot {

			// check that the stored snapshot corresponds to the wanted network ID
			if snapshotInfo.NetworkID != deps.NetworkID {
				networkIDSource := deps.NodeConfig.String(protocfg.CfgProtocolNetworkIDName)
				log.Panicf("node is configured to operate in network %d/%s but the stored snapshot data corresponds to %d", deps.NetworkID, networkIDSource, snapshotInfo.NetworkID)
			}

			// If we don't enforce loading of a snapshot,
			// we can check the ledger state of current database and start the node.
			if err := deps.UTXO.CheckLedgerState(); err != nil {
				log.Fatal(err.Error())
			}
			return
		}
	}

	path := deps.NodeConfig.String(CfgSnapshotsPath)
	if path == "" {
		log.Fatal(snapshot.ErrNoSnapshotSpecified.Error())
	}

	if _, fileErr := os.Stat(path); os.IsNotExist(fileErr) {
		// create dir if it not exists
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			log.Fatalf("could not create snapshot dir '%s'", path)
		}

		urls := deps.NodeConfig.Strings(CfgSnapshotsDownloadURLs)
		if len(urls) == 0 {
			log.Fatal(snapshot.ErrNoSnapshotDownloadURL.Error())
		}

		log.Infof("Downloading snapshot from one of the provided sources %v", urls)
		if err := deps.Snapshot.DownloadSnapshotFile(path, urls); err != nil {
			log.Fatalf("Error downloading snapshot file: %s", err)
		}

		log.Info("Snapshot download finished")
	}

	if err := deps.Snapshot.LoadFullSnapshotFromFile(deps.NetworkID, path); err != nil {
		deps.Storage.MarkDatabaseCorrupted()
		log.Panic(err.Error())
	}
}

func run() {

	onSolidMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		select {
		case newSolidMilestoneSignal <- msIndex:
		default:
		}
	})

	CorePlugin.Daemon().BackgroundWorker("Snapshots", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Snapshots ... done")

		deps.Tangle.Events.SolidMilestoneIndexChanged.Attach(onSolidMilestoneIndexChanged)
		defer deps.Tangle.Events.SolidMilestoneIndexChanged.Detach(onSolidMilestoneIndexChanged)

		for {
			select {
			case <-shutdownSignal:
				log.Info("Stopping Snapshots...")
				log.Info("Stopping Snapshots... done")
				return

			case solidMilestoneIndex := <-newSolidMilestoneSignal:
				deps.Snapshot.HandleNewSolidMilestoneEvent(solidMilestoneIndex, shutdownSignal)
			}
		}
	}, shutdown.PrioritySnapshots)
}
