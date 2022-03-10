package coordinator

import (
	"context"
	"fmt"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/utils"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go/v3"

	// import implementation
	_ "golang.org/x/crypto/blake2b"
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

// MerkleTreeHash is the merkle tree root hash of all messages.
type MerkleTreeHash [iotago.MilestoneInclusionMerkleProofLength]byte

// Events are the events issued by the coordinator.
type Events struct {
	// Fired when a checkpoint message is issued.
	IssuedCheckpointMessage *events.Event
	// Fired when a milestone is issued.
	IssuedMilestone *events.Event
	// SoftError is triggered when a soft error is encountered.
	SoftError *events.Event
	// QuorumFinished is triggered after a coordinator quorum call was finished.
	QuorumFinished *events.Event
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
	// the logger used to log events.
	*utils.WrappedLogger

	// used to issue only one milestone at a time.
	milestoneLock syncutils.Mutex
	// used to access the node storage.
	storage *storage.Storage
	// used to determine the sync status of the node.
	syncManager *syncmanager.SyncManager
	// id of the network the coordinator is running in.
	networkID uint64
	// Deserialization parameters including byte costs
	deSeriParas *iotago.DeSerializationParameters
	// used to get receipts for the WOTS migration.
	migratorService *migrator.MigratorService
	// used to get the treasury output.
	utxoManager *utxo.Manager
	// used to sign the milestones.
	signerProvider MilestoneSignerProvider
	// used to do the PoW for the coordinator messages.
	powHandler *pow.Handler
	// the function used to send a message.
	sendMesssageFunc SendMessageFunc
	// holds the coordinator options.
	opts *Options

	// back pressure functions that signal congestion.
	backpressureFuncs []BackPressureFunc
	// state of the coordinator holds information about the last issued milestones.
	state *State
	// whether the coordinator was bootstrapped.
	bootstrapped bool
	// events of the coordinator.
	Events *Events
}

const (
	defaultStateFilePath     = "coordinator.state"
	defaultMilestoneInterval = time.Duration(10) * time.Second
	defaultPoWWorkerCount    = 0
)

// the default options applied to the Coordinator.
var defaultOptions = []Option{
	WithStateFilePath(defaultStateFilePath),
	WithMilestoneInterval(defaultMilestoneInterval),
	WithPoWWorkerCount(defaultPoWWorkerCount),
	WithSigningRetryAmount(10),
	WithSigningRetryTimeout(2 * time.Second),
}

// Options define options for the Coordinator.
type Options struct {
	// the logger used to log events.
	logger *logger.Logger
	// the path to the state file of the coordinator.
	stateFilePath string
	// the interval milestones are issued.
	milestoneInterval time.Duration
	// the timeout between signing retries.
	signingRetryTimeout time.Duration
	// the amount of times to retry signing before bailing and shutting down the Coordinator.
	signingRetryAmount int
	// the amount of workers used for calculating PoW when issuing checkpoints and milestones.
	powWorkerCount int
	// the optional quorum used by the coordinator to check for correct ledger state calculation.
	quorum *quorum
}

// applies the given Option.
func (so *Options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(so)
	}
}

// WithLogger enables logging within the coordinator.
func WithLogger(logger *logger.Logger) Option {
	return func(opts *Options) {
		opts.logger = logger
	}
}

// WithStateFilePath defines the path to the state file of the coordinator.
func WithStateFilePath(stateFilePath string) Option {
	return func(opts *Options) {
		opts.stateFilePath = stateFilePath
	}
}

// WithMilestoneInterval defines interval milestones are issued.
func WithMilestoneInterval(milestoneInterval time.Duration) Option {
	return func(opts *Options) {
		opts.milestoneInterval = milestoneInterval
	}
}

// WithSigningRetryTimeout defines signing retry timeout.
func WithSigningRetryTimeout(timeout time.Duration) Option {
	return func(opts *Options) {
		opts.signingRetryTimeout = timeout
	}
}

