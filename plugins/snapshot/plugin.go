package snapshot

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/plugins/gossip"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

var (
	PLUGIN = node.NewPlugin("Snapshot", node.Enabled, configure, run)
	log    *logger.Logger

	overwriteCooAddress  = pflag.Bool("overwriteCooAddress", false, "apply new coordinator address from config file to database")
	forceLoadingSnapshot = pflag.Bool("forceLoadingSnapshot", false, "force loading of a snapshot, even if a database already exists")

	ErrNoSnapshotSpecified               = errors.New("no snapshot file was specified in the config")
	ErrNoSnapshotDownloadURL             = fmt.Errorf("no download URL given for local snapshot under config option '%s", config.CfgSnapshotsDownloadURLs)
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
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	snapshotDepth = milestone.Index(config.NodeConfig.GetInt(config.CfgSnapshotsDepth))
	if snapshotDepth < SolidEntryPointCheckThresholdFuture {
		log.Warnf("Parameter '%s' is too small (%d). Value was changed to %d", config.CfgSnapshotsDepth, snapshotDepth, SolidEntryPointCheckThresholdFuture)
		snapshotDepth = SolidEntryPointCheckThresholdFuture
	}
	snapshotIntervalSynced = milestone.Index(config.NodeConfig.GetInt(config.CfgSnapshotsIntervalSynced))
	snapshotIntervalUnsynced = milestone.Index(config.NodeConfig.GetInt(config.CfgSnapshotsIntervalUnsynced))

	pruningEnabled = config.NodeConfig.GetBool(config.CfgPruningEnabled)
	pruningDelay = milestone.Index(config.NodeConfig.GetInt(config.CfgPruningDelay))
	pruningDelayMin := snapshotDepth + SolidEntryPointCheckThresholdPast + AdditionalPruningThreshold + 1
	if pruningDelay < pruningDelayMin {
		log.Warnf("Parameter '%s' is too small (%d). Value was changed to %d", config.CfgPruningDelay, pruningDelay, pruningDelayMin)
		pruningDelay = pruningDelayMin
	}

	gossip.AddRequestBackpressureSignal(isSnapshottingOrPruning)

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		cooPublicKey, err := utils.ParseEd25519PublicKeyFromString(config.NodeConfig.GetString(config.CfgCoordinatorPublicKey))
		if err != nil {
			log.Fatal(err.Error())
		}

		// Check coordinator address in database
		if !snapshotInfo.CoordinatorPublicKey.Equal(cooPublicKey) {
			if !*overwriteCooAddress {
				snapshotCooPublicKey := snapshotInfo.CoordinatorPublicKey[:]
				configCooPublicKey := cooPublicKey[:]

				log.Panic(errors.Wrapf(ErrWrongCoordinatorPublicKeyDatabase, "%v != %v", hex.EncodeToString(snapshotCooPublicKey), hex.EncodeToString(configCooPublicKey)))
			}

			// Overwrite old coordinator address
			snapshotInfo.CoordinatorPublicKey = cooPublicKey
			tangle.SetSnapshotInfo(snapshotInfo)
		}

		if !*forceLoadingSnapshot {
			// If we don't enforce loading of a snapshot,
			// we can check the ledger state of current database and start the node.
			if err := utxo.CheckLedgerState(); err != nil {
				log.Fatal(err.Error())
			}
			return
		}
	}

	path := config.NodeConfig.GetString(config.CfgSnapshotsPath)
	if path == "" {
		log.Fatal(ErrNoSnapshotSpecified.Error())
	}

	if _, fileErr := os.Stat(path); os.IsNotExist(fileErr) {
		// create dir if it not exists
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			log.Fatalf("could not create snapshot dir '%s'", path)
		}

		urls := config.NodeConfig.GetStringSlice(config.CfgSnapshotsDownloadURLs)
		if len(urls) == 0 {
			log.Fatal(ErrNoSnapshotDownloadURL.Error())
		}

		log.Infof("Downloading snapshot from one of the provided sources %v", urls)
		if err := downloadSnapshotFile(path, urls); err != nil {
			log.Fatalf("Error downloading snapshot file: %w", err)
		}

		log.Info("Snapshot download finished")
	}

	if err := LoadFullSnapshotFromFile(path); err != nil {
		tangle.MarkDatabaseCorrupted()
		log.Panic(err.Error())
	}
}

func isSnapshottingOrPruning() bool {
	statusLock.RLock()
	defer statusLock.RUnlock()
	return isSnapshotting || isPruning
}

func run(_ *node.Plugin) {

	onSolidMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		select {
		case newSolidMilestoneSignal <- msIndex:
		default:
		}
	})

	daemon.BackgroundWorker("LocalSnapshots", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting LocalSnapshots ... done")

		tanglePlugin.Events.SolidMilestoneIndexChanged.Attach(onSolidMilestoneIndexChanged)
		defer tanglePlugin.Events.SolidMilestoneIndexChanged.Detach(onSolidMilestoneIndexChanged)

		for {
			select {
			case <-shutdownSignal:
				log.Info("Stopping LocalSnapshots...")
				log.Info("Stopping LocalSnapshots... done")
				return

			case solidMilestoneIndex := <-newSolidMilestoneSignal:
				localSnapshotLock.Lock()

				if shouldTakeSnapshot(solidMilestoneIndex) {
					localSnapshotPath := config.NodeConfig.GetString(config.CfgSnapshotsPath)
					if err := createFullLocalSnapshotWithoutLocking(solidMilestoneIndex-snapshotDepth, localSnapshotPath, true, shutdownSignal); err != nil {
						if errors.Is(err, ErrCritical) {
							log.Panic(errors.Wrap(ErrSnapshotCreationFailed, err.Error()))
						}
						log.Warn(errors.Wrap(ErrSnapshotCreationFailed, err.Error()))
					}
				}

				if pruningEnabled {
					if solidMilestoneIndex <= pruningDelay {
						// Not enough history
						localSnapshotLock.Unlock()
						continue
					}

					if err := pruneDatabase(solidMilestoneIndex-pruningDelay, shutdownSignal); err != nil {
						log.Debugf("pruning aborted: %v", err.Error())
					}
				}

				localSnapshotLock.Unlock()
			}
		}
	}, shutdown.PriorityLocalSnapshots)
}

func PruneDatabaseByDepth(depth milestone.Index) error {
	localSnapshotLock.Lock()
	defer localSnapshotLock.Unlock()

	solidMilestoneIndex := tangle.GetSolidMilestoneIndex()

	if solidMilestoneIndex <= depth {
		// Not enough history
		return ErrNotEnoughHistory
	}

	return pruneDatabase(solidMilestoneIndex-depth, nil)
}

func PruneDatabaseByTargetIndex(targetIndex milestone.Index) error {
	localSnapshotLock.Lock()
	defer localSnapshotLock.Unlock()

	return pruneDatabase(targetIndex, nil)
}
