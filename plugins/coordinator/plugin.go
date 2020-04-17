package coordinator

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/metrics"
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

	// config options
	seed                   trinary.Hash
	securityLvl            int
	merkleTreeDepth        int
	mwm                    int
	stateFilePath          string
	merkleTreeFilePath     string
	milestoneIntervalSec   int
	checkpointTransactions int

	coordinatorState      *CoordinatorState
	coordinatorMerkleTree *MerkleTree
	milestoneLock         = syncutils.Mutex{}
	lastCheckpointCount   int
	lastCheckpointHash    *trinary.Hash

	_, powFunc = pow.GetFastestProofOfWorkImpl()

	bootstrap    = pflag.Bool("cooBootstrap", false, "bootstrap the network")
	startIndex   = pflag.Uint32("cooStartIndex", 0, "first index of network")
	bootstrapped = false

	ErrNoMerkleTree = errors.New("no merkle tree file found")
)

func LoadSeedFromEnvironment() (trinary.Hash, error) {
	viper.BindEnv("COO_SEED")
	seed := viper.GetString("COO_SEED")

	if len(seed) == 0 {
		return "", errors.New("Environment variable COO_SEED not set!")
	}

	if !guards.IsTransactionHash(seed) {
		return "", errors.New("Invalid coordinator seed. Check environment variable COO_SEED.")
	}

	return seed, nil
}

func InitLogger(pluginName string) {
	log = logger.NewLogger(pluginName)
}

func configure(plugin *node.Plugin) {

	InitLogger(plugin.Name)

	tanglePlugin.SetUpdateSyncedAtStartup(true)

	var err error
	seed, err = LoadSeedFromEnvironment()
	if err != nil {
		log.Panic(err)
	}

	cooAddress := config.NodeConfig.GetString(config.CfgCoordinatorAddress)
	securityLvl = config.NodeConfig.GetInt(config.CfgCoordinatorSecurityLevel)
	merkleTreeDepth = config.NodeConfig.GetInt(config.CfgCoordinatorMerkleTreeDepth)
	mwm = config.NodeConfig.GetInt(config.CfgCoordinatorMWM)
	stateFilePath = config.NodeConfig.GetString(config.CfgCoordinatorStateFilePath)
	merkleTreeFilePath = config.NodeConfig.GetString(config.CfgCoordinatorMerkleTreeFilePath)
	milestoneIntervalSec = config.NodeConfig.GetInt(config.CfgCoordinatorIntervalSeconds)
	checkpointTransactions = config.NodeConfig.GetInt(config.CfgCoordinatorCheckpointTransactions)

	if _, err = os.Stat(merkleTreeFilePath); os.IsNotExist(err) {
		log.Panicf("COO merkle tree file not found. %v", merkleTreeFilePath)
	}

	coordinatorMerkleTree, err = LoadMerkleTreeFile(merkleTreeFilePath)
	if err != nil {
		log.Panic(err)
	}

	if cooAddress != coordinatorMerkleTree.Root {
		log.Panicf("COO address does not match the merkle tree: %v != %v", cooAddress, coordinatorMerkleTree.Root)
	}

	_, err = os.Stat(stateFilePath)
	stateFileExists := !os.IsNotExist(err)

	if *bootstrap {
		if stateFileExists {
			log.Panic("COO state file already exists!")
		}

		coordinatorState = &CoordinatorState{}
		coordinatorState.latestMilestoneHash = consts.NullHashTrytes
		coordinatorState.latestMilestoneIndex = milestone.Index(*startIndex)
		coordinatorState.latestMilestoneTime = 0
		coordinatorState.latestMilestoneTransactions = []trinary.Hash{consts.NullHashTrytes}
		bootstrapped = false
	} else {
		if !stateFileExists {
			log.Panic("COO state file not found!")
		}

		coordinatorState, err = loadStateFile(stateFilePath)
		if err != nil {
			log.Panic(err)
		}
		bootstrapped = true
	}
}

func run(plugin *node.Plugin) {

	// create a background worker that issues milestones
	daemon.BackgroundWorker("Coordinator", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(issueNextMilestone, time.Second*time.Duration(milestoneIntervalSec)/time.Duration(checkpointTransactions), shutdownSignal)
	}, shutdown.PriorityCoordinator)

}