// WithSigningRetryAmount defines signing retry amount.
func WithSigningRetryAmount(amount int) Option {
	return func(opts *Options) {
		opts.signingRetryAmount = amount
	}
}

// WithPoWWorkerCount defines the amount of workers used for calculating PoW when issuing checkpoints and milestones.
func WithPoWWorkerCount(powWorkerCount int) Option {

	if powWorkerCount == 0 {
		powWorkerCount = runtime.NumCPU() - 1
	}

	if powWorkerCount < 1 {
		powWorkerCount = 1
	}

	return func(opts *Options) {
		opts.powWorkerCount = powWorkerCount
	}
}

// WithQuorum defines a quorum, which is used to check the correct ledger state of the coordinator.
// If no quorumGroups are given, the quorum is disabled.
func WithQuorum(quorumEnabled bool, quorumGroups map[string][]*QuorumClientConfig, timeout time.Duration) Option {
	return func(opts *Options) {
		if !quorumEnabled {
			opts.quorum = nil
			return
		}
		opts.quorum = newQuorum(quorumGroups, timeout)
	}
}

// Option is a function setting a coordinator option.
type Option func(opts *Options)

// New creates a new coordinator instance.
func New(
	dbStorage *storage.Storage,
	syncManager *syncmanager.SyncManager,
	networkID uint64,
	deSeriParas *iotago.DeSerializationParameters,
	signerProvider MilestoneSignerProvider,
	migratorService *migrator.MigratorService,
	utxoManager *utxo.Manager,
	powHandler *pow.Handler,
	sendMessageFunc SendMessageFunc,
	opts ...Option) (*Coordinator, error) {

	options := &Options{}
	options.apply(defaultOptions...)
	options.apply(opts...)

	result := &Coordinator{
		storage:          dbStorage,
		syncManager:      syncManager,
		networkID:        networkID,
		deSeriParas:      deSeriParas,
		signerProvider:   signerProvider,
		migratorService:  migratorService,
		utxoManager:      utxoManager,
		powHandler:       powHandler,
		sendMesssageFunc: sendMessageFunc,
		opts:             options,

		Events: &Events{
			IssuedCheckpointMessage: events.NewEvent(CheckpointCaller),
			IssuedMilestone:         events.NewEvent(MilestoneCaller),
			SoftError:               events.NewEvent(events.ErrorCaller),
			QuorumFinished:          events.NewEvent(QuorumFinishedCaller),
		},
	}
	result.WrappedLogger = utils.NewWrappedLogger(options.logger)

	return result, nil
}

// InitState loads an existing state file or bootstraps the network.
// All errors are critical.
func (coo *Coordinator) InitState(bootstrap bool, startIndex milestone.Index) error {

	_, err := os.Stat(coo.opts.stateFilePath)
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

		latestMilestoneMessageID := hornet.NullMessageID()
		if startIndex != 1 {
			// If we don't start a new network, the last milestone has to be referenced
			cachedMsgMilestone := coo.storage.MilestoneCachedMessageOrNil(latestMilestoneFromDatabase) // message +1
			if cachedMsgMilestone == nil {
				return fmt.Errorf("latest milestone (%d) not found in database. database is corrupt", latestMilestoneFromDatabase)
			}
			latestMilestoneMessageID = cachedMsgMilestone.Message().MessageID()
			cachedMsgMilestone.Release() // message -1
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
		return fmt.Errorf("state file not found: %v", coo.opts.stateFilePath)
	}

	coo.state = &State{}
	if err := utils.ReadJSONFromFile(coo.opts.stateFilePath, coo.state); err != nil {
		return err
	}

	if latestMilestoneFromDatabase != coo.state.LatestMilestoneIndex {
		return fmt.Errorf("previous milestone does not match latest milestone in database. previous: %d, database: %d", coo.state.LatestMilestoneIndex, latestMilestoneFromDatabase)
	}

	cachedMsgMilestone := coo.storage.MilestoneCachedMessageOrNil(latestMilestoneFromDatabase) // message +1
	if cachedMsgMilestone == nil {
		return fmt.Errorf("latest milestone (%d) not found in database. database is corrupt", latestMilestoneFromDatabase)
	}
	cachedMsgMilestone.Release() // message -1

	coo.bootstrapped = true
	return nil
}

