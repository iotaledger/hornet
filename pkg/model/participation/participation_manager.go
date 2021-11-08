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

// WithLogger enables logging within the ParticipationManager.
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

// Option is a function setting a ParticipationManager option.
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

func (pm *ParticipationManager) init() error {

	corrupted, err := pm.participationStoreHealth.IsCorrupted()
	if err != nil {
		return err
	}
	if corrupted {
		return ErrParticipationCorruptedStorage
	}

	correctDatabasesVersion, err := pm.participationStoreHealth.CheckCorrectDatabaseVersion()
	if err != nil {
		return err
	}

	if !correctDatabasesVersion {
		databaseVersionUpdated, err := pm.participationStoreHealth.UpdateDatabaseVersion()
		if err != nil {
			return err
		}

		if !databaseVersionUpdated {
			return errors.New("HORNET participation database version mismatch. The database scheme was updated. Please delete the database folder and start with a new snapshot.")
		}
	}

	// Read events from storage
	events, err := pm.loadEvents()
	if err != nil {
		return err
	}
	pm.events = events

	// Mark the database as corrupted here and as clean when we shut it down
	return pm.participationStoreHealth.MarkCorrupted()
}

// CloseDatabase flushes the store and closes the underlying database
func (pm *ParticipationManager) CloseDatabase() error {
	var flushAndCloseError error

	if err := pm.participationStoreHealth.MarkHealthy(); err != nil {
		flushAndCloseError = err
	}

	if err := pm.participationStore.Flush(); err != nil {
		flushAndCloseError = err
	}
	if err := pm.participationStore.Close(); err != nil {
		flushAndCloseError = err
	}
	return flushAndCloseError
}

// EventIDs return the IDs of all known events. Can be optionally filtered by event payload type.
func (pm *ParticipationManager) EventIDs(eventPayloadType ...uint32) []EventID {
	pm.RLock()
	defer pm.RUnlock()

	events := pm.events
	if len(eventPayloadType) > 0 {
		events = filteredEvents(events, eventPayloadType)
	}

	var ids []EventID
	for id := range events {
		ids = append(ids, id)
	}
	return ids
}

// Events returns all known events
func (pm *ParticipationManager) Events() []*Event {
	pm.RLock()
	defer pm.RUnlock()
	var ref []*Event
	for _, r := range pm.events {
		ref = append(ref, r)
	}
	return ref
}

func filteredEvents(events map[EventID]*Event, filterPayloadTypes []uint32) map[EventID]*Event {

	filtered := make(map[EventID]*Event)
eventLoop:
	for id, event := range events {
		eventPayloadType := event.payloadType()
		for _, payloadType := range filterPayloadTypes {
			if payloadType == eventPayloadType {
				filtered[id] = event
			}
			continue eventLoop
		}
	}
	return filtered
}

// EventsAcceptingParticipation returns the events that are currently accepting participation, i.e. commencing or in the holding period.
func (pm *ParticipationManager) EventsAcceptingParticipation() []*Event {
	return filterEvents(pm.Events(), pm.syncManager.ConfirmedMilestoneIndex(), func(e *Event, index milestone.Index) bool {
		return e.IsAcceptingParticipation(index)
	})
}

// EventsCountingParticipation returns the events that are currently actively counting participation, i.e. in the holding period
func (pm *ParticipationManager) EventsCountingParticipation() []*Event {
	return filterEvents(pm.Events(), pm.syncManager.ConfirmedMilestoneIndex(), func(e *Event, index milestone.Index) bool {
		return e.IsCountingParticipation(index)
	})
}

// StoreEvent accepts a new Event the manager should track.
// The current confirmed milestone index needs to be provided, so that the manager can check if the event can be added.
func (pm *ParticipationManager) StoreEvent(event *Event) (EventID, error) {
	pm.Lock()
	defer pm.Unlock()

	confirmedMilestoneIndex := pm.syncManager.ConfirmedMilestoneIndex()

	if confirmedMilestoneIndex >= event.EndMilestoneIndex() {
		return NullEventID, ErrParticipationEventAlreadyEnded
	}

	if confirmedMilestoneIndex >= event.CommenceMilestoneIndex() {
		return NullEventID, ErrParticipationEventAlreadyStarted
	}

	eventID, err := pm.storeEvent(event)
	if err != nil {
		return NullEventID, err
	}

	pm.events[eventID] = event

	return eventID, err
}

// Event returns the event for the given eventID if it exists
func (pm *ParticipationManager) Event(eventID EventID) *Event {
	pm.RLock()
	defer pm.RUnlock()
	return pm.events[eventID]
}

// DeleteEvent deletes the event for the given eventID if it exists, else returns ErrEventNotFound.
func (pm *ParticipationManager) DeleteEvent(eventID EventID) error {
	pm.Lock()
	defer pm.Unlock()

	event := pm.Event(eventID)
	if event == nil {
		return ErrEventNotFound
	}

	if err := pm.deleteEvent(eventID); err != nil {
		return err
	}

	delete(pm.events, eventID)
	return nil
}

