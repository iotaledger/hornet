package participation

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/serializer"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go/v2"
)

var (
	ErrParticipationCorruptedStorage    = errors.New("the participation database was not shutdown properly")
	ErrParticipationEventAlreadyStarted = errors.New("the given participation event already started")
	ErrParticipationEventAlreadyEnded   = errors.New("the given participation event already ended")
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

	events map[EventID]*Event
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
			return errors.New("HORNET participation database version mismatch. The database scheme was updated. Please delete the database folder and start with a new snapshot.")
		}
	}

	// Read events from storage
	events, err := rm.loadEvents()
	if err != nil {
		return err
	}
	rm.events = events

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

func (rm *ParticipationManager) EventIDs() []EventID {
	rm.RLock()
	defer rm.RUnlock()
	var ids []EventID
	for id, _ := range rm.events {
		ids = append(ids, id)
	}
	return ids
}

func (rm *ParticipationManager) Events() []*Event {
	rm.RLock()
	defer rm.RUnlock()
	var ref []*Event
	for _, r := range rm.events {
		ref = append(ref, r)
	}
	return ref
}

// EventsAcceptingParticipation returns the events that are currently accepting participation, i.event. commencing or in the holding period.
func (rm *ParticipationManager) EventsAcceptingParticipation() []*Event {
	return filterEvents(rm.Events(), rm.syncManager.ConfirmedMilestoneIndex(), func(ref *Event, index milestone.Index) bool {
		return ref.IsAcceptingParticipation(index)
	})
}

// EventsCountingParticipation returns the events that are currently actively counting participation, i.event. in the holding period
func (rm *ParticipationManager) EventsCountingParticipation() []*Event {
	return filterEvents(rm.Events(), rm.syncManager.ConfirmedMilestoneIndex(), func(ref *Event, index milestone.Index) bool {
		return ref.IsCountingParticipation(index)
	})
}

// StoreEvent accepts a new Event the manager should track.
// The current confirmed milestone index needs to be provided, so that the manager can check if the event can be added.
func (rm *ParticipationManager) StoreEvent(event *Event) (EventID, error) {
	rm.Lock()
	defer rm.Unlock()

	confirmedMilestoneIndex := rm.syncManager.ConfirmedMilestoneIndex()

	if confirmedMilestoneIndex >= event.EndMilestoneIndex() {
		return NullEventID, ErrParticipationEventAlreadyEnded
	}

	if confirmedMilestoneIndex >= event.CommenceMilestoneIndex() {
		return NullEventID, ErrParticipationEventAlreadyStarted
	}

	eventID, err := rm.storeEvent(event)
	if err != nil {
		return NullEventID, err
	}

	rm.events[eventID] = event

	return eventID, err
}

func (rm *ParticipationManager) Event(eventID EventID) *Event {
	rm.RLock()
	defer rm.RUnlock()
	return rm.events[eventID]
}

func (rm *ParticipationManager) DeleteEvent(eventID EventID) error {
	rm.Lock()
	defer rm.Unlock()

	event := rm.Event(eventID)
	if event == nil {
		return ErrEventNotFound
	}

	if err := rm.deleteEvent(eventID); err != nil {
		return err
	}

	delete(rm.events, eventID)
	return nil
}

