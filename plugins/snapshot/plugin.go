package snapshot

import (
	"errors"
	"strings"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/packages/compressed"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/parameter"
	"github.com/gohornet/hornet/packages/shutdown"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

var (
	PLUGIN = node.NewPlugin("Snapshot", node.Enabled, configure, run)
	log    *logger.Logger

	ErrNoSnapshotSpecified        = errors.New("no snapshot file was specified in the config")
	ErrSnapshotImportWasAborted   = errors.New("snapshot import was aborted")
	ErrSnapshotImportFailed       = errors.New("snapshot import failed")
	ErrSnapshotCreationWasAborted = errors.New("operation was aborted")
	ErrSnapshotCreationFailed     = errors.New("creating snapshot failed: %v")
	ErrTargetIndexTooNew          = errors.New("snapshot target is too new.")
	ErrTargetIndexTooOld          = errors.New("snapshot target is too old.")

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

	localSnapshotsEnabled = parameter.NodeConfig.GetBool("localSnapshots.enabled")
	snapshotDepth = milestone_index.MilestoneIndex(parameter.NodeConfig.GetInt("localSnapshots.depth"))
	if snapshotDepth < SolidEntryPointCheckThresholdFuture {
		log.Warnf("Parameter \"localSnapshots.depth\" is too small (%d). Value was changed to %d", snapshotDepth, SolidEntryPointCheckThresholdFuture)
		snapshotDepth = SolidEntryPointCheckThresholdFuture
	}
	snapshotIntervalSynced = milestone_index.MilestoneIndex(parameter.NodeConfig.GetInt("localSnapshots.intervalSynced"))
	snapshotIntervalUnsynced = milestone_index.MilestoneIndex(parameter.NodeConfig.GetInt("localSnapshots.intervalUnsynced"))

	pruningEnabled = parameter.NodeConfig.GetBool("pruning.enabled")
	pruningDelay = milestone_index.MilestoneIndex(parameter.NodeConfig.GetInt("pruning.delay"))
	pruningDelayMin := snapshotDepth + SolidEntryPointCheckThresholdPast + AdditionalPruningThreshold + 1
	if pruningDelay < pruningDelayMin {
		log.Warnf("Parameter \"pruning.delay\" is too small (%d). Value was changed to %d", pruningDelay, pruningDelayMin)
		pruningDelay = pruningDelayMin
	}
}

func run(plugin *node.Plugin) {

	notifyNewSolidMilestone := events.NewClosure(func(cachedBndl *tangle.CachedBundle) {
		select {
		case newSolidMilestoneSignal <- cachedBndl.GetBundle().GetMilestoneIndex():
		default:
		}
		cachedBndl.Release() // bundle -1
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
						if err := createLocalSnapshotWithoutLocking(solidMilestoneIndex-snapshotDepth, parameter.NodeConfig.GetString("localSnapshots.path"), shutdownSignal); err != nil {
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

	if tangle.GetSnapshotInfo() != nil {
		// Check the ledger state
		tangle.GetAllBalances(nil)
		return
	}

	var err = ErrNoSnapshotSpecified

	snapshotToLoad := parameter.NodeConfig.GetString("loadSnapshot")

	if strings.EqualFold(snapshotToLoad, "global") {
		if path := parameter.NodeConfig.GetString("globalSnapshot.path"); path != "" {
			err = LoadGlobalSnapshot(path,
				parameter.NodeConfig.GetStringSlice("globalSnapshot.spentAddressesPaths"),
				milestone_index.MilestoneIndex(parameter.NodeConfig.GetInt("globalSnapshot.index")))
		}
	} else if strings.EqualFold(snapshotToLoad, "local") {
		if path := parameter.NodeConfig.GetString("localSnapshots.path"); path != "" {
			err = LoadSnapshotFromFile(path)
		}
	}

	if err != nil {
		tangle.MarkDatabaseCorrupted()
		log.Panic(err.Error())
	}
}

func installGenesisTransaction() {
	// ensure genesis transaction exists
	genesisTxTrits := make(trinary.Trits, consts.TransactionTrinarySize)
	genesis, _ := transaction.ParseTransaction(genesisTxTrits, true)
	genesis.Hash = consts.NullHashTrytes
	txBytesTruncated := compressed.TruncateTx(trinary.MustTritsToBytes(genesisTxTrits))
	genesisTx := hornet.NewTransaction(genesis, txBytesTruncated)

	// ensure the bundle is also existent for the genesis tx
	cachedTx, _ := tangle.AddTransactionToStorage(genesisTx, 0, false)
	cachedTx.Release() // tx -1
}
