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
	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/gossip"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
	"github.com/gohornet/hornet/plugins/tipselection"
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

	coo *coordinator.Coordinator
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

	coo.Events.IssuedCheckpoint.Attach(events.NewClosure(func(index int, lastIndex int, txHash trinary.Hash) {
		log.Infof("checkpoint issued (%d/%d): %v", index, lastIndex, txHash)
	}))

	coo.Events.IssuedMilestone.Attach(events.NewClosure(func(index milestone.Index, tailTxHash trinary.Hash) {
		log.Infof("milestone issued (%d): %v", index, tailTxHash)
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
		config.NodeConfig.GetInt(config.CfgCoordinatorSecurityLevel),
		config.NodeConfig.GetInt(config.CfgCoordinatorMerkleTreeDepth),
		config.NodeConfig.GetInt(config.CfgCoordinatorMWM),
		config.NodeConfig.GetString(config.CfgCoordinatorStateFilePath),
		config.NodeConfig.GetInt(config.CfgCoordinatorIntervalSeconds),
		config.NodeConfig.GetInt(config.CfgCoordinatorCheckpointTransactions),
		powFunc,
		tipselection.SelectTips,
		sendBundle,
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
	// create a background worker that issues milestones
	daemon.BackgroundWorker("Coordinator", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(func() {
			err, criticalErr := coo.IssueNextCheckpointOrMilestone()
			if criticalErr != nil {
				log.Panic(err)
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
		txHashes[string(hornet.Hash(trinary.MustTrytesToBytes(t.Hash)[:49]))] = struct{}{}
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
