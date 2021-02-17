package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
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
	CorePlugin                          *node.CorePlugin
	log                                 *logger.Logger
	forceLoadingSnapshot                = flag.Bool(CfgSnapshotsForceLoadingSnapshot, false, "force loading of a snapshot, even if a database already exists")
	errInvalidSnapshotAvailabilityState = errors.New("invalid snapshot files availability")
	deps                                dependencies
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

		pruningDelay := milestone.Index(deps.NodeConfig.Int(CfgPruningDelay))
		pruningDelayMin := snapshotDepth + SolidEntryPointCheckThresholdPast + AdditionalPruningThreshold + 1
		if pruningDelay < pruningDelayMin {
			log.Warnf("Parameter '%s' is too small (%d). Value was changed to %d", CfgPruningDelay, pruningDelay, pruningDelayMin)
			pruningDelay = pruningDelayMin
		}

		if err := deps.NodeConfig.SetDefault(CfgSnapshotsDownloadURLs, []snapshot.DownloadTarget{}); err != nil {
			panic(err)
		}

		return snapshot.New(CorePlugin.Daemon().ContextStopped(),
			log,
			deps.Storage,
			deps.Tangle,
			deps.UTXO,
			deps.NodeConfig.String(CfgSnapshotsFullPath),
			deps.NodeConfig.String(CfgSnapshotsDeltaPath),
			SolidEntryPointCheckThresholdPast,
			SolidEntryPointCheckThresholdFuture,
			AdditionalPruningThreshold,
			snapshotDepth,
			milestone.Index(deps.NodeConfig.Int(CfgSnapshotsIntervalSynced)),
			milestone.Index(deps.NodeConfig.Int(CfgSnapshotsIntervalUnsynced)),
			deps.NodeConfig.Bool(CfgPruningEnabled),
			pruningDelay,
		)
	}); err != nil {
		panic(err)
	}
}

func configure() {

	snapshotInfo := deps.Storage.GetSnapshotInfo()

	switch {
	case snapshotInfo != nil && !*forceLoadingSnapshot:
		if err := checkCurrentSnapshot(snapshotInfo); err != nil {
			log.Panic(err.Error())
		}
	default:
		if err := importSnapshots(); err != nil {
			log.Panic(err.Error())
		}
	}

}

func run() {

	newSolidMilestoneSignal := make(chan milestone.Index)
	onSolidMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		select {
		case newSolidMilestoneSignal <- msIndex:
		default:
		}
	})

	_ = CorePlugin.Daemon().BackgroundWorker("Snapshots", func(shutdownSignal <-chan struct{}) {
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

// checks that the current snapshot info is valid regarding its network ID and the ledger state.
func checkCurrentSnapshot(snapshotInfo *storage.SnapshotInfo) error {

	// check that the stored snapshot corresponds to the wanted network ID
	if snapshotInfo.NetworkID != deps.NetworkID {
		networkIDSource := deps.NodeConfig.String(protocfg.CfgProtocolNetworkIDName)
		log.Panicf("node is configured to operate in network %d/%s but the stored snapshot data corresponds to %d", deps.NetworkID, networkIDSource, snapshotInfo.NetworkID)
	}

	// if we don't enforce loading of a snapshot,
	// we can check the ledger state of the current database and start the node.
	if err := deps.UTXO.CheckLedgerState(); err != nil {
		log.Fatal(err.Error())
	}

	return nil
}

// imports snapshot data from the configured file paths.
// automatically downloads snapshot data if no files are available.
func importSnapshots() error {
	fullPath := deps.NodeConfig.String(CfgSnapshotsFullPath)
	deltaPath := deps.NodeConfig.String(CfgSnapshotsDeltaPath)

	snapAvail, err := checkSnapshotFilesAvailability(fullPath, deltaPath)
	if err != nil {
		return err
	}

	if snapAvail == snapshotAvailNone {
		if err := downloadSnapshotFiles(fullPath, deltaPath); err != nil {
			return err
		}
	}

	if err := deps.Snapshot.LoadSnapshotFromFile(snapshot.Full, deps.NetworkID, fullPath); err != nil {
		deps.Storage.MarkDatabaseCorrupted()
		return err
	}

	if snapAvail == snapshotAvailOnlyFull {
		return nil
	}

	if err := deps.Snapshot.LoadSnapshotFromFile(snapshot.Delta, deps.NetworkID, deltaPath); err != nil {
		deps.Storage.MarkDatabaseCorrupted()
		return err
	}

	return nil
}

type snapshotAvailability byte

const (
	snapshotAvailBoth snapshotAvailability = iota
	snapshotAvailOnlyFull
	snapshotAvailNone
)

// checks that either both snapshot files are available, only the full snapshot or none.
func checkSnapshotFilesAvailability(fullPath string, deltaPath string) (snapshotAvailability, error) {
	switch {
	case len(fullPath) == 0:
		return 0, fmt.Errorf("%w: full snapshot file path not defined", snapshot.ErrNoSnapshotSpecified)
	case len(deltaPath) == 0:
		return 0, fmt.Errorf("%w: delta snapshot file path not defined", snapshot.ErrNoSnapshotSpecified)
	}

	_, fullSnapshotStatErr := os.Stat(fullPath)
	_, deltaSnapshotStatErr := os.Stat(deltaPath)

	switch {
	case os.IsNotExist(fullSnapshotStatErr) && deltaSnapshotStatErr == nil:
		// only having the delta snapshot file does not make sense,
		// as it relies on a full snapshot file to be available.
		// downloading the full snapshot would not help, as it will probably
		// be incompatible with the delta snapshot index.
		return 0, fmt.Errorf("%w: there exists a delta snapshot but not a full snapshot file, delete the delta snapshot file and restart", errInvalidSnapshotAvailabilityState)
	case os.IsNotExist(fullSnapshotStatErr) && os.IsNotExist(deltaSnapshotStatErr):
		return snapshotAvailNone, nil
	case fullSnapshotStatErr == nil && os.IsNotExist(deltaSnapshotStatErr):
		return snapshotAvailOnlyFull, nil
	default:
		return snapshotAvailBoth, nil
	}
}

// ensures that the folders to both paths exists and then downloads the appropriate snapshot files.
func downloadSnapshotFiles(fullPath string, deltaPath string) error {
	fullPathDir := filepath.Dir(fullPath)
	deltaPathDir := filepath.Dir(deltaPath)

	if err := os.MkdirAll(fullPathDir, 0700); err != nil {
		return fmt.Errorf("could not create snapshot dir '%s': %w", fullPath, err)
	}

	if err := os.MkdirAll(deltaPathDir, 0700); err != nil {
		return fmt.Errorf("could not create snapshot dir '%s': %w", fullPath, err)
	}

	var targets []snapshot.DownloadTarget
	if err := deps.NodeConfig.Unmarshal(CfgSnapshotsDownloadURLs, &targets); err != nil {
		panic(err)
	}

	if len(targets) == 0 {
		return snapshot.ErrNoSnapshotDownloadURL
	}

	targetsJson, err := json.MarshalIndent(targets, "", "   ")
	if err != nil {
		return fmt.Errorf("unable to marshal targets into formatted JSON: %w", err)
	}
	log.Infof("downloading snapshot files from one of the provided sources %s", string(targetsJson))

	if err := deps.Snapshot.DownloadSnapshotFiles(fullPath, deltaPath, targets); err != nil {
		return fmt.Errorf("unable to download snapshot files: %w", err)
	}

	log.Info("snapshot download finished")
	return nil
}
