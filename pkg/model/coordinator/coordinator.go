package coordinator

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/pkg/errors"

	_ "golang.org/x/crypto/blake2b" // import implementation

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

// BackPressureFunc is a function which tells the Coordinator
// to stop issuing milestones and checkpoints under high load.
type BackPressureFunc func() bool

// SendMessageFunc is a function which sends a message to the network.
type SendMessageFunc = func(msg *storage.Message, msIndex ...milestone.Index) error

var (
	// ErrNoTipsGiven is returned when no tips were given to issue a checkpoint.
	ErrNoTipsGiven = errors.New("no tips given")
	// ErrNetworkBootstrapped is returned when the flag for bootstrap network was given, but a state file already exists.
	ErrNetworkBootstrapped = errors.New("network already bootstrapped")
	// ErrInvalidSiblingsTrytesLength is returned when the computed siblings trytes do not fit into the signature message fragment.
	ErrInvalidSiblingsTrytesLength = errors.New("siblings trytes too long")
)

// Events are the events issued by the coordinator.
type Events struct {
	// Fired when a checkpoint message is issued.
	IssuedCheckpointMessage *events.Event
	// Fired when a milestone is issued.
	IssuedMilestone *events.Event
}

// PublicKeyRange is a public key of milestones with a valid range.
type PublicKeyRange struct {
	Key        string          `json:"key" koanf:"key"`
	StartIndex milestone.Index `json:"start" koanf:"start"`
	EndIndex   milestone.Index `json:"end" koanf:"end"`
}

// PublicKeyRanges are public keys of milestones with their valid ranges.
type PublicKeyRanges []*PublicKeyRange

// Coordinator is used to issue signed messages, called "milestones" to secure an IOTA network and prevent double spends.
type Coordinator struct {
	milestoneLock syncutils.Mutex

	storage         *storage.Storage
	migratorService *migrator.MigratorService
	utxoManager     *utxo.Manager
	signerProvider  MilestoneSignerProvider

	// config options
	stateFilePath            string
	milestoneIntervalSec     int
	powParallelism           int
	milestonePublicKeysCount int
	networkID                uint64
	powHandler               *pow.Handler
	sendMesssageFunc         SendMessageFunc
	backpressureFuncs        []BackPressureFunc

	// internal state
	state        *State
	bootstrapped bool

	// events of the coordinator
	Events *Events
}

// New creates a new coordinator instance.
func New(
	storage *storage.Storage, networkID uint64, signerProvider MilestoneSignerProvider,
	stateFilePath string, milestoneIntervalSec int, powParallelism int,
	powHandler *pow.Handler, migratorService *migrator.MigratorService, utxoManager *utxo.Manager,
	sendMessageFunc SendMessageFunc) (*Coordinator, error) {

	result := &Coordinator{
		storage:              storage,
		networkID:            networkID,
		signerProvider:       signerProvider,
		stateFilePath:        stateFilePath,
		milestoneIntervalSec: milestoneIntervalSec,
		powParallelism:       powParallelism,
		powHandler:           powHandler,
		sendMesssageFunc:     sendMessageFunc,
		migratorService:      migratorService,
		utxoManager:          utxoManager,
		Events: &Events{
			IssuedCheckpointMessage: events.NewEvent(CheckpointCaller),
			IssuedMilestone:         events.NewEvent(MilestoneCaller),
		},
	}

	return result, nil
}