// ApplyNewUTXO checks if the new UTXO is part of a voting transaction.
// The following rules must be satisfied:
// 	- Must be a value transaction
// 	- Inputs must all come from the same address. Multiple inputs are allowed.
// 	- Has a singular output going to the same address as all input addresses.
// 	- Output Type 0 (SigLockedSingleOutput) and Type 1 (SigLockedDustAllowanceOutput) are both valid for this.
// 	- The Indexation must match the configured Indexation.
//  - The participation data must be parseable.
func (rm *ParticipationManager) ApplyNewUTXO(index milestone.Index, newOutput *utxo.Output) error {

	acceptingEvents := filterEvents(rm.Events(), index, func(ref *Event, index milestone.Index) bool {
		return ref.ShouldAcceptParticipation(index)
	})

	// No events accepting participation, so no work to be done
	if len(acceptingEvents) == 0 {
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

	participations, err := participationFromIndexation(txEssenceIndexation)
	if err != nil {
		return err
	}

	validParticipations := rm.validParticipation(index, participations)

	if len(validParticipations) == 0 {
		// No participations for anything we are tracking
		return nil
	}

	mutations := rm.participationStore.Batched()

	// Store the message holding the participation
	if err := rm.storeMessage(msg, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	for _, participation := range validParticipations {

		// Store the participation started at this milestone
		if err := rm.startParticipationAtMilestone(participation.EventID, depositOutputs[0], index, mutations); err != nil {
			mutations.Cancel()
			return err
		}

		// Count the new ballot votes by increasing the current vote balance
		if err := rm.startCountingBallotAnswers(participation, depositOutputs[0].Amount(), mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}

	return mutations.Commit()
}

func (rm *ParticipationManager) ApplySpentUTXO(index milestone.Index, spent *utxo.Spent) error {

	acceptingEvents := filterEvents(rm.Events(), index, func(ref *Event, index milestone.Index) bool {
		return ref.ShouldAcceptParticipation(index)
	})

	// No events accepting participation, so no work to be done
	if len(acceptingEvents) == 0 {
		return nil
	}

	// Check if we tracked the participation initially, event.g. saved the Message that created this UTXO
	msg, err := rm.MessageForMessageID(spent.MessageID())
	if err != nil {
		return err
	}

	if msg == nil {
		// This UTXO had no valid participation, so we did not store the message for it
		return nil
	}

	txEssenceIndexation := msg.TransactionEssenceIndexation()
	if txEssenceIndexation == nil {
		// We tracked this participation before, and now we don't have its indexation, so something happened
		return ErrInvalidPreviouslyTrackedParticipation
	}

	participations, err := participationFromIndexation(txEssenceIndexation)
	if err != nil {
		return err
	}

	validParticipations := rm.validParticipation(index, participations)

	if len(validParticipations) == 0 {
		// This might happen if the participation ended, and we spend the UTXO
		return nil
	}

	mutations := rm.participationStore.Batched()

	for _, participation := range validParticipations {

		// Store the participation ended at this milestone
		if err := rm.endParticipationAtMilestone(participation.EventID, spent.Output(), index, mutations); err != nil {
			if errors.Is(err, ErrUnknownParticipation) {
				// This was a previously invalid participation, so we did not track it
				continue
			}
			mutations.Cancel()
			return err
		}

		// Count the spent votes by decreasing the current vote balance
		if err := rm.stopCountingBallotAnswers(participation, spent.Output().Amount(), mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}

	return mutations.Commit()
}

// ApplyNewConfirmedMilestoneIndex iterates over each counting ballot participation and applies the current vote balance for each question to the total vote balance
func (rm *ParticipationManager) ApplyNewConfirmedMilestoneIndex(index milestone.Index) error {

	countingEvents := filterEvents(rm.Events(), index, func(ref *Event, index milestone.Index) bool {
		return ref.ShouldCountParticipation(index)
	})

	// No events counting participation, so no work to be done
	if len(countingEvents) == 0 {
		return nil
	}

	mutations := rm.participationStore.Batched()

	// Iterate over all known events that are currently counting
	for _, event := range countingEvents {

		eventID, err := event.ID()
		if err != nil {
			mutations.Cancel()
			return err
		}

		// For each participation, iterate over all questions
		for idx, question := range event.BallotQuestions() {
			questionIndex := uint8(idx)

			// For each question, iterate over all answers. Include 0 here, since that is valid, i.event. answer skipped by voter
			// TODO: also handle the invalid vote usecase 255
			for idx := 0; idx <= len(question.Answers); idx++ {
				answerIndex := uint8(idx)

				accumulatedBalance, err := rm.AccumulatedBallotVoteBalanceForQuestionAndAnswer(eventID, questionIndex, answerIndex)
				if err != nil {
					mutations.Cancel()
					return err
				}

				currentBalance, err := rm.CurrentBallotVoteBalanceForQuestionAndAnswer(eventID, questionIndex, answerIndex)
				if err != nil {
					mutations.Cancel()
					return err
				}

				// Add current vote balance to accumulated vote balance for each answer
				newAccumulatedBalance := accumulatedBalance + currentBalance

				if err := setAccumulatedBallotVoteBalanceForQuestionAndAnswer(eventID, questionIndex, answerIndex, newAccumulatedBalance, mutations); err != nil {
					mutations.Cancel()
					return err
				}
			}
		}

		// End all participation if event is ending this milestone
		if event.EndMilestoneIndex() == index {
			if err := rm.endAllParticipationsAtMilestone(eventID, index, mutations); err != nil {
				mutations.Cancel()
				return err
			}
		}
	}

	return mutations.Commit()
}

func (rm *ParticipationManager) validParticipation(index milestone.Index, votes []*Participation) []*Participation {

	var validParticipations []*Participation
	for _, vote := range votes {

		// Check that we have the event for the given participation
		event := rm.Event(vote.EventID)
		if event == nil {
			continue
		}

		// Check that the event is accepting participations
		if !event.ShouldAcceptParticipation(index) {
			continue
		}

		// Check that the amount of answers equals the questions in the ballot
		if len(vote.Answers) != len(event.BallotQuestions()) {
			continue
		}

		//TODO: validate answers? We would create a current vote for invalid answers, but only count valid answers and skipped (index == 0) anyway

		validParticipations = append(validParticipations, vote)
	}

	return validParticipations
}

func participationFromIndexation(indexation *iotago.Indexation) ([]*Participation, error) {

	// try to parse the votes payload
	parsedVotes := &Participations{}
	if _, err := parsedVotes.Deserialize(indexation.Data, serializer.DeSeriModePerformValidation); err != nil {
		// votes payload can't be parsed => ignore votes
		return nil, fmt.Errorf("no valid votes payload")
	}

	var votes []*Participation
	for _, vote := range parsedVotes.Participations {
		votes = append(votes, vote.(*Participation))
	}

	return votes, nil
}

func filterEvents(events []*Event, index milestone.Index, includeFunc func(ref *Event, index milestone.Index) bool) []*Event {
	var filtered []*Event
	for _, event := range events {
		if includeFunc(event, index) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}
