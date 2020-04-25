package coordinator

import (
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/shutdown"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
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
	_, powFunc            = pow.GetFastestProofOfWorkImpl()
	bootstrapped          = false
	bootstrap             = pflag.Bool("cooBootstrap", false, "bootstrap the network")
	startIndex            = pflag.Uint32("cooStartIndex", 0, "first index of network")

	ErrMerkleTreeFileNotFound           = errors.New("merkle tree file not found")
	ErrStateFileNotFound                = errors.New("state file not found")
	ErrNetworkBootstrapped              = errors.New("network already bootstrapped")
	ErrMerkleRootDoesNotMatch           = errors.New("merkle root does not match")
	ErrCooAddressDoesNotMatchMerkleTree = errors.New("coordinator address does not match merkle tree")
	ErrEnvironmentVariableSeedNotSet    = errors.New("environment variable COO_SEED not set")
	ErrEnvironmentVariableInvalidSeed   = errors.New("invalid coordinator seed. check environment variable COO_SEED")
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	// set the node as synced at startup, so the coo plugin can select tips
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
		log.Panic(errors.Wrapf(ErrMerkleTreeFileNotFound, "%v", merkleTreeFilePath))
	}

	coordinatorMerkleTree, err = loadMerkleTreeFile(merkleTreeFilePath)
	if err != nil {
		log.Panic(err)
	}

	if cooAddress != coordinatorMerkleTree.Root {
		log.Panic(errors.Wrapf(ErrCooAddressDoesNotMatchMerkleTree, "%v != %v", cooAddress, coordinatorMerkleTree.Root))
	}

	_, err = os.Stat(stateFilePath)
	stateFileExists := !os.IsNotExist(err)

	if *bootstrap {
		if stateFileExists {
			log.Panic(ErrNetworkBootstrapped)
		}

		coordinatorState = &CoordinatorState{}
		coordinatorState.latestMilestoneHash = consts.NullHashTrytes
		coordinatorState.latestMilestoneIndex = milestone.Index(*startIndex)
		if coordinatorState.latestMilestoneIndex == 0 {
			// start with milestone 1 at least
			coordinatorState.latestMilestoneIndex = 1
		}
		coordinatorState.latestMilestoneTime = 0
		coordinatorState.latestMilestoneTransactions = []trinary.Hash{consts.NullHashTrytes}
		bootstrapped = false
	} else {
		if !stateFileExists {
			log.Panic(ErrStateFileNotFound)
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
		timeutil.Ticker(issueNextCheckpointOrMilestone, time.Second*time.Duration(milestoneIntervalSec)/time.Duration(checkpointTransactions), shutdownSignal)
	}, shutdown.PriorityCoordinator)

}
