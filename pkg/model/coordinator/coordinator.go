package coordinator

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/model/tipselection"
)

// Bundle represents grouped together transactions forming a transfer.
type Bundle = []*transaction.Transaction

// SendBundleFunc is a function which sends a bundle to the network.
type SendBundleFunc = func(b Bundle) error

var (
	// ErrNetworkBootstrapped is returned when the flag for bootstrap network was given, but a state file already exists.
	ErrNetworkBootstrapped = errors.New("network already bootstrapped")
)

// coordinatorEvents are the events issued by the coordinator.
type coordinatorEvents struct {
	// Fired when a checkpoint transaction is issued.
	IssuedCheckpoint *events.Event
	// Fired when a milestone is issued.
	IssuedMilestone *events.Event
}

// Coordinator is used to issue signed transactions, called "milestones" to secure an IOTA network and prevent double spends.
type Coordinator struct {
	milestoneLock syncutils.Mutex

	// config options
	seed                   trinary.Hash
	securityLvl            int
	merkleTreeDepth        int
	minWeightMagnitude     int
	stateFilePath          string
	milestoneIntervalSec   int
	checkpointTransactions int
	powFunc                pow.ProofOfWorkFunc
	tipselFunc             tipselection.TipSelectionFunc
	sendBundleFunc         SendBundleFunc

	// internal state
	state               *State
	merkleTree          *MerkleTree
	lastCheckpointCount int
	lastCheckpointHash  *hornet.Hash
	bootstrapped        bool

	// events of the coordinator
	Events *coordinatorEvents
}

// New creates a new coordinator instance.
func New(seed trinary.Hash, securityLvl int, merkleTreeDepth int, minWeightMagnitude int, stateFilePath string, milestoneIntervalSec int, checkpointTransactions int, powFunc pow.ProofOfWorkFunc, tipselFunc tipselection.TipSelectionFunc, sendBundleFunc SendBundleFunc) *Coordinator {
	result := &Coordinator{
		seed:                   seed,
		securityLvl:            securityLvl,
		merkleTreeDepth:        merkleTreeDepth,
		minWeightMagnitude:     minWeightMagnitude,
		stateFilePath:          stateFilePath,
		milestoneIntervalSec:   milestoneIntervalSec,
		checkpointTransactions: checkpointTransactions,
		powFunc:                powFunc,
		tipselFunc:             tipselFunc,
		sendBundleFunc:         sendBundleFunc,
		Events: &coordinatorEvents{
			IssuedCheckpoint: events.NewEvent(CheckpointCaller),
			IssuedMilestone:  events.NewEvent(MilestoneCaller),
		},
	}

	return result
}

// InitMerkleTree loads the merkle tree file and checks the coordinator address.
func (coo *Coordinator) InitMerkleTree(filePath string, cooAddress trinary.Hash) error {

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("merkle tree file not found: %v", filePath)
	}

	var err error
	coo.merkleTree, err = loadMerkleTreeFile(filePath)
	if err != nil {
		return err
	}

	if cooAddress != coo.merkleTree.Root {
		return fmt.Errorf("coordinator address does not match merkle tree root: %v != %v", cooAddress, coo.merkleTree.Root)
	}

	return nil
}

// InitState loads an existing state file or bootstraps the network.
func (coo *Coordinator) InitState(bootstrap bool, startIndex milestone.Index) error {

	_, err := os.Stat(coo.stateFilePath)
	stateFileExists := !os.IsNotExist(err)

	latestMilestoneFromDatabase := tangle.SearchLatestMilestoneIndexInStore()

	if bootstrap {
		if stateFileExists {
			return ErrNetworkBootstrapped
		}

		if startIndex == 0 {
			// start with milestone 1 at least
			startIndex = 1
		}

		if latestMilestoneFromDatabase != startIndex-1 {
			return fmt.Errorf("previous milestone does not match latest milestone in database: %d != %d", startIndex-1, latestMilestoneFromDatabase)
		}

		latestMilestoneHash := hornet.NullHashBytes
		if startIndex != 1 {
			// If we don't start a new network, the last milestone has to be referenced
			cachedBndl := tangle.GetMilestoneOrNil(latestMilestoneFromDatabase)
			if cachedBndl == nil {
				return fmt.Errorf("latest milestone (%d) not found in database. database is corrupt", latestMilestoneFromDatabase)
			}
			latestMilestoneHash = cachedBndl.GetBundle().GetTailHash()
			cachedBndl.Release()
		}

		// create a new coordinator state to bootstrap the network
		state := &State{}
		state.LatestMilestoneHash = latestMilestoneHash
		state.LatestMilestoneIndex = startIndex
		state.LatestMilestoneTime = 0
		state.LatestMilestoneTransactions = hornet.Hashes{hornet.NullHashBytes}

		coo.state = state
		coo.lastCheckpointHash = &(coo.state.LatestMilestoneHash)
		coo.bootstrapped = false
		return nil
	}

	if !stateFileExists {
		return fmt.Errorf("state file not found: %v", coo.stateFilePath)
	}

	coo.state, err = loadStateFile(coo.stateFilePath)
	if err != nil {
		return err
	}

	if latestMilestoneFromDatabase != coo.state.LatestMilestoneIndex {
		return fmt.Errorf("previous milestone does not match latest milestone in database: %d != %d", coo.state.LatestMilestoneIndex, latestMilestoneFromDatabase)
	}

	cachedBndl := tangle.GetMilestoneOrNil(latestMilestoneFromDatabase)
	if cachedBndl == nil {
		return fmt.Errorf("latest milestone (%d) not found in database. database is corrupt", latestMilestoneFromDatabase)
	}
	cachedBndl.Release()

	coo.lastCheckpointHash = &(coo.state.LatestMilestoneHash)
	coo.bootstrapped = true
	return nil
}

