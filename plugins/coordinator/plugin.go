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

	coo      *coordinator.Coordinator
	selector *mselection.HeaviestSelector
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	// set the node as synced at startup, so the coo plugin can select tips
	tanglePlugin.SetUpdateSyncedAtStartup(true)

	// use the heaviest pair tip selection for the milestones
	selector = mselection.HPS(hornet.NullHashBytes)

	var err error
	coo, err = initCoordinator(*bootstrap, *startIndex)
	if err != nil {
		log.Panic(err)
	}

	coo.Events.IssuedCheckpoint.Attach(events.NewClosure(func(index int, lastIndex int, txHash hornet.Hash) {
		log.Infof("checkpoint issued (%d/%d): %v", index, lastIndex, txHash.Trytes())
		selector.SetRoot(txHash)
	}))

	coo.Events.IssuedMilestone.Attach(events.NewClosure(func(index milestone.Index, tailTxHash hornet.Hash) {
		log.Infof("milestone issued (%d): %v", index, tailTxHash.Trytes())
		selector.SetRoot(tailTxHash)
	}))
}

func initCoordinator(bootstrap bool, startIndex uint32) (*coordinator.Coordinator, error) {

	seed, err := config.LoadHashFromEnvironment("COO_SEED")
	if err != nil {
		return nil, err
	}

	_, powFunc := pow.GetFastestProofOfWorkImpl()

	coo := coordinator.New(
		seed,
		consts.SecurityLevel(config.NodeConfig.GetInt(config.CfgCoordinatorSecurityLevel)),
		config.NodeConfig.GetInt(config.CfgCoordinatorMerkleTreeDepth),
		config.NodeConfig.GetInt(config.CfgCoordinatorMWM),
		config.NodeConfig.GetString(config.CfgCoordinatorStateFilePath),
		config.NodeConfig.GetInt(config.CfgCoordinatorIntervalSeconds),
		config.NodeConfig.GetInt(config.CfgCoordinatorCheckpointTransactions),
		powFunc,
		selector.SelectTipsWithReference,
		sendBundle,
		coordinator.MilestoneMerkleTreeHashFuncWithName(config.NodeConfig.GetString(config.CfgCoordinatorMilestoneMerkleTreeHashFunc)),
	)

	if err := coo.InitMerkleTree(config.NodeConfig.GetString(config.CfgCoordinatorMerkleTreeFilePath), config.NodeConfig.GetString(config.CfgCoordinatorAddress)); err != nil {
		return nil, err
	}

	if err := coo.InitState(bootstrap, milestone.Index(startIndex)); err != nil {
		return nil, err
	}

	// initialize the selector
	selector.SetRoot(coo.State().LatestMilestoneHash)

	return coo, nil
}

func run(plugin *node.Plugin) {
	// pass all new transactions to the selector
	notifyNewTx := events.NewClosure(func(cachedBundle *tangle.CachedBundle) {
		// TODO: use a queue for this? Especially during SelectTips this should be stopped.
		selector.OnNewSolidBundle(cachedBundle)
	})

	// create a background worker that issues milestones
	daemon.BackgroundWorker("Coordinator", func(shutdownSignal <-chan struct{}) {
		tanglePlugin.Events.BundleSolid.Attach(notifyNewTx)
		defer tanglePlugin.Events.BundleSolid.Detach(notifyNewTx)

		// TODO: add some random jitter to make the ms intervals not predictable
		timeutil.Ticker(func() {
			err, criticalErr := coo.IssueNextCheckpointOrMilestone()
			if criticalErr != nil {
				log.Panic(criticalErr)
			}
			if err != nil {
				log.Warn(err)
			}
		}, coo.GetInterval(), shutdownSignal)
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