// ApplyNewUTXO checks if the new UTXO is part of a participation transaction.
// The following rules must be satisfied:
// 	- Must be a value transaction
// 	- Inputs must all come from the same address. Multiple inputs are allowed.
// 	- Has a singular output going to the same address as all input addresses.
// 	- Output Type 0 (SigLockedSingleOutput) and Type 1 (SigLockedDustAllowanceOutput) are both valid for this.
// 	- The Indexation must match the configured Indexation.
//  - The participation data must be parseable.
func (pm *ParticipationManager) ApplyNewUTXO(index milestone.Index, newOutput *utxo.Output) error {

	acceptingEvents := filterEvents(pm.Events(), index, func(e *Event, index milestone.Index) bool {
		return e.ShouldAcceptParticipation(index)
	})

	// No events accepting participation, so no work to be done
	if len(acceptingEvents) == 0 {
		return nil
	}
	messageID := newOutput.MessageID()

	cachedMsg := pm.storage.CachedMessageOrNil(messageID)
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
	if !bytes.Equal(txEssenceIndexation.Index, pm.opts.indexationMessage) {
		return nil
	}

	// collect inputs
	inputOutputs := utxo.Outputs{}
	for _, input := range msg.TransactionEssenceUTXOInputs() {
		output, err := pm.storage.UTXOManager().ReadOutputByOutputIDWithoutLocking(input)
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

	validParticipations := pm.validParticipation(index, participations)

	if len(validParticipations) == 0 {
		// No participations for anything we are tracking
		return nil
	}

	mutations := pm.participationStore.Batched()

	// Store the message holding the participation
	if err := pm.storeMessage(msg, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	for _, participation := range validParticipations {

		// Store the participation started at this milestone
		if err := pm.startParticipationAtMilestone(participation.EventID, depositOutputs[0], index, mutations); err != nil {
			mutations.Cancel()
			return err
		}

		// Count the new ballot votes by increasing the current vote balance
		if err := pm.startCountingBallotAnswers(participation, depositOutputs[0].Amount(), mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}

	return mutations.Commit()
}

// ApplySpentUTXO checks if the spent UTXO was part of a participation transaction.
func (pm *ParticipationManager) ApplySpentUTXO(index milestone.Index, spent *utxo.Spent) error {

	acceptingEvents := filterEvents(pm.Events(), index, func(e *Event, index milestone.Index) bool {
		return e.ShouldAcceptParticipation(index)
	})

	// No events accepting participation, so no work to be done
	if len(acceptingEvents) == 0 {
		return nil
	}

	// Check if we tracked the participation initially, event.g. saved the Message that created this UTXO
	msg, err := pm.MessageForMessageID(spent.MessageID())
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

	validParticipations := pm.validParticipation(index, participations)

	if len(validParticipations) == 0 {
		// This might happen if the participation ended, and we spend the UTXO
		return nil
	}

	mutations := pm.participationStore.Batched()

	for _, participation := range validParticipations {

		// Store the participation ended at this milestone
		if err := pm.endParticipationAtMilestone(participation.EventID, spent.Output(), index, mutations); err != nil {
			if errors.Is(err, ErrUnknownParticipation) {
				// This was a previously invalid participation, so we did not track it
				continue
			}
			mutations.Cancel()
			return err
		}

		// Count the spent votes by decreasing the current vote balance
		if err := pm.stopCountingBallotAnswers(participation, spent.Output().Amount(), mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}

	return mutations.Commit()
}

// ApplyNewConfirmedMilestoneIndex iterates over each counting ballot participation and applies the current vote balance for each question to the total vote balance
func (pm *ParticipationManager) ApplyNewConfirmedMilestoneIndex(index milestone.Index) error {

	countingEvents := filterEvents(pm.Events(), index, func(e *Event, index milestone.Index) bool {
		return e.ShouldCountParticipation(index)
	})

	// No events counting participation, so no work to be done
	if len(countingEvents) == 0 {
		return nil
	}

	mutations := pm.participationStore.Batched()

	// Iterate over all known events that are currently counting
	for _, event := range countingEvents {

		eventID, err := event.ID()
		if err != nil {
			mutations.Cancel()
			return err
		}

		increaseAnswerValueBalances := func(questionIndex uint8, answerValue uint8) error {
			accumulatedBalance, err := pm.AccumulatedBallotVoteBalanceForQuestionAndAnswer(eventID, questionIndex, answerValue)
			if err != nil {
				mutations.Cancel()
				return err
			}

			currentBalance, err := pm.CurrentBallotVoteBalanceForQuestionAndAnswer(eventID, questionIndex, answerValue)
			if err != nil {
				mutations.Cancel()
				return err
			}

			// Add current vote balance to accumulated vote balance for each answer
			newAccumulatedBalance := accumulatedBalance + currentBalance

			if err := setAccumulatedBallotVoteBalanceForQuestionAndAnswer(eventID, questionIndex, answerValue, newAccumulatedBalance, mutations); err != nil {
				mutations.Cancel()
				return err
			}
			return nil
		}

		// For each participation, iterate over all questions
		for idx, question := range event.BallotQuestions() {
			questionIndex := uint8(idx)

			// For each question, iterate over all answers values
			for _, answer := range question.QuestionAnswers() {
				if err := increaseAnswerValueBalances(questionIndex, answer.Value); err != nil {
					return err
				}
			}
			if err := increaseAnswerValueBalances(questionIndex, AnswerValueSkipped); err != nil {
				return err
			}
			if err := increaseAnswerValueBalances(questionIndex, AnswerValueInvalid); err != nil {
				return err
			}
		}

		// End all participation if event is ending this milestone
		if event.EndMilestoneIndex() == index {
			if err := pm.endAllParticipationsAtMilestone(eventID, index, mutations); err != nil {
				mutations.Cancel()
				return err
			}
		}
	}

	return mutations.Commit()
}

func (pm *ParticipationManager) validParticipation(index milestone.Index, votes []*Participation) []*Participation {

	var validParticipations []*Participation
	for _, vote := range votes {

		// Check that we have the event for the given participation
		event := pm.Event(vote.EventID)
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

func filterEvents(events []*Event, index milestone.Index, includeFunc func(e *Event, index milestone.Index) bool) []*Event {
	var filtered []*Event
	for _, event := range events {
		if includeFunc(event, index) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}