// issueCheckpoint sends a secret checkpoint transaction to the network.
// we do that to prevent parasite chain attacks.
// only honest tipselection will reference our checkpoints, so the milestone will reference honest tips.
func (coo *Coordinator) issueCheckpoint() error {

	tips, _, err := coo.tipselFunc(0, coo.lastCheckpointHash)
	if err != nil {
		return err
	}

	b, err := createCheckpoint(tips[0].Trytes(), tips[1].Trytes(), coo.minWeightMagnitude, coo.powFunc)
	if err != nil {
		return err
	}

	if err := coo.sendBundleFunc(b); err != nil {
		return err
	}

	coo.lastCheckpointCount++
	lastCheckpointHash := hornet.Hash(trinary.MustTrytesToBytes(b[0].Hash)[:49])
	coo.lastCheckpointHash = &lastCheckpointHash

	coo.Events.IssuedCheckpoint.Trigger(coo.lastCheckpointCount, coo.checkpointTransactions, b[0].Hash)

	return nil
}

// createAndSendMilestone creates a milestone, sends it to the network and stores a new coordinator state file.
func (coo *Coordinator) createAndSendMilestone(trunkHash trinary.Hash, branchHash trinary.Hash, newMilestoneIndex milestone.Index) error {

	b, err := createMilestone(coo.seed, newMilestoneIndex, coo.securityLvl, trunkHash, branchHash, coo.minWeightMagnitude, coo.merkleTree, coo.powFunc)
	if err != nil {
		return err
	}

	if err := coo.sendBundleFunc(b); err != nil {
		return err
	}

	txHashes := hornet.Hashes{}
	for _, tx := range b {
		txHashes = append(txHashes, hornet.Hash(trinary.MustTrytesToBytes(tx.Hash)[:49]))
	}

	tailTx := b[0]

	// reset checkpoint count
	coo.lastCheckpointCount = 0

	// always reference the last milestone directly to speed up syncing (or indirectly via checkpoints)
	latestMilestoneHash := hornet.Hash(trinary.MustTrytesToBytes(tailTx.Hash)[:49])
	coo.lastCheckpointHash = &latestMilestoneHash

	coo.state.LatestMilestoneHash = latestMilestoneHash
	coo.state.LatestMilestoneIndex = newMilestoneIndex
	coo.state.LatestMilestoneTime = int64(tailTx.Timestamp)
	coo.state.LatestMilestoneTransactions = txHashes

	if err := coo.state.storeStateFile(coo.stateFilePath); err != nil {
		return err
	}

	coo.Events.IssuedMilestone.Trigger(coo.state.LatestMilestoneIndex, coo.state.LatestMilestoneHash)

	return nil
}

// IssueNextCheckpointOrMilestone creates the next checkpoint or milestone.
// if the network was not bootstrapped yet, it creates the first milestone.
// Returns non-critical and critical errors.
func (coo *Coordinator) IssueNextCheckpointOrMilestone() (error, error) {

	coo.milestoneLock.Lock()
	defer coo.milestoneLock.Unlock()

	if !coo.bootstrapped {
		// create first milestone to bootstrap the network
		if err := coo.createAndSendMilestone(consts.NullHashTrytes, consts.NullHashTrytes, coo.state.LatestMilestoneIndex); err != nil {
			// creating milestone failed => critical error
			return nil, err
		}
		coo.bootstrapped = true
		return nil, nil
	}

	if coo.lastCheckpointCount < coo.checkpointTransactions {
		// issue a checkpoint
		if err := coo.issueCheckpoint(); err != nil {
			// issuing checkpoint failed => not critical
			return err, nil
		}
		return nil, nil
	}

	// issue new milestone
	tips, _, err := coo.tipselFunc(0, coo.lastCheckpointHash)
	if err != nil {
		// tipselection failed => not critical
		return err, nil
	}

	if err := coo.createAndSendMilestone(tips[0].Trytes(), tips[1].Trytes(), coo.state.LatestMilestoneIndex+1); err != nil {
		// creating milestone failed => critical error
		return nil, err
	}

	return nil, nil
}

// GetInterval returns the interval milestones or checkpoints should be issued.
func (coo *Coordinator) GetInterval() time.Duration {
	return time.Second * time.Duration(coo.milestoneIntervalSec) / time.Duration(coo.checkpointTransactions+1)
}
