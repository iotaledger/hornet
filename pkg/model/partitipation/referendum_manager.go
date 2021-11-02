package partitipation

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/serializer"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go/v2"
)

// Events are the events issued by the ParticipationManager.
type Events struct {
	// SoftError is triggered when a soft error is encountered.
	SoftError *events.Event
}

var (
	ErrParticipationCorruptedStorage    = errors.New("the partitipation database was not shutdown properly")
	ErrParticipationEventAlreadyStarted = errors.New("the given partitipation event already started")
	ErrParticipationEventAlreadyEnded   = errors.New("the given partitipation event already ended")
)

// ParticipationManager is used to track the outcome of participation in the tangle.
type ParticipationManager struct {
	syncutils.RWMutex

	// used to access the node storage.
	storage *storage.Storage

	// used to sync with the nodes status.
	syncManager *syncmanager.SyncManager

	// holds the ParticipationManager options.
	opts *Options

	participationStore       kvstore.KVStore
	participationStoreHealth *storage.StoreHealthTracker

	participationEvents map[ParticipationEventID]*ParticipationEvent

	// events of the ParticipationManager.
	Events *Events
}

// the default options applied to the ParticipationManager.
var defaultOptions = []Option{
	WithIndexationMessage("IOTAVOTE"),
}

// Options define options for the ParticipationManager.
type Options struct {
	logger *logger.Logger

	indexationMessage []byte
}

// applies the given Option.
func (so *Options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(so)
	}
}

// WithLogger enables logging within the faucet.
func WithLogger(logger *logger.Logger) Option {
	return func(opts *Options) {
		opts.logger = logger
	}
}

// WithIndexationMessage defines the ParticipationManager indexation payload to track.
func WithIndexationMessage(indexationMessage string) Option {
	return func(opts *Options) {
		opts.indexationMessage = []byte(indexationMessage)
	}
}

// Option is a function setting a faucet option.
type Option func(opts *Options)

// NewManager creates a new ParticipationManager instance.
func NewManager(
	dbStorage *storage.Storage,
	syncManager *syncmanager.SyncManager,
	participationStore kvstore.KVStore,
	opts ...Option) (*ParticipationManager, error) {

	options := &Options{}
	options.apply(defaultOptions...)
	options.apply(opts...)

	manager := &ParticipationManager{
		storage:                  dbStorage,
		syncManager:              syncManager,
		participationStore:       participationStore,
		participationStoreHealth: storage.NewStoreHealthTracker(participationStore),
		opts:                     options,

		Events: &Events{
			SoftError: events.NewEvent(events.ErrorCaller),
		},
	}

	err := manager.init()
	if err != nil {
		return nil, err
	}

	return manager, nil
}

func (rm *ParticipationManager) init() error {

	corrupted, err := rm.participationStoreHealth.IsCorrupted()
	if err != nil {
		return err
	}
	if corrupted {
		return ErrParticipationCorruptedStorage
	}

	correctDatabasesVersion, err := rm.participationStoreHealth.CheckCorrectDatabaseVersion()
	if err != nil {
		return err
	}

	if !correctDatabasesVersion {
		databaseVersionUpdated, err := rm.participationStoreHealth.UpdateDatabaseVersion()
		if err != nil {
			return err
		}

		if !databaseVersionUpdated {
			return errors.New("HORNET partitipation database version mismatch. The database scheme was updated. Please delete the database folder and start with a new snapshot.")
		}
	}

	// Read events from storage
	events, err := rm.loadParticipationEvents()
	if err != nil {
		return err
	}
	rm.participationEvents = events

	// Mark the database as corrupted here and as clean when we shut it down
	return rm.participationStoreHealth.MarkCorrupted()
}

func (rm *ParticipationManager) CloseDatabase() error {
	var flushAndCloseError error

	if err := rm.participationStoreHealth.MarkHealthy(); err != nil {
		flushAndCloseError = err
	}

	if err := rm.participationStore.Flush(); err != nil {
		flushAndCloseError = err
	}
	if err := rm.participationStore.Close(); err != nil {
		flushAndCloseError = err
	}
	return flushAndCloseError
}

func (rm *ParticipationManager) ParticipationEventIDs() []ParticipationEventID {
	rm.RLock()
	defer rm.RUnlock()
	var ids []ParticipationEventID
	for id, _ := range rm.participationEvents {
		ids = append(ids, id)
	}
	return ids
}