// createAndSendMilestone creates a milestone, sends it to the network and stores a new coordinator state file.
// Returns non-critical and critical errors.
func (coo *Coordinator) createAndSendMilestone(parents hornet.MessageIDs, newMilestoneIndex milestone.Index) error {

	parents = parents.RemoveDupsAndSortByLexicalOrder()

	messagesMemcache := storage.NewMessagesMemcache(coo.storage.CachedMessage)
	metadataMemcache := storage.NewMetadataMemcache(coo.storage.CachedMessageMetadata)
	memcachedParentsTraverserStorage := dag.NewMemcachedParentsTraverserStorage(coo.storage, metadataMemcache)

	defer func() {
		// all releases are forced since the cone is referenced and not needed anymore
		memcachedParentsTraverserStorage.Cleanup(true)

		// release all messages at the end
		messagesMemcache.Cleanup(true)

		// Release all message metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	parentsTraverser := dag.NewParentsTraverser(memcachedParentsTraverserStorage)

	// We have to set a timestamp for when we run the white-flag mutations due to the semantic validation.
	// This should be exactly the same one used when issuing the milestone later on.
	newMilestoneTimestamp := time.Now()

	// compute merkle tree root
	// we pass a background context here to not cancel the white-flag computation!
	// otherwise the coordinator could panic at shutdown.
	mutations, err := whiteflag.ComputeWhiteFlagMutations(context.Background(), coo.storage.UTXOManager(), parentsTraverser, messagesMemcache.CachedMessage, coo.networkID, newMilestoneIndex, uint64(newMilestoneTimestamp.Unix()), parents, whiteflag.DefaultWhiteFlagTraversalCondition)
	if err != nil {
		return common.CriticalError(fmt.Errorf("failed to compute white flag mutations: %w", err))
	}

	// ask the quorum for correct ledger state if enabled
	if coo.opts.quorum != nil {
		ts := time.Now()
		err := coo.opts.quorum.checkMerkleTreeHash(mutations.MerkleTreeHash, newMilestoneIndex, parents, func(groupName string, entry *quorumGroupEntry, err error) {
			coo.LogInfof("coordinator quorum group encountered an error, group: %s, baseURL: %s, err: %s", groupName, entry.stats.BaseURL, err)
		})

		duration := time.Since(ts)
		coo.Events.QuorumFinished.Trigger(&QuorumFinishedResult{Duration: duration, Err: err})

		if err != nil {
			// quorum failed => non-critical or critical error
			coo.LogInfof("coordinator quorum failed after %v, err: %s", time.Since(ts).Truncate(time.Millisecond), err)
			return err
		}

		coo.LogInfof("coordinator quorum took %v", duration.Truncate(time.Millisecond))
	}

	// get receipt data in case migrator is enabled
	var receipt *iotago.Receipt
	if coo.migratorService != nil {
		receipt = coo.migratorService.Receipt()
		if receipt != nil {
			if err := coo.migratorService.PersistState(true); err != nil {
				return common.CriticalError(fmt.Errorf("unable to persist migrator state before send: %w", err))
			}

			currentTreasuryOutput, err := coo.utxoManager.UnspentTreasuryOutputWithoutLocking()
			if err != nil {
				return common.CriticalError(fmt.Errorf("unable to fetch unspent treasury output: %w", err))
			}

			// embed treasury within the receipt
			input := &iotago.TreasuryInput{}
			copy(input[:], currentTreasuryOutput.MilestoneID[:])
			output := &iotago.TreasuryOutput{Amount: currentTreasuryOutput.Amount - receipt.Sum()}
			treasuryTx := &iotago.TreasuryTransaction{Input: input, Output: output}
			receipt.Transaction = treasuryTx
			receipt.SortFunds()
		}
	}

	milestoneMsg, err := coo.createMilestone(newMilestoneIndex, uint64(newMilestoneTimestamp.Unix()), parents, receipt, mutations.MerkleTreeHash)
	if err != nil {
		return common.CriticalError(fmt.Errorf("failed to create milestone: %w", err))
	}

	if err := coo.sendMesssageFunc(milestoneMsg, newMilestoneIndex); err != nil {
		return common.CriticalError(fmt.Errorf("failed to send milestone: %w", err))
	}

	if coo.migratorService != nil && receipt != nil {
		if err := coo.migratorService.PersistState(false); err != nil {
			return common.CriticalError(fmt.Errorf("unable to persist migrator state after send: %w", err))
		}
	}

	// always reference the last milestone directly to speed up syncing
	latestMilestoneMessageID := milestoneMsg.MessageID()

	coo.state.LatestMilestoneMessageID = latestMilestoneMessageID
	coo.state.LatestMilestoneIndex = newMilestoneIndex
	coo.state.LatestMilestoneTime = newMilestoneTimestamp

	if err := utils.WriteJSONToFile(coo.opts.stateFilePath, coo.state, 0660); err != nil {
		return common.CriticalError(fmt.Errorf("failed to update coordinator state file: %w", err))
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
		err := coo.createAndSendMilestone(hornet.MessageIDs{coo.state.LatestMilestoneMessageID}, coo.state.LatestMilestoneIndex+1)
		if err != nil {
			// creating milestone failed => always a critical error at bootstrap
			return nil, common.CriticalError(err)
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

	if !coo.syncManager.IsNodeSynced() {
		return nil, common.SoftError(common.ErrNodeNotSynced)
	}

	// check whether we should hold issuing checkpoints
	// if the node is currently under a lot of load
	if coo.checkBackPressureFunctions() {
		return nil, common.SoftError(common.ErrNodeLoadTooHigh)
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

		msg, err := coo.createCheckpoint(parents)
		if err != nil {
			return nil, common.SoftError(fmt.Errorf("failed to create checkPoint: %w", err))
		}

		if err := coo.sendMesssageFunc(msg); err != nil {
			return nil, common.SoftError(fmt.Errorf("failed to send checkPoint: %w", err))
		}

		lastCheckpointMessageID = msg.MessageID()

		coo.Events.IssuedCheckpointMessage.Trigger(checkpointIndex, i, checkpointsNumber, lastCheckpointMessageID)
	}

	return lastCheckpointMessageID, nil
}

// IssueMilestone creates the next milestone.
// Returns non-critical and critical errors.
func (coo *Coordinator) IssueMilestone(parents hornet.MessageIDs) (hornet.MessageID, error) {

	coo.milestoneLock.Lock()
	defer coo.milestoneLock.Unlock()

	if !coo.syncManager.IsNodeSynced() {
		// return a non-critical error to not kill the database
		return nil, common.SoftError(common.ErrNodeNotSynced)
	}

	// check whether we should hold issuing miletones
	// if the node is currently under a lot of load
	if coo.checkBackPressureFunctions() {
		return nil, common.SoftError(common.ErrNodeLoadTooHigh)
	}

	if err := coo.createAndSendMilestone(parents, coo.state.LatestMilestoneIndex+1); err != nil {
		// creating milestone failed => non-critical or critical error
		return nil, err
	}

	return coo.state.LatestMilestoneMessageID, nil
}

// Interval returns the interval milestones should be issued.
func (coo *Coordinator) Interval() time.Duration {
	return coo.opts.milestoneInterval
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

// QuorumStats returns statistics about the response time and errors of every node in the quorum.
func (coo *Coordinator) QuorumStats() []QuorumClientStatistic {
	if coo.opts.quorum == nil {
		return nil
	}

	return coo.opts.quorum.quorumStatsSnapshot()
}