func issueNextMilestone() {

	milestoneLock.Lock()
	defer milestoneLock.Unlock()

	if !bootstrapped {
		// create first milestone to bootstrap the network
		if err := createAndSendMilestone(consts.NullHashTrytes, consts.NullHashTrytes, coordinatorState.latestMilestoneIndex, coordinatorMerkleTree); err != nil {
			log.Warn(err)
			return
		}
		bootstrapped = true
		return
	}

	if lastCheckpointCount < (checkpointTransactions) {
		// issue a checkpoint
		checkpointHash, err := issueCheckpoint(lastCheckpointHash)
		if err != nil {
			log.Warn(err)
			return
		}

		lastCheckpointCount++
		log.Infof("Issued checkpoint (%d): %v", lastCheckpointCount, checkpointHash)
		lastCheckpointHash = &checkpointHash
		return
	}

	// issue new milestone
	tips, _, err := tipselection.SelectTips(0, lastCheckpointHash)
	if err != nil {
		log.Warn(err)
		return
	}

	if err = createAndSendMilestone(tips[0], tips[1], coordinatorState.latestMilestoneIndex+1, coordinatorMerkleTree); err != nil {
		log.Warn(err)
		return
	}

	lastCheckpointCount = 0
}

func issueCheckpoint(lastCheckpointHash *trinary.Hash) (trinary.Hash, error) {

	tips, _, err := tipselection.SelectTips(0, lastCheckpointHash)
	if err != nil {
		return "", err
	}

	tx := &transaction.Transaction{}
	tx.SignatureMessageFragment = consts.NullSignatureMessageFragmentTrytes
	tx.Address = consts.NullHashTrytes
	tx.Value = 0
	tx.ObsoleteTag = consts.NullTagTrytes
	tx.Timestamp = uint64(time.Now().Unix())
	tx.CurrentIndex = 0
	tx.LastIndex = 0
	tx.Bundle = consts.NullHashTrytes
	tx.TrunkTransaction = tips[0]
	tx.BranchTransaction = tips[1]
	tx.Tag = consts.NullTagTrytes
	tx.AttachmentTimestamp = 0
	tx.AttachmentTimestampLowerBound = consts.LowerBoundAttachmentTimestamp
	tx.AttachmentTimestampUpperBound = consts.UpperBoundAttachmentTimestamp
	tx.Nonce = consts.NullTagTrytes

	b := Bundle{tx}

	// finalize bundle by adding the bundle hash
	b, err = finalizeInsecure(b)
	if err != nil {
		return "", fmt.Errorf("Bundle.Finalize: %v", err.Error())
	}

	if err = doPow(tx, mwm); err != nil {
		return "", fmt.Errorf("doPow: %v", err.Error())
	}

	for _, tx := range b {
		txTrits, _ := transaction.TransactionToTrits(tx)
		if err := gossip.Processor().CompressAndEmit(tx, txTrits); err != nil {
			return "", err
		}
		metrics.SharedServerMetrics.SentSpamTransactions.Inc()
	}

	return tx.Hash, nil
}

func createAndSendMilestone(trunkHash trinary.Hash, branchHash trinary.Hash, newMilestoneIndex milestone.Index, merkleTree *MerkleTree) error {

	b, err := createMilestone(trunkHash, branchHash, newMilestoneIndex, coordinatorMerkleTree)
	if err != nil {
		return err
	}

	txHashes := []trinary.Hash{}
	for _, tx := range b {
		txTrits, err := transaction.TransactionToTrits(tx)
		if err != nil {
			log.Panic(err)
		}

		if err := gossip.Processor().CompressAndEmit(tx, txTrits); err != nil {
			log.Panic(err)
		}
		txHashes = append(txHashes, tx.Hash)
	}

	tailTx := b[0]
	coordinatorState.latestMilestoneHash = tailTx.Hash
	coordinatorState.latestMilestoneIndex = newMilestoneIndex
	coordinatorState.latestMilestoneTime = int64(tailTx.Timestamp)
	coordinatorState.latestMilestoneTransactions = txHashes

	if err := coordinatorState.storeStateFile(stateFilePath); err != nil {
		log.Panic(err)
	}

	log.Infof("Milestone created (%d): %v", coordinatorState.latestMilestoneIndex, coordinatorState.latestMilestoneHash)
	return nil
}