func (rm *ParticipationManager) ParticipationEvents() []*ParticipationEvent {
	rm.RLock()
	defer rm.RUnlock()
	var ref []*ParticipationEvent
	for _, r := range rm.participationEvents {
		ref = append(ref, r)
	}
	return ref
}

// EventsAcceptingParticipation returns the participationEvents that are currently accepting participation, i.e. commencing or in the holding period.
func (rm *ParticipationManager) EventsAcceptingParticipation() []*ParticipationEvent {
	return filterReferendums(rm.ParticipationEvents(), rm.syncManager.ConfirmedMilestoneIndex(), func(ref *ParticipationEvent, index milestone.Index) bool {
		return ref.IsAcceptingParticipation(index)
	})
}

// EventsCountingParticipation returns the participationEvents that are currently actively counting participation, i.e. in the holding period
func (rm *ParticipationManager) EventsCountingParticipation() []*ParticipationEvent {
	return filterReferendums(rm.ParticipationEvents(), rm.syncManager.ConfirmedMilestoneIndex(), func(ref *ParticipationEvent, index milestone.Index) bool {
		return ref.IsCountingParticipation(index)
	})
}

// StoreReferendum accepts a new ParticipationEvent the manager should track.
// The current confirmed milestone index needs to be provided, so that the manager can check if the partitipation can be added.
func (rm *ParticipationManager) StoreReferendum(referendum *ParticipationEvent) (ParticipationEventID, error) {
	rm.Lock()
	defer rm.Unlock()

	confirmedMilestoneIndex := rm.syncManager.ConfirmedMilestoneIndex()

	if confirmedMilestoneIndex >= referendum.EndMilestoneIndex() {
		return NullParticipationEventID, ErrParticipationEventAlreadyEnded
	}

	if confirmedMilestoneIndex >= referendum.CommenceMilestoneIndex() {
		return NullParticipationEventID, ErrParticipationEventAlreadyStarted
	}

	referendumID, err := rm.storeReferendum(referendum)
	if err != nil {
		return NullParticipationEventID, err
	}

	rm.participationEvents[referendumID] = referendum

	return referendumID, err
}

func (rm *ParticipationManager) Referendum(referendumID ParticipationEventID) *ParticipationEvent {
	rm.RLock()
	defer rm.RUnlock()
	return rm.participationEvents[referendumID]
}

func (rm *ParticipationManager) DeleteReferendum(referendumID ParticipationEventID) error {
	rm.Lock()
	defer rm.Unlock()

	referendum := rm.Referendum(referendumID)
	if referendum == nil {
		return ErrReferendumNotFound
	}

	if err := rm.deleteReferendum(referendumID); err != nil {
		return err
	}

	delete(rm.participationEvents, referendumID)
	return nil
}

// logSoftError logs a soft error and triggers the event.
func (rm *ParticipationManager) logSoftError(err error) {
	if rm.opts.logger != nil {
		rm.opts.logger.Warn(err)
	}
	rm.Events.SoftError.Trigger(err)
}

