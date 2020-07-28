package coordinator

import (
	"crypto"
	"fmt"
	"os"
	"strings"
	"time"

	_ "golang.org/x/crypto/blake2b" // import implementation

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/merkle"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

// Bundle represents grouped together transactions forming a transfer.
type Bundle = []*transaction.Transaction

// SendBundleFunc is a function which sends a bundle to the network.
type SendBundleFunc = func(b Bundle) error

// CheckpointTipSelectionFunc is a function which performs a tipselection and returns several tips for a checkpoint.
type CheckpointTipSelectionFunc = func(minRequiredTips int) (hornet.Hashes, error)

var (
	// ErrNetworkBootstrapped is returned when the flag for bootstrap network was given, but a state file already exists.
	ErrNetworkBootstrapped = errors.New("network already bootstrapped")
)

// CoordinatorEvents are the events issued by the coordinator.
type CoordinatorEvents struct {
	// Fired when a checkpoint transaction is issued.
	IssuedCheckpointTransaction *events.Event
	// Fired when a milestone is issued.
	IssuedMilestone *events.Event
}

// Coordinator is used to issue signed transactions, called "milestones" to secure an IOTA network and prevent double spends.
type Coordinator struct {
	milestoneLock syncutils.Mutex

	// config options
	seed                    trinary.Hash
	securityLvl             consts.SecurityLevel
	merkleTreeDepth         int
	minWeightMagnitude      int
	stateFilePath           string
	milestoneIntervalSec    int
	powHandler              *pow.Handler
	checkpointTipselFunc    CheckpointTipSelectionFunc
	sendBundleFunc          SendBundleFunc
	milestoneMerkleHashFunc crypto.Hash

	// internal state
	state               *State
	merkleTree          *merkle.MerkleTree
	lastCheckpointIndex int
	lastCheckpointHash  hornet.Hash
	lastMilestoneHash   hornet.Hash
	bootstrapped        bool

	// events of the coordinator
	Events *CoordinatorEvents
}

// MilestoneMerkleTreeHashFuncWithName maps the passed name to one of the supported crypto.Hash hashing functions.
// Also verifies that the available function is available or else panics.
func MilestoneMerkleTreeHashFuncWithName(name string) crypto.Hash {
	//TODO: golang 1.15 will include a String() method to get the string from the crypto.Hash, so we could iterate over them instead
	var hashFunc crypto.Hash
	switch strings.ToLower(name) {
	case "blake2b-512":
		hashFunc = crypto.BLAKE2b_512
	case "blake2b-384":
		hashFunc = crypto.BLAKE2b_384
	case "blake2b-256":
		hashFunc = crypto.BLAKE2b_256
	case "blake2s-256":
		hashFunc = crypto.BLAKE2s_256
	default:
		panic(fmt.Sprintf("Unsupported merkle tree hash func '%s'", name))
	}

	if !hashFunc.Available() {
		panic(fmt.Sprintf("Merkle tree hash func '%s' not available. Please check the package imports", name))
	}
	return hashFunc
}

// New creates a new coordinator instance.
func New(seed trinary.Hash, securityLvl consts.SecurityLevel, merkleTreeDepth int, minWeightMagnitude int, stateFilePath string, milestoneIntervalSec int, powHandler *pow.Handler, checkpointTipselFunc CheckpointTipSelectionFunc, sendBundleFunc SendBundleFunc, milestoneMerkleHashFunc crypto.Hash) *Coordinator {
	result := &Coordinator{
		seed:                    seed,
		securityLvl:             securityLvl,
		merkleTreeDepth:         merkleTreeDepth,
		minWeightMagnitude:      minWeightMagnitude,
		stateFilePath:           stateFilePath,
		milestoneIntervalSec:    milestoneIntervalSec,
		powHandler:              powHandler,
		checkpointTipselFunc:    checkpointTipselFunc,
		sendBundleFunc:          sendBundleFunc,
		milestoneMerkleHashFunc: milestoneMerkleHashFunc,
		Events: &CoordinatorEvents{
			IssuedCheckpointTransaction: events.NewEvent(CheckpointCaller),
			IssuedMilestone:             events.NewEvent(MilestoneCaller),
		},
	}

	return result
}

// InitMerkleTree loads the Merkle tree file and checks the coordinator address.
func (coo *Coordinator) InitMerkleTree(filePath string, cooAddress trinary.Hash) error {

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("Merkle tree file not found: %v", filePath)
	}

	var err error
	coo.merkleTree, err = merkle.LoadMerkleTreeFile(filePath)
	if err != nil {
		return err
	}

	if cooAddress != coo.merkleTree.Root {
		return fmt.Errorf("coordinator address does not match Merkle tree root: %v != %v", cooAddress, coo.merkleTree.Root)
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

		// if we bootstrap a network, NullHash has to be set as a solid entry point
		tangle.SolidEntryPointsAdd(hornet.NullHashBytes, startIndex)

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
		coo.lastCheckpointHash = coo.state.LatestMilestoneHash
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

	coo.lastCheckpointHash = coo.state.LatestMilestoneHash
	coo.lastMilestoneHash = coo.state.LatestMilestoneHash
	coo.bootstrapped = true
	return nil
}

// issueCheckpointWithoutLocking tries to create and send a "checkpoint" to the network.
// a checkpoint can contain multiple chained transactions to reference big parts of the unconfirmed cone.
// this is done to keep the confirmation rate as high as possible, even if there is an attack ongoing.
// new checkpoints always reference the last checkpoint or the last milestone if it is the first checkpoint after a new milestone.
func (coo *Coordinator) issueCheckpointWithoutLocking(minRequiredTips int) error {

	if !tangle.IsNodeSynced() {
		return tangle.ErrNodeNotSynced
	}

	tips, err := coo.checkpointTipselFunc(minRequiredTips)
	if err != nil {
		return err
	}

	var lastCheckpointHash hornet.Hash
	if coo.lastCheckpointIndex == 0 {
		// reference the last milestone
		lastCheckpointHash = coo.lastMilestoneHash
	} else {
		// reference the last checkpoint
		lastCheckpointHash = coo.lastCheckpointHash
	}

	for i, tip := range tips {
		b, err := createCheckpoint(tip, lastCheckpointHash, coo.minWeightMagnitude, coo.powHandler)
		if err != nil {
			return err
		}

		if err := coo.sendBundleFunc(b); err != nil {
			return err
		}

		lastCheckpointHash = hornet.HashFromHashTrytes(b[0].Hash)

		coo.Events.IssuedCheckpointTransaction.Trigger(coo.lastCheckpointIndex, i, len(tips), lastCheckpointHash)
	}

	coo.lastCheckpointIndex++
	coo.lastCheckpointHash = lastCheckpointHash

	return nil
}

// createAndSendMilestone creates a milestone, sends it to the network and stores a new coordinator state file.
func (coo *Coordinator) createAndSendMilestone(trunkHash hornet.Hash, branchHash hornet.Hash, newMilestoneIndex milestone.Index) error {

	// compute merkle tree root
	byteEncodedMerkleTreeRootHash, err := whiteflag.ComputeMerkleTreeRootHash(coo.milestoneMerkleHashFunc, trunkHash, branchHash)
	if err != nil {
		return err
	}

	b, err := createMilestone(coo.seed, newMilestoneIndex, coo.securityLvl, trunkHash, branchHash, coo.minWeightMagnitude, coo.merkleTree, byteEncodedMerkleTreeRootHash, coo.powHandler)
	if err != nil {
		return err
	}

	if err := coo.sendBundleFunc(b); err != nil {
		return err
	}

	txHashes := hornet.Hashes{}
	for _, tx := range b {
		txHashes = append(txHashes, hornet.HashFromHashTrytes(tx.Hash))
	}

	tailTx := b[0]

	// reset checkpoint count
	coo.lastCheckpointIndex = 0

	// always reference the last milestone directly to speed up syncing
	latestMilestoneHash := hornet.HashFromHashTrytes(tailTx.Hash)
	coo.lastMilestoneHash = latestMilestoneHash

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

// Bootstrap creates the first milestone, if the network was not bootstrapped yet.
// Returns critical errors.
func (coo *Coordinator) Bootstrap() error {

	coo.milestoneLock.Lock()
	defer coo.milestoneLock.Unlock()

	if !coo.bootstrapped {
		// create first milestone to bootstrap the network
		if err := coo.createAndSendMilestone(hornet.Hash(hornet.NullHashBytes), hornet.Hash(hornet.NullHashBytes), coo.state.LatestMilestoneIndex); err != nil {
			// creating milestone failed => critical error
			return err
		}
		coo.bootstrapped = true
	}

	return nil
}

// IssueCheckpoint tries to create and send a "checkpoint" to the network.
// a checkpoint can contain multiple chained transactions to reference big parts of the unconfirmed cone.
// this is done to keep the confirmation rate as high as possible, even if there is an attack ongoing.
// new checkpoints always reference the last checkpoint or the last milestone if it is the first checkpoint after a new milestone.
func (coo *Coordinator) IssueCheckpoint() error {

	coo.milestoneLock.Lock()
	defer coo.milestoneLock.Unlock()

	return coo.issueCheckpointWithoutLocking(0)
}

// IssueMilestone creates the next milestone.
// a new checkpoint is created right in front of the milestone to raise confirmation rate.
// Returns non-critical and critical errors.
func (coo *Coordinator) IssueMilestone() (error, error) {

	coo.milestoneLock.Lock()
	defer coo.milestoneLock.Unlock()

	if !tangle.IsNodeSynced() {
		// return a non-critical error to not kill the database
		return tangle.ErrNodeNotSynced, nil
	}

	lastCheckpointHash := coo.lastCheckpointHash

	// issue a new checkpoint right in front of the milestone
	if err := coo.issueCheckpointWithoutLocking(1); err != nil {
		// creating checkpoint failed => not critical
		if coo.lastCheckpointIndex == 0 {
			// no checkpoint created => use the last milestone hash
			lastCheckpointHash = coo.lastMilestoneHash
		}
	} else {
		// use the new checkpoint hash
		lastCheckpointHash = coo.lastCheckpointHash
	}

	if err := coo.createAndSendMilestone(coo.lastMilestoneHash, lastCheckpointHash, coo.state.LatestMilestoneIndex+1); err != nil {
		// creating milestone failed => critical error
		return nil, err
	}

	return nil, nil
}

// GetInterval returns the interval milestones should be issued.
func (coo *Coordinator) GetInterval() time.Duration {
	return time.Second * time.Duration(coo.milestoneIntervalSec)
}

// State returns the current state of the coordinator.
func (coo *Coordinator) State() *State {
	return coo.state
}
