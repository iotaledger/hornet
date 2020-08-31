package snapshot

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/gossip"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

var (
	PLUGIN = node.NewPlugin("Snapshot", node.Enabled, configure, run)
	log    *logger.Logger

	overwriteCooAddress = pflag.Bool("overwriteCooAddress", false, "apply new coordinator address from config file to database")
	forceGlobalSnapshot = pflag.Bool("forceGlobalSnapshot", false, "force loading of a global snapshot, even if a database already exists")

	ErrNoSnapshotSpecified             = errors.New("no snapshot file was specified in the config")
	ErrNoSnapshotDownloadURL           = fmt.Errorf("no download URL given for local snapshot under config option '%s", config.CfgLocalSnapshotsDownloadURLs)
	ErrSnapshotDownloadWasAborted      = errors.New("snapshot download was aborted")
	ErrSnapshotDownloadNoValidSource   = errors.New("no valid source found, snapshot download not possible")
	ErrSnapshotImportWasAborted        = errors.New("snapshot import was aborted")
	ErrSnapshotImportFailed            = errors.New("snapshot import failed")
	ErrSnapshotCreationWasAborted      = errors.New("operation was aborted")
	ErrSnapshotCreationFailed          = errors.New("creating snapshot failed")
	ErrTargetIndexTooNew               = errors.New("snapshot target is too new.")
	ErrTargetIndexTooOld               = errors.New("snapshot target is too old.")
	ErrNotEnoughHistory                = errors.New("not enough history.")
	ErrNoPruningNeeded                 = errors.New("no pruning needed.")
	ErrPruningAborted                  = errors.New("pruning was aborted.")
	ErrUnconfirmedTxInSubtangle        = errors.New("unconfirmed tx in subtangle")
	ErrInvalidBalance                  = errors.New("invalid balance! total does not match supply:")
	ErrWrongCoordinatorAddressDatabase = errors.New("configured coordinator address does not match database information")

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

	snapshotDepth = milestone.Index(config.NodeConfig.GetInt(config.CfgLocalSnapshotsDepth))
	if snapshotDepth < SolidEntryPointCheckThresholdFuture {
		log.Warnf("Parameter '%s' is too small (%d). Value was changed to %d", config.CfgLocalSnapshotsDepth, snapshotDepth, SolidEntryPointCheckThresholdFuture)
		snapshotDepth = SolidEntryPointCheckThresholdFuture
	}
	snapshotIntervalSynced = milestone.Index(config.NodeConfig.GetInt(config.CfgLocalSnapshotsIntervalSynced))
	snapshotIntervalUnsynced = milestone.Index(config.NodeConfig.GetInt(config.CfgLocalSnapshotsIntervalUnsynced))

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
		coordinatorAddress := hornet.HashFromAddressTrytes(config.NodeConfig.GetString(config.CfgCoordinatorAddress))

		// Check coordinator address in database
		if !bytes.Equal(snapshotInfo.CoordinatorAddress, coordinatorAddress) {
			if !*overwriteCooAddress {
				log.Panic(errors.Wrapf(ErrWrongCoordinatorAddressDatabase, "%v != %v", snapshotInfo.CoordinatorAddress.Trytes(), config.NodeConfig.GetString(config.CfgCoordinatorAddress)))
			}

			// Overwrite old coordinator address
			snapshotInfo.CoordinatorAddress = coordinatorAddress
			tangle.SetSnapshotInfo(snapshotInfo)
		}

		if !*forceGlobalSnapshot {
			// If we don't enforce loading of a global snapshot,
			// we can check the ledger state of current database and start the node.
			tangle.GetLedgerStateForLSMI(nil)
			return
		}
	}

	snapshotTypeToLoad := strings.ToLower(config.NodeConfig.GetString(config.CfgSnapshotLoadType))

	if *forceGlobalSnapshot && snapshotTypeToLoad != "global" {
		log.Fatalf("global snapshot enforced but wrong snapshot type under config option '%s': %s", config.CfgSnapshotLoadType, config.NodeConfig.GetString(config.CfgSnapshotLoadType))
	}

	var err = ErrNoSnapshotSpecified
	switch snapshotTypeToLoad {
	case "global":
		if path := config.NodeConfig.GetString(config.CfgGlobalSnapshotPath); path != "" {
			err = LoadGlobalSnapshot(path,
				config.NodeConfig.GetStringSlice(config.CfgGlobalSnapshotSpentAddressesPaths),
				milestone.Index(config.NodeConfig.GetInt(config.CfgGlobalSnapshotIndex)))
		}
	case "local":
		if path := config.NodeConfig.GetString(config.CfgLocalSnapshotsPath); path != "" {

			if _, fileErr := os.Stat(path); os.IsNotExist(fileErr) {
				// create dir if it not exists
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					log.Fatalf("could not create snapshot dir '%s'", path)
				}
				if urls := config.NodeConfig.GetStringSlice(config.CfgLocalSnapshotsDownloadURLs); len(urls) > 0 {
					log.Infof("Downloading snapshot from one of the provided sources %v", urls)
					downloadErr := downloadSnapshotFile(path, urls)
					if downloadErr != nil {
						err = errors.Wrap(downloadErr, "Error downloading snapshot file")
						break
					}
					log.Info("Snapshot download finished")
				} else {
					err = ErrNoSnapshotDownloadURL
					break
				}
			}

			err = LoadSnapshotFromFile(path)
		}
	default:
		log.Fatalf("invalid snapshot type under config option '%s': %s", config.CfgSnapshotLoadType, config.NodeConfig.GetString(config.CfgSnapshotLoadType))
	}

	if err != nil {
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
					localSnapshotPath := config.NodeConfig.GetString(config.CfgLocalSnapshotsPath)
					if err := createLocalSnapshotWithoutLocking(solidMilestoneIndex-snapshotDepth, localSnapshotPath, true, shutdownSignal); err != nil {
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
