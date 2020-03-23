package snapshot

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/packages/compressed"
	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

var (
	PLUGIN = node.NewPlugin("Snapshot", node.Enabled, configure, run)
	log    *logger.Logger

	ErrNoSnapshotSpecified             = errors.New("no snapshot file was specified in the config")
	ErrSnapshotImportWasAborted        = errors.New("snapshot import was aborted")
	ErrSnapshotImportFailed            = errors.New("snapshot import failed")
	ErrSnapshotCreationWasAborted      = errors.New("operation was aborted")
	ErrSnapshotCreationFailed          = errors.New("creating snapshot failed: %v")
	ErrTargetIndexTooNew               = errors.New("snapshot target is too new.")
	ErrTargetIndexTooOld               = errors.New("snapshot target is too old.")
	ErrNotEnoughHistory                = errors.New("not enough history.")
	ErrUnconfirmedTxInSubtangle        = errors.New("Unconfirmed tx in subtangle")
	ErrInvalidBalance                  = errors.New("Invalid balance! Total does not match supply:")
	ErrWrongCoordinatorAddressDatabase = errors.New("Configured coordinator address does not match database information")

	localSnapshotLock       = syncutils.Mutex{}
	newSolidMilestoneSignal = make(chan milestone_index.MilestoneIndex)

	localSnapshotsEnabled    bool
	snapshotDepth            milestone_index.MilestoneIndex
	snapshotIntervalSynced   milestone_index.MilestoneIndex
	snapshotIntervalUnsynced milestone_index.MilestoneIndex

	pruningEnabled bool
	pruningDelay   milestone_index.MilestoneIndex
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)
	installGenesisTransaction()

	localSnapshotsEnabled = config.NodeConfig.GetBool(config.CfgLocalSnapshotsEnabled)
	snapshotDepth = milestone_index.MilestoneIndex(config.NodeConfig.GetInt(config.CfgLocalSnapshotsDepth))
	if snapshotDepth < SolidEntryPointCheckThresholdFuture {
		log.Warnf("Parameter '%s' is too small (%d). Value was changed to %d", config.CfgLocalSnapshotsDepth, snapshotDepth, SolidEntryPointCheckThresholdFuture)
		snapshotDepth = SolidEntryPointCheckThresholdFuture
	}
	snapshotIntervalSynced = milestone_index.MilestoneIndex(config.NodeConfig.GetInt(config.CfgLocalSnapshotsIntervalSynced))
	snapshotIntervalUnsynced = milestone_index.MilestoneIndex(config.NodeConfig.GetInt(config.CfgLocalSnapshotsIntervalUnsynced))

	pruningEnabled = config.NodeConfig.GetBool(config.CfgPruningEnabled)
	pruningDelay = milestone_index.MilestoneIndex(config.NodeConfig.GetInt(config.CfgPruningDelay))
	pruningDelayMin := snapshotDepth + SolidEntryPointCheckThresholdPast + AdditionalPruningThreshold + 1
	if pruningDelay < pruningDelayMin {
		log.Warnf("Parameter '%s' is too small (%d). Value was changed to %d", config.CfgPruningDelay, pruningDelay, pruningDelayMin)
		pruningDelay = pruningDelayMin
	}

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		// Check coordinator address in database
		if snapshotInfo.CoordinatorAddress != config.NodeConfig.GetString(config.CfgMilestoneCoordinator)[:81] {
			log.Panic(errors.Wrapf(ErrWrongCoordinatorAddressDatabase, "%v != %v", snapshotInfo.CoordinatorAddress, config.NodeConfig.GetString(config.CfgMilestoneCoordinator)[:81]))
		}

		// Check the ledger state
		tangle.GetAllLedgerBalances(nil)
		return
	}

	var err = ErrNoSnapshotSpecified

	snapshotTypeToLoad := config.NodeConfig.GetString(config.CfgSnapshotLoadType)
	switch strings.ToLower(snapshotTypeToLoad) {
	case "global":
		if path := config.NodeConfig.GetString(config.CfgGlobalSnapshotPath); path != "" {
			err = LoadGlobalSnapshot(path,
				config.NodeConfig.GetStringSlice(config.CfgGlobalSnapshotSpentAddressesPaths),
				milestone_index.MilestoneIndex(config.NodeConfig.GetInt(config.CfgGlobalSnapshotIndex)))
		}
	case "local":
		if path := config.NodeConfig.GetString(config.CfgLocalSnapshotsPath); path != "" {
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

func run(plugin *node.Plugin) {

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
				if localSnapshotsEnabled {
					localSnapshotLock.Lock()

					if shouldTakeSnapshot(solidMilestoneIndex) {
						localSnapshotPath := config.NodeConfig.GetString(config.CfgLocalSnapshotsPath)
						if err := createLocalSnapshotWithoutLocking(solidMilestoneIndex-snapshotDepth, localSnapshotPath, shutdownSignal); err != nil {
							log.Warnf(ErrSnapshotCreationFailed.Error(), err)
						}
					}

					if pruningEnabled {
						pruneDatabase(solidMilestoneIndex, shutdownSignal)
					}

					localSnapshotLock.Unlock()
				}
			}
		}
	}, shutdown.ShutdownPriorityLocalSnapshots)
}

func installGenesisTransaction() {
	// ensure genesis transaction exists
	genesisTxTrits := make(trinary.Trits, consts.TransactionTrinarySize)
	genesis, _ := transaction.ParseTransaction(genesisTxTrits, true)
	genesis.Hash = consts.NullHashTrytes
	txBytesTruncated := compressed.TruncateTx(trinary.MustTritsToBytes(genesisTxTrits))
	genesisTx := hornet.NewTransaction(genesis, txBytesTruncated)

	// ensure the bundle is also existent for the genesis tx
	cachedTx, _ := tangle.AddTransactionToStorage(genesisTx, 0, false, false, true)
	cachedTx.Release() // tx -1
}
