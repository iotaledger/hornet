package coordinator

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

// SendMessageFunc is a function which sends a message to the network.
type SendMessageFunc = func(msg *storage.Message, msIndex ...milestone.Index) error

var (
	// ErrNoTipsGiven is returned when no tips were given to issue a checkpoint.
	ErrNoTipsGiven = errors.New("no tips given")
	// ErrNetworkBootstrapped is returned when the flag for bootstrap network was given, but a state file already exists.
	ErrNetworkBootstrapped = errors.New("network already bootstrapped")
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

	storage        *storage.Storage
	signerProvider MilestoneSignerProvider

	// config options
	stateFilePath            string
	milestoneIntervalSec     int
	milestonePublicKeysCount int
	networkID                uint64
	powHandler               *pow.Handler
	sendMesssageFunc         SendMessageFunc

	// internal state
	state        *State
	bootstrapped bool

	// events of the coordinator
	Events *Events
}

// New creates a new coordinator instance.
func New(storage *storage.Storage, networkID uint64, signerProvider MilestoneSignerProvider, stateFilePath string, milestoneIntervalSec int, powHandler *pow.Handler, sendMessageFunc SendMessageFunc) (*Coordinator, error) {

	result := &Coordinator{
		storage:              storage,
		networkID:            networkID,
		signerProvider:       signerProvider,
		stateFilePath:        stateFilePath,
		milestoneIntervalSec: milestoneIntervalSec,
		powHandler:           powHandler,
		sendMesssageFunc:     sendMessageFunc,
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

		if startIndex == 1 {
			// if we bootstrap a network, NullMessageID has to be set as a solid entry point
			coo.storage.SolidEntryPointsAdd(hornet.GetNullMessageID(), startIndex)
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
func (coo *Coordinator) createAndSendMilestone(parent1MessageID *hornet.MessageID, parent2MessageID *hornet.MessageID, newMilestoneIndex milestone.Index) error {

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

	// compute merkle tree root
	mutations, err := whiteflag.ComputeWhiteFlagMutations(coo.storage, newMilestoneIndex, cachedMsgMetas, cachedMessages, parent1MessageID, parent2MessageID)
	if err != nil {
		return err
	}

	milestoneMsg, err := createMilestone(newMilestoneIndex, coo.networkID, parent1MessageID, parent2MessageID, coo.signerProvider, mutations.MerkleTreeHash, coo.powHandler)
	if err != nil {
		return err
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
		return err
	}

	coo.Events.IssuedMilestone.Trigger(coo.state.LatestMilestoneIndex, coo.state.LatestMilestoneMessageID)

	return nil
}

// Bootstrap creates the first milestone, if the network was not bootstrapped yet.
// Returns critical errors.
func (coo *Coordinator) Bootstrap() (*hornet.MessageID, error) {

	coo.milestoneLock.Lock()
	defer coo.milestoneLock.Unlock()

	if !coo.bootstrapped {
		// create first milestone to bootstrap the network
		// parent1 and parent2 reference the last known milestone or NullMessageID if startIndex = 1 (see InitState)
		if err := coo.createAndSendMilestone(coo.state.LatestMilestoneMessageID, coo.state.LatestMilestoneMessageID, coo.state.LatestMilestoneIndex+1); err != nil {
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
func (coo *Coordinator) IssueCheckpoint(checkpointIndex int, lastCheckpointMessageID *hornet.MessageID, tips hornet.MessageIDs) (*hornet.MessageID, error) {

	if len(tips) == 0 {
		return nil, ErrNoTipsGiven
	}

	coo.milestoneLock.Lock()
	defer coo.milestoneLock.Unlock()

	if !coo.storage.IsNodeSynced() {
		return nil, common.ErrNodeNotSynced
	}

	for i, tip := range tips {
		msg, err := createCheckpoint(coo.networkID, tip, lastCheckpointMessageID, coo.powHandler)
		if err != nil {
			return nil, err
		}

		if err := coo.sendMesssageFunc(msg); err != nil {
			return nil, err
		}

		lastCheckpointMessageID = msg.GetMessageID()

		coo.Events.IssuedCheckpointMessage.Trigger(checkpointIndex, i, len(tips), lastCheckpointMessageID)
	}

	return lastCheckpointMessageID, nil
}

// IssueMilestone creates the next milestone.
// Returns non-critical and critical errors.
func (coo *Coordinator) IssueMilestone(parent1MessageID *hornet.MessageID, parent2MessageID *hornet.MessageID) (*hornet.MessageID, error, error) {

	coo.milestoneLock.Lock()
	defer coo.milestoneLock.Unlock()

	if !coo.storage.IsNodeSynced() {
		// return a non-critical error to not kill the database
		return nil, common.ErrNodeNotSynced, nil
	}

	if err := coo.createAndSendMilestone(parent1MessageID, parent2MessageID, coo.state.LatestMilestoneIndex+1); err != nil {
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