// ApplyNewUTXO checks if the new UTXO is part of a voting transaction.
// The following rules must be satisfied:
// 	- Must be a value transaction
// 	- Inputs must all come from the same address. Multiple inputs are allowed.
// 	- Has a singular output going to the same address as all input addresses.
// 	- Output Type 0 (SigLockedSingleOutput) and Type 1 (SigLockedDustAllowanceOutput) are both valid for this.
// 	- The Indexation must match the configured Indexation.
//  - The vote data must be parseable.
func (rm *ParticipationManager) ApplyNewUTXO(index milestone.Index, newOutput *utxo.Output) error {

	acceptingReferendums := filterReferendums(rm.ParticipationEvents(), index, func(ref *ParticipationEvent, index milestone.Index) bool {
		return ref.ShouldAcceptParticipation(index)
	})

	// No partitipation accepting votes, so no work to be done
	if len(acceptingReferendums) == 0 {
		return nil
	}
	messageID := newOutput.MessageID()

	cachedMsg := rm.storage.CachedMessageOrNil(messageID)
	if cachedMsg == nil {
		// if the message was included, there must be a message
		return fmt.Errorf("message not found: %s", messageID.ToHex())
	}
	defer cachedMsg.Release(true)

	msg := cachedMsg.Message()

	transaction := msg.Transaction()
	if transaction == nil {
		// if the message was included, there must be a transaction payload
		return fmt.Errorf("no transaction payload found: MsgID: %s", messageID.ToHex())
	}

	txEssence := msg.TransactionEssence()
	if txEssence == nil {
		// if the message was included, there must be a transaction payload essence
		return fmt.Errorf("no transaction transactionEssence found: MsgID: %s", messageID.ToHex())
	}

	txEssenceIndexation := msg.TransactionEssenceIndexation()
	if txEssenceIndexation == nil {
		// no need to check if there is not indexation payload
		return nil
	}

	// the index of the transaction payload must match our configured indexation
	if !bytes.Equal(txEssenceIndexation.Index, rm.opts.indexationMessage) {
		return nil
	}

	// collect inputs
	inputOutputs := utxo.Outputs{}
	for _, input := range msg.TransactionEssenceUTXOInputs() {
		output, err := rm.storage.UTXOManager().ReadOutputByOutputIDWithoutLocking(input)
		if err != nil {
			return err
		}
		inputOutputs = append(inputOutputs, output)
	}

	// collect outputs
	depositOutputs := utxo.Outputs{}
	for i := 0; i < len(txEssence.Outputs); i++ {
		output, err := utxo.NewOutput(messageID, transaction, uint16(i))
		if err != nil {
			return err
		}
		depositOutputs = append(depositOutputs, output)
	}

	// only a single output is allowed
	if len(depositOutputs) != 1 {
		return nil
	}

	// only OutputSigLockedSingleOutput and OutputSigLockedDustAllowanceOutput are allowed as output type
	switch depositOutputs[0].OutputType() {
	case iotago.OutputSigLockedDustAllowanceOutput:
	case iotago.OutputSigLockedSingleOutput:
	default:
		return nil
	}

	outputAddress, err := depositOutputs[0].Address().Serialize(serializer.DeSeriModeNoValidation)
	if err != nil {
		return nil
	}

	// check if all inputs come from the same address as the output
	for _, input := range inputOutputs {
		inputAddress, err := input.Address().Serialize(serializer.DeSeriModeNoValidation)
		if err != nil {
			return nil
		}

		if !bytes.Equal(outputAddress, inputAddress) {
			// input address does not match the output address =>  not a voting transaction
			return nil
		}
	}

	votes, err := votesFromIndexation(txEssenceIndexation)
	if err != nil {
		return err
	}

	validVotes := rm.validVotes(index, votes)

	if len(validVotes) == 0 {
		// No votes for anything we are tracking
		return nil
	}

	mutations := rm.participationStore.Batched()

	// Store the message holding the vote
	if err := rm.storeMessage(msg, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	// Count the new votes by increasing the current vote balance
	for _, vote := range validVotes {

		// Store the vote started at this milestone
		if err := rm.startVoteAtMilestone(vote.ReferendumID, depositOutputs[0], index, mutations); err != nil {
			mutations.Cancel()
			return err
		}

		if err := rm.startCountingVoteAnswers(vote, depositOutputs[0].Amount(), mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}

	return mutations.Commit()
}

func (rm *ParticipationManager) ApplySpentUTXO(index milestone.Index, spent *utxo.Spent) error {

	acceptingReferendums := filterReferendums(rm.ParticipationEvents(), index, func(ref *ParticipationEvent, index milestone.Index) bool {
		return ref.ShouldAcceptParticipation(index)
	})

	// No partitipation accepting votes, so no work to be done
	if len(acceptingReferendums) == 0 {
		return nil
	}

	// Check if we tracked the vote initially, e.g. saved the Message that created this UTXO
	msg, err := rm.MessageForMessageID(spent.MessageID())
	if err != nil {
		return err
	}

	if msg == nil {
		// This UTXO had no valid vote, so we did not store the message for it
		return nil
	}

	txEssenceIndexation := msg.TransactionEssenceIndexation()
	if txEssenceIndexation == nil {
		// We tracked this vote before, and now we don't have its indexation, so something happened
		return ErrInvalidPreviouslyTrackedVote
	}

	votes, err := votesFromIndexation(txEssenceIndexation)
	if err != nil {
		return err
	}

	validVotes := rm.validVotes(index, votes)

	if len(validVotes) == 0 {
		// This might happen if the vote ended, and we spend the UTXO
		return nil
	}

	mutations := rm.participationStore.Batched()

	// Count the spent votes by decreasing the current vote balance
	for _, vote := range validVotes {

		// Store the vote ended at this milestone
		if err := rm.endVoteAtMilestone(vote.ReferendumID, spent.Output(), index, mutations); err != nil {
			if errors.Is(err, ErrUnknownVote) {
				// This was a previously invalid vote, so we did not track it
				continue
			}
			mutations.Cancel()
			return err
		}

		if err := rm.stopCountingVoteAnswers(vote, spent.Output().Amount(), mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}

	return mutations.Commit()
}

// ApplyNewConfirmedMilestoneIndex iterates over each counting partitipation and applies the current vote for each question to the total vote
func (rm *ParticipationManager) ApplyNewConfirmedMilestoneIndex(index milestone.Index) error {

	countingReferendums := filterReferendums(rm.ParticipationEvents(), index, func(ref *ParticipationEvent, index milestone.Index) bool {
		return ref.ShouldCountParticipation(index)
	})

	// No counting partitipation, so no work to be done
	if len(countingReferendums) == 0 {
		return nil
	}

	mutations := rm.participationStore.Batched()

	// Iterate over all known participationEvents that are currently counting
	for _, referendum := range countingReferendums {

		referendumID, err := referendum.ID()
		if err != nil {
			mutations.Cancel()
			return err
		}

		// For each partitipation, iterate over all questions
		for idx, question := range referendum.BallotQuestions() {
			questionIndex := uint8(idx)

			// For each question, iterate over all answers. Include 0 here, since that is valid, i.e. answer skipped by voter
			// TODO: also handle the invalid vote usecase 255
			for idx := 0; idx <= len(question.Answers); idx++ {
				answerIndex := uint8(idx)

				accumulatedBalance, err := rm.AccumulatedVoteBalanceForQuestionAndAnswer(referendumID, questionIndex, answerIndex)
				if err != nil {
					mutations.Cancel()
					return err
				}

				currentBalance, err := rm.CurrentVoteBalanceForQuestionAndAnswer(referendumID, questionIndex, answerIndex)
				if err != nil {
					mutations.Cancel()
					return err
				}

				// Add current vote balance to accumulated vote balance for each answer
				newAccumulatedBalance := accumulatedBalance + currentBalance

				if err := setAccumulatedVoteBalanceForQuestionAndAnswer(referendumID, questionIndex, answerIndex, newAccumulatedBalance, mutations); err != nil {
					mutations.Cancel()
					return err
				}
			}
		}

		// End all votes if partitipation is ending this milestone
		if referendum.EndMilestoneIndex() == index {
			if err := rm.endAllVotesAtMilestone(referendumID, index, mutations); err != nil {
				mutations.Cancel()
				return err
			}
		}
	}

	return mutations.Commit()
}

func (rm *ParticipationManager) validVotes(index milestone.Index, votes []*Vote) []*Vote {

	var validVotes []*Vote
	for _, vote := range votes {

		// Check that we have the partitipation for the given vote
		referendum := rm.Referendum(vote.ReferendumID)
		if referendum == nil {
			continue
		}

		// Check that the partitipation is accepting votes
		if !referendum.ShouldAcceptParticipation(index) {
			continue
		}

		// Check that the amount of answers equals the questions in the partitipation
		if len(vote.Answers) != len(referendum.BallotQuestions()) {
			continue
		}

		//TODO: validate answers? We would create a current vote for invalid answers, but only count valid answers and skipped (index == 0) anyway

		validVotes = append(validVotes, vote)
	}

	return validVotes
}

func votesFromIndexation(indexation *iotago.Indexation) ([]*Vote, error) {

	// try to parse the votes payload
	parsedVotes := &Votes{}
	if _, err := parsedVotes.Deserialize(indexation.Data, serializer.DeSeriModePerformValidation); err != nil {
		// votes payload can't be parsed => ignore votes
		return nil, fmt.Errorf("no valid votes payload")
	}

	var votes []*Vote
	for _, vote := range parsedVotes.Votes {
		votes = append(votes, vote.(*Vote))
	}

	return votes, nil
}

func filterReferendums(referendums []*ParticipationEvent, index milestone.Index, includeFunc func(ref *ParticipationEvent, index milestone.Index) bool) []*ParticipationEvent {
	var filtered []*ParticipationEvent
	for _, referendum := range referendums {
		if includeFunc(referendum, index) {
			filtered = append(filtered, referendum)
		}
	}
	return filtered
}
