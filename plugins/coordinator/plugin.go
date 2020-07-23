package coordinator

import (
	"sync"

	"github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/transaction"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/mselection"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/gossip"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

func init() {
	pflag.CommandLine.MarkHidden("cooBootstrap")
	pflag.CommandLine.MarkHidden("cooStartIndex")
}

var (
	PLUGIN = node.NewPlugin("Coordinator", node.Disabled, configure, run)
	log    *logger.Logger

	bootstrap  = pflag.Bool("cooBootstrap", false, "bootstrap the network")
	startIndex = pflag.Uint32("cooStartIndex", 0, "index of the first milestone at bootstrap")

	maxTipsCount      int
	maxApproveesCount int

	nextCheckpointSignal chan struct{}
	nextMilestoneSignal  chan struct{}

	coo      *coordinator.Coordinator
	selector *mselection.HeaviestSelector
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	// set the node as synced at startup, so the coo plugin can select tips
	tanglePlugin.SetUpdateSyncedAtStartup(true)

	var err error
	coo, err = initCoordinator(*bootstrap, *startIndex)
	if err != nil {
		log.Panic(err)
	}

	coo.Events.IssuedCheckpointTransaction.Attach(events.NewClosure(func(checkpointIndex int, tipIndex int, tipsTotal int, txHash hornet.Hash) {
		log.Infof("checkpoint (%d) transaction issued (%d/%d): %v", checkpointIndex+1, tipIndex+1, tipsTotal, txHash.Trytes())
	}))

	coo.Events.IssuedMilestone.Attach(events.NewClosure(func(index milestone.Index, tailTxHash hornet.Hash) {
		log.Infof("milestone issued (%d): %v", index, tailTxHash.Trytes())
	}))
}

func initCoordinator(bootstrap bool, startIndex uint32) (*coordinator.Coordinator, error) {

	seed, err := config.LoadHashFromEnvironment("COO_SEED")
	if err != nil {
		return nil, err
	}

	// use the heaviest branch tip selection for the milestones
	selector = mselection.New(
		config.NodeConfig.GetInt(config.CfgCoordinatorTipselectMinHeaviestBranchUnconfirmedTransactionsThreshold),
		config.NodeConfig.GetInt(config.CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint),
		config.NodeConfig.GetInt(config.CfgCoordinatorTipselectRandomTipsPerCheckpoint),
	)

	_, powFunc := pow.GetFastestProofOfWorkImpl()

	nextCheckpointSignal = make(chan struct{})
	nextMilestoneSignal = make(chan struct{})

	maxTipsCount = config.NodeConfig.GetInt(config.CfgCoordinatorCheckpointsMaxTipsCount)
	maxApproveesCount = config.NodeConfig.GetInt(config.CfgCoordinatorCheckpointsMaxApproveesCount)

	coo := coordinator.New(
		seed,
		consts.SecurityLevel(config.NodeConfig.GetInt(config.CfgCoordinatorSecurityLevel)),
		config.NodeConfig.GetInt(config.CfgCoordinatorMerkleTreeDepth),
		config.NodeConfig.GetInt(config.CfgCoordinatorMWM),
		config.NodeConfig.GetString(config.CfgCoordinatorStateFilePath),
		config.NodeConfig.GetInt(config.CfgCoordinatorIntervalSeconds),
		powFunc,
		selector.SelectTips,
		sendBundle,
		coordinator.MilestoneMerkleTreeHashFuncWithName(config.NodeConfig.GetString(config.CfgCoordinatorMilestoneMerkleTreeHashFunc)),
	)

	if err := coo.InitMerkleTree(config.NodeConfig.GetString(config.CfgCoordinatorMerkleTreeFilePath), config.NodeConfig.GetString(config.CfgCoordinatorAddress)); err != nil {
		return nil, err
	}

	if err := coo.InitState(bootstrap, milestone.Index(startIndex)); err != nil {
		return nil, err
	}

	return coo, nil
}

func run(plugin *node.Plugin) {
	// pass all new solid bundles to the selector
	onBundleSolid := events.NewClosure(func(cachedBundle *tangle.CachedBundle) {
		cachedBundle.ConsumeBundle(func(bndl *tangle.Bundle) { // bundle -1

			if bndl.IsInvalidPastCone() || !bndl.IsValid() || !bndl.ValidStrictSemantics() {
				// ignore invalid bundles or semantically invalid bundles or bundles with invalid past cone
				return
			}

			if tipCount, approveeCount := selector.OnNewSolidBundle(bndl); (tipCount >= maxTipsCount) || (approveeCount >= maxApproveesCount) {
				// issue next checkpoint
				select {
				case nextCheckpointSignal <- struct{}{}:
				default:
					// do not block if already another signal is waiting
				}
			}
		})
	})

	// create a background worker that signals to issue new milestones
	daemon.BackgroundWorker("Coordinator[MilestoneTicker]", func(shutdownSignal <-chan struct{}) {

		timeutil.Ticker(func() {
			// issue next milestone
			select {
			case nextMilestoneSignal <- struct{}{}:
			default:
				// do not block if already another signal is waiting
			}
		}, coo.GetInterval(), shutdownSignal)

	}, shutdown.PriorityCoordinator)

	// create a background worker that issues milestones
	daemon.BackgroundWorker("Coordinator", func(shutdownSignal <-chan struct{}) {
		tanglePlugin.Events.BundleSolid.Attach(onBundleSolid)
		defer tanglePlugin.Events.BundleSolid.Detach(onBundleSolid)

		// bootstrap the network if not done yet
		if criticalErr := coo.Bootstrap(); criticalErr != nil {
			log.Panic(criticalErr)
		}

	coordinatorLoop:
		for {
			select {
			case <-nextCheckpointSignal:
				// check the thresholds again, because a new milestone could have been issued in the meantime
				if tipCount, approveeCount := selector.GetStats(); (tipCount < maxTipsCount) && (approveeCount < maxApproveesCount) {
					continue
				}

				// issue a checkpoint
				if err := coo.IssueCheckpoint(); err != nil {
					// issuing checkpoint failed => not critical
					if err != mselection.ErrNoTipsAvailable {
						log.Warn(err)
					}
				}

			case <-nextMilestoneSignal:
				err, criticalErr := coo.IssueMilestone()
				if criticalErr != nil {
					log.Panic(criticalErr)
				}
				if err != nil {
					log.Warn(err)
				}

			case <-shutdownSignal:
				break coordinatorLoop
			}
		}
	}, shutdown.PriorityCoordinator)
}

func sendBundle(b coordinator.Bundle) error {

	// collect all tx hashes of the bundle
	txHashes := make(map[string]struct{})
	for _, t := range b {
		txHashes[string(hornet.HashFromHashTrytes(t.Hash))] = struct{}{}
	}

	txHashesLock := syncutils.Mutex{}

	// wgBundleProcessed waits until all txs of the bundle were processed in the storage layer
	wgBundleProcessed := sync.WaitGroup{}
	wgBundleProcessed.Add(len(txHashes))

	processedTxEvent := events.NewClosure(func(txHash hornet.Hash) {
		txHashesLock.Lock()
		defer txHashesLock.Unlock()

		if _, exists := txHashes[string(txHash)]; exists {
			// tx of bundle was processed
			wgBundleProcessed.Done()
			delete(txHashes, string(txHash))
		}
	})

	tanglePlugin.Events.ProcessedTransaction.Attach(processedTxEvent)
	defer tanglePlugin.Events.ProcessedTransaction.Detach(processedTxEvent)

	for _, t := range b {
		tx := t // assign to new variable, otherwise it would be overwritten by the loop before processed
		txTrits, _ := transaction.TransactionToTrits(tx)
		if err := gossip.Processor().CompressAndEmit(tx, txTrits); err != nil {
			return err
		}
	}

	// wait until all txs of the bundle are processed in the storage layer
	wgBundleProcessed.Wait()

	return nil
}

// GetEvents returns the events of the coordinator
func GetEvents() *coordinator.CoordinatorEvents {
	if coo == nil {
		return nil
	}
	return coo.Events
}