// InitState loads an existing state file or bootstraps the network.
func (coo *Coordinator) InitState(bootstrap bool, startIndex milestone.Index) error {

	_, err := os.Stat(coo.stateFilePath)
	stateFileExists := !os.IsNotExist(err)

	latestMilestoneFromDatabase := coo.storage.SearchLatestMilestoneIndexInStore()

	if bootstrap {
		if stateFileExists {
			return ErrNetworkBootstrapped
		}

		if startIndex == 0 {
			// start with milestone 1 at least
			startIndex = 1
		}

		if latestMilestoneFromDatabase != startIndex-1 {
			return fmt.Errorf("previous milestone does not match latest milestone in database! previous: %d, database: %d", startIndex-1, latestMilestoneFromDatabase)
		}

		latestMilestoneMessageID := hornet.GetNullMessageID()
		if startIndex != 1 {
			// If we don't start a new network, the last milestone has to be referenced
			cachedMilestoneMsg := coo.storage.GetMilestoneCachedMessageOrNil(latestMilestoneFromDatabase)
			if cachedMilestoneMsg == nil {
				return fmt.Errorf("latest milestone (%d) not found in database. database is corrupt", latestMilestoneFromDatabase)
			}
			latestMilestoneMessageID = cachedMilestoneMsg.GetMessage().GetMessageID()
			cachedMilestoneMsg.Release()
		}

		// create a new coordinator state to bootstrap the network
		state := &State{}
		state.LatestMilestoneMessageID = latestMilestoneMessageID
		state.LatestMilestoneIndex = startIndex - 1
		state.LatestMilestoneTime = time.Now()

		coo.state = state
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
		return fmt.Errorf("previous milestone does not match latest milestone in database. previous: %d, database: %d", coo.state.LatestMilestoneIndex, latestMilestoneFromDatabase)
	}

	cachedMilestoneMsg := coo.storage.GetMilestoneCachedMessageOrNil(latestMilestoneFromDatabase)
	if cachedMilestoneMsg == nil {
		return fmt.Errorf("latest milestone (%d) not found in database. database is corrupt", latestMilestoneFromDatabase)
	}
	cachedMilestoneMsg.Release()

	coo.bootstrapped = true
	return nil
}

// createAndSendMilestone creates a milestone, sends it to the network and stores a new coordinator state file.
func (coo *Coordinator) createAndSendMilestone(parents hornet.MessageIDs, newMilestoneIndex milestone.Index) error {

	cachedMsgMetas := make(map[string]*storage.CachedMetadata)
	cachedMessages := make(map[string]*storage.CachedMessage)

	defer func() {
		// All releases are forced since the cone is referenced and not needed anymore

		// release all messages at the end
		for _, cachedMessage := range cachedMessages {
			cachedMessage.Release(true) // message -1
		}

		// Release all message metadata at the end
		for _, cachedMsgMeta := range cachedMsgMetas {
			cachedMsgMeta.Release(true) // meta -1
		}
	}()

	parents = parents.RemoveDupsAndSortByLexicalOrder()

	// compute merkle tree root
	mutations, err := whiteflag.ComputeWhiteFlagMutations(coo.storage, newMilestoneIndex, cachedMsgMetas, cachedMessages, parents)
	if err != nil {
		return fmt.Errorf("failed to compute white flag mutations: %w", err)
	}

	// get receipt data in case migrator is enabled
	var receipt *iotago.Receipt
	if coo.migratorService != nil {
		receipt = coo.migratorService.Receipt()
		if receipt != nil {
			currentTreasuryOutput, err := coo.utxoManager.UnspentTreasuryOutput()
			if err != nil {
				return fmt.Errorf("unable to fetch unspent treasury output: %w", err)
			}

			// embed treasury within the receipt
			input := &iotago.TreasuryInput{}
			copy(input[:], currentTreasuryOutput.MilestoneID[:])
			output := &iotago.TreasuryOutput{Amount: currentTreasuryOutput.Amount - receipt.Sum()}
			treasuryTx := &iotago.TreasuryTransaction{Input: input, Output: output}
			receipt.Transaction = treasuryTx
		}
	}

	milestoneMsg, err := createMilestone(newMilestoneIndex, coo.networkID, parents, coo.signerProvider, receipt, mutations.MerkleTreeHash, coo.powParallelism, coo.powHandler)
	if err != nil {
		return fmt.Errorf("failed to create milestone: %w", err)
	}

	if err := coo.sendMesssageFunc(milestoneMsg, newMilestoneIndex); err != nil {
		return err
	}

	// always reference the last milestone directly to speed up syncing
	latestMilestoneMessageID := milestoneMsg.GetMessageID()

	coo.state.LatestMilestoneMessageID = latestMilestoneMessageID
	coo.state.LatestMilestoneIndex = newMilestoneIndex
	coo.state.LatestMilestoneTime = time.Now()

	if err := coo.state.storeStateFile(coo.stateFilePath); err != nil {
		return fmt.Errorf("failed to update state file: %w", err)
	}

	coo.Events.IssuedMilestone.Trigger(coo.state.LatestMilestoneIndex, coo.state.LatestMilestoneMessageID)

	return nil
}

