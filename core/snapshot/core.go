package snapshot

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gohornet/hornet/core/protocfg"
	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/core/gossip"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
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
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	CorePlugin *node.CorePlugin
	log        *logger.Logger

	forceLoadingSnapshot = flag.Bool(CfgSnapshotsForceLoadingSnapshot, false, "force loading of a snapshot, even if a database already exists")

	ErrNoSnapshotSpecified               = errors.New("no snapshot file was specified in the config")
	ErrNoSnapshotDownloadURL             = fmt.Errorf("no download URL given for local snapshot under config option '%s", CfgSnapshotsDownloadURLs)
	ErrSnapshotDownloadWasAborted        = errors.New("snapshot download was aborted")
	ErrSnapshotDownloadNoValidSource     = errors.New("no valid source found, snapshot download not possible")
	ErrSnapshotImportWasAborted          = errors.New("snapshot import was aborted")
	ErrSnapshotImportFailed              = errors.New("snapshot import failed")
	ErrSnapshotCreationWasAborted        = errors.New("operation was aborted")
	ErrSnapshotCreationFailed            = errors.New("creating snapshot failed")
	ErrTargetIndexTooNew                 = errors.New("snapshot target is too new.")
	ErrTargetIndexTooOld                 = errors.New("snapshot target is too old.")
	ErrNotEnoughHistory                  = errors.New("not enough history.")
	ErrNoPruningNeeded                   = errors.New("no pruning needed.")
	ErrPruningAborted                    = errors.New("pruning was aborted.")
	ErrUnreferencedTxInSubtangle         = errors.New("unreferenced msg in subtangle")
	ErrInvalidBalance                    = errors.New("invalid balance! total does not match supply:")
	ErrWrongCoordinatorPublicKeyDatabase = errors.New("configured coordinator public key does not match database information")

	localSnapshotLock       = syncutils.Mutex{}
	newSolidMilestoneSignal = make(chan milestone.Index)

	snapshotDepth            milestone.Index
	snapshotIntervalSynced   milestone.Index
	snapshotIntervalUnsynced milestone.Index

	pruningEnabled bool
	pruningDelay   milestone.Index

	statusLock     syncutils.RWMutex
	isSnapshotting bool
	isPruning      bool

	deps dependencies
)

type dependencies struct {
	dig.In
	Storage    *storage.Storage
	Tangle     *tangle.Tangle
	UTXO       *utxo.Manager
	NodeConfig *configuration.Configuration `name:"nodeConfig"`
	NetworkID  uint64                       `name:"networkId"`
}

func configure() {
	log = logger.NewLogger(CorePlugin.Name)

	snapshotDepth = milestone.Index(deps.NodeConfig.Int(CfgSnapshotsDepth))
	if snapshotDepth < SolidEntryPointCheckThresholdFuture {
		log.Warnf("Parameter '%s' is too small (%d). Value was changed to %d", CfgSnapshotsDepth, snapshotDepth, SolidEntryPointCheckThresholdFuture)
		snapshotDepth = SolidEntryPointCheckThresholdFuture
	}
	snapshotIntervalSynced = milestone.Index(deps.NodeConfig.Int(CfgSnapshotsIntervalSynced))
	snapshotIntervalUnsynced = milestone.Index(deps.NodeConfig.Int(CfgSnapshotsIntervalUnsynced))

	pruningEnabled = deps.NodeConfig.Bool(CfgPruningEnabled)
	pruningDelay = milestone.Index(deps.NodeConfig.Int(CfgPruningDelay))
	pruningDelayMin := snapshotDepth + SolidEntryPointCheckThresholdPast + AdditionalPruningThreshold + 1
	if pruningDelay < pruningDelayMin {
		log.Warnf("Parameter '%s' is too small (%d). Value was changed to %d", CfgPruningDelay, pruningDelay, pruningDelayMin)
		pruningDelay = pruningDelayMin
	}

	gossip.AddRequestBackpressureSignal(isSnapshottingOrPruning)

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
		log.Fatal(ErrNoSnapshotSpecified.Error())
	}

	if _, fileErr := os.Stat(path); os.IsNotExist(fileErr) {
		// create dir if it not exists
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			log.Fatalf("could not create snapshot dir '%s'", path)
		}

		urls := deps.NodeConfig.Strings(CfgSnapshotsDownloadURLs)
		if len(urls) == 0 {
			log.Fatal(ErrNoSnapshotDownloadURL.Error())
		}

		log.Infof("Downloading snapshot from one of the provided sources %v", urls)
		if err := downloadSnapshotFile(path, urls); err != nil {
			log.Fatalf("Error downloading snapshot file: %s", err)
		}

		log.Info("Snapshot download finished")
	}

	if err := LoadFullSnapshotFromFile(deps.NetworkID, path); err != nil {
		deps.Storage.MarkDatabaseCorrupted()
		log.Panic(err.Error())
	}
}

func isSnapshottingOrPruning() bool {
	statusLock.RLock()
	defer statusLock.RUnlock()
	return isSnapshotting || isPruning
}

func run() {

	onSolidMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		select {
		case newSolidMilestoneSignal <- msIndex:
		default:
		}
	})

	CorePlugin.Daemon().BackgroundWorker("LocalSnapshots", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting LocalSnapshots ... done")

		deps.Tangle.Events.SolidMilestoneIndexChanged.Attach(onSolidMilestoneIndexChanged)
		defer deps.Tangle.Events.SolidMilestoneIndexChanged.Detach(onSolidMilestoneIndexChanged)

		for {
			select {
			case <-shutdownSignal:
				log.Info("Stopping LocalSnapshots...")
				log.Info("Stopping LocalSnapshots... done")
				return

			case solidMilestoneIndex := <-newSolidMilestoneSignal:
				localSnapshotLock.Lock()

				if shouldTakeSnapshot(solidMilestoneIndex) {
					localSnapshotPath := deps.NodeConfig.String(CfgSnapshotsPath)
					if err := createFullLocalSnapshotWithoutLocking(solidMilestoneIndex-snapshotDepth, localSnapshotPath, true, shutdownSignal); err != nil {
						if errors.Is(err, ErrCritical) {
							log.Panicf("%s %s", ErrSnapshotCreationFailed, err)
						}
						log.Warnf("%s %s", ErrSnapshotCreationFailed, err)
					}
				}

				if pruningEnabled {
					if solidMilestoneIndex <= pruningDelay {
						// Not enough history
						localSnapshotLock.Unlock()
						continue
					}

					if _, err := pruneDatabase(solidMilestoneIndex-pruningDelay, shutdownSignal); err != nil {
						log.Debugf("pruning aborted: %v", err)
					}
				}

				localSnapshotLock.Unlock()
			}
		}
	}, shutdown.PriorityLocalSnapshots)
}

func PruneDatabaseByDepth(depth milestone.Index) (milestone.Index, error) {
	localSnapshotLock.Lock()
	defer localSnapshotLock.Unlock()

	solidMilestoneIndex := deps.Storage.GetSolidMilestoneIndex()

	if solidMilestoneIndex <= depth {
		// Not enough history
		return 0, ErrNotEnoughHistory
	}

	return pruneDatabase(solidMilestoneIndex-depth, nil)
}

func PruneDatabaseByTargetIndex(targetIndex milestone.Index) (milestone.Index, error) {
	localSnapshotLock.Lock()
	defer localSnapshotLock.Unlock()

	return pruneDatabase(targetIndex, nil)
}
