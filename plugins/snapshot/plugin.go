package snapshot

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/compressed"
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

	ErrNoSnapshotSpecified             = errors.New("no snapshot file was specified in the config")
	ErrNoSnapshotDownloadURL           = fmt.Errorf("no download URL given for local snapshot under config option '%s", config.CfgLocalSnapshotsDownloadURL)
	ErrSnapshotDownloadWasAborted      = errors.New("snapshot download was aborted")
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
	installGenesisTransaction()

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
		coordinatorAddress := hornet.Hash(trinary.MustTrytesToBytes(config.NodeConfig.GetString(config.CfgCoordinatorAddress)[:81])[:49])

		// Check coordinator address in database
		if !bytes.Equal(snapshotInfo.CoordinatorAddress, coordinatorAddress) {
			if !*overwriteCooAddress {
				log.Panic(errors.Wrapf(ErrWrongCoordinatorAddressDatabase, "%v != %v", snapshotInfo.CoordinatorAddress, config.NodeConfig.GetString(config.CfgCoordinatorAddress)[:81]))
			}

			// Overwrite old coordinator address
			snapshotInfo.CoordinatorAddress = coordinatorAddress
			tangle.SetSnapshotInfo(snapshotInfo)
		}

		// Check the ledger state
		tangle.GetLedgerStateForLSMI(nil)
		return
	}

	var err = ErrNoSnapshotSpecified

	snapshotTypeToLoad := config.NodeConfig.GetString(config.CfgSnapshotLoadType)
	switch strings.ToLower(snapshotTypeToLoad) {
	case "global":
		if path := config.NodeConfig.GetString(config.CfgGlobalSnapshotPath); path != "" {
			err = LoadGlobalSnapshot(path,
				config.NodeConfig.GetStringSlice(config.CfgGlobalSnapshotSpentAddressesPaths),
				milestone.Index(config.NodeConfig.GetInt(config.CfgGlobalSnapshotIndex)))
		}
	case "local":
		if path := config.NodeConfig.GetString(config.CfgLocalSnapshotsPath); path != "" {

			if _, fileErr := os.Stat(path); os.IsNotExist(fileErr) {
				if url := config.NodeConfig.GetString(config.CfgLocalSnapshotsDownloadURL); url != "" {
					log.Infof("Downloading snapshot from %s", url)
					downloadErr := downloadSnapshotFile(path, url)
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
		log.Fatalf("invalid snapshot type under config option '%s': %s", config.CfgSnapshotLoadType, snapshotTypeToLoad)
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

	notifyNewSolidMilestone := events.NewClosure(func(cachedBndl *tangle.CachedBundle) {
		select {
		case newSolidMilestoneSignal <- cachedBndl.GetBundle().GetMilestoneIndex():
		default:
		}
		cachedBndl.Release(true) // bundle -1
	})

	daemon.BackgroundWorker("LocalSnapshots", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting LocalSnapshots ... done")

		tanglePlugin.Events.SolidMilestoneChanged.Attach(notifyNewSolidMilestone)

		for {
			select {
			case <-shutdownSignal:
				log.Info("Stopping LocalSnapshots...")
				tanglePlugin.Events.SolidMilestoneChanged.Detach(notifyNewSolidMilestone)
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
						return
					}

					pruneDatabase(solidMilestoneIndex-pruningDelay, shutdownSignal)
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

func installGenesisTransaction() {
	// ensure genesis transaction exists
	genesisTxTrits := make(trinary.Trits, consts.TransactionTrinarySize)
	genesis, _ := transaction.ParseTransaction(genesisTxTrits, true)
	genesis.Hash = consts.NullHashTrytes
	txBytesTruncated := compressed.TruncateTx(trinary.MustTritsToBytes(genesisTxTrits))
	genesisTx := hornet.NewTransactionFromTx(genesis, txBytesTruncated)

	// ensure the bundle is also existent for the genesis tx
	cachedTx, _ := tangle.AddTransactionToStorage(genesisTx, 0, false, false, true)
	cachedTx.Release() // tx -1
}