// Bootstrap creates the first milestone, if the network was not bootstrapped yet.
// Returns critical errors.
func (coo *Coordinator) Bootstrap() (hornet.MessageID, error) {

	coo.milestoneLock.Lock()
	defer coo.milestoneLock.Unlock()

	if !coo.bootstrapped {
		// create first milestone to bootstrap the network
		// only one parent references the last known milestone or NullMessageID if startIndex = 1 (see InitState)
		if err := coo.createAndSendMilestone(hornet.MessageIDs{coo.state.LatestMilestoneMessageID}, coo.state.LatestMilestoneIndex+1); err != nil {
			// creating milestone failed => critical error
			return nil, err
		}

		coo.bootstrapped = true
	}

	return coo.state.LatestMilestoneMessageID, nil
}

// IssueCheckpoint tries to create and send a "checkpoint" to the network.
// a checkpoint can contain multiple chained messages to reference big parts of the unreferenced cone.
// this is done to keep the confirmation rate as high as possible, even if there is an attack ongoing.
// new checkpoints always reference the last checkpoint or the last milestone if it is the first checkpoint after a new milestone.
func (coo *Coordinator) IssueCheckpoint(checkpointIndex int, lastCheckpointMessageID hornet.MessageID, tips hornet.MessageIDs) (hornet.MessageID, error) {

	if len(tips) == 0 {
		return nil, ErrNoTipsGiven
	}

	coo.milestoneLock.Lock()
	defer coo.milestoneLock.Unlock()

	if !coo.storage.IsNodeSynced() {
		return nil, common.ErrNodeNotSynced
	}

	// check whether we should hold issuing checkpoints
	// if the node is currently under a lot of load
	if coo.checkBackPressureFunctions() {
		return nil, common.ErrNodeLoadTooHigh
	}

	// maximum 8 parents per message (7 tips + last checkpoint messageID)
	checkpointsNumber := int(math.Ceil(float64(len(tips)) / 7.0))

	// issue several checkpoints until all tips are used
	for i := 0; i < checkpointsNumber; i++ {
		tipStart := i * 7
		tipEnd := tipStart + 7

		if tipEnd > len(tips) {
			tipEnd = len(tips)
		}

		parents := hornet.MessageIDs{lastCheckpointMessageID}
		parents = append(parents, tips[tipStart:tipEnd]...)
		parents = parents.RemoveDupsAndSortByLexicalOrder()

		msg, err := createCheckpoint(coo.networkID, parents, coo.powParallelism, coo.powHandler)
		if err != nil {
			return nil, fmt.Errorf("failed to create checkPoint: %w", err)
		}

		if err := coo.sendMesssageFunc(msg); err != nil {
			return nil, err
		}

		lastCheckpointMessageID = msg.GetMessageID()

		coo.Events.IssuedCheckpointMessage.Trigger(checkpointIndex, i, checkpointsNumber, lastCheckpointMessageID)
	}

	return lastCheckpointMessageID, nil
}

// IssueMilestone creates the next milestone.
// Returns non-critical and critical errors.
func (coo *Coordinator) IssueMilestone(parents hornet.MessageIDs) (hornet.MessageID, error, error) {

	coo.milestoneLock.Lock()
	defer coo.milestoneLock.Unlock()

	if !coo.storage.IsNodeSynced() {
		// return a non-critical error to not kill the database
		return nil, common.ErrNodeNotSynced, nil
	}

	// check whether we should hold issuing miletones
	// if the node is currently under a lot of load
	if coo.checkBackPressureFunctions() {
		return nil, common.ErrNodeLoadTooHigh, nil
	}

	if err := coo.createAndSendMilestone(parents, coo.state.LatestMilestoneIndex+1); err != nil {
		// creating milestone failed => critical error
		return nil, nil, err
	}

	return coo.state.LatestMilestoneMessageID, nil, nil
}

// GetInterval returns the interval milestones should be issued.
func (coo *Coordinator) GetInterval() time.Duration {
	return time.Second * time.Duration(coo.milestoneIntervalSec)
}

// State returns the current state of the coordinator.
func (coo *Coordinator) State() *State {
	return coo.state
}

// AddBackPressureFunc adds a BackPressureFunc.
// This function can be called multiple times to add additional BackPressureFunc.
func (coo *Coordinator) AddBackPressureFunc(bpFunc BackPressureFunc) {
	coo.backpressureFuncs = append(coo.backpressureFuncs, bpFunc)
}

// checkBackPressureFunctions checks whether any back pressure function is signaling congestion.
func (coo *Coordinator) checkBackPressureFunctions() bool {
	for _, f := range coo.backpressureFuncs {
		if f() {
			return true
		}
	}
	return false
}
