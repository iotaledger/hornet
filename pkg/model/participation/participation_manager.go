package participation

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	ErrParticipationCorruptedStorage               = errors.New("the participation database was not shutdown properly")
	ErrParticipationEventStartedBeforePruningIndex = errors.New("the given participation event started before the pruning index of this node")
	ErrParticipationEventBallotCanOverflow         = errors.New("the given participation duration in combination with the maximum voting weight can overflow uint64")
	ErrParticipationEventStakingCanOverflow        = errors.New("the given participation staking nominator and denominator in combination with the duration can overflow uint64")
	ErrParticipationEventAlreadyExists             = errors.New("the given participation event already exists")
)

// ParticipationManager is used to track the outcome of participation in the tangle.
type ParticipationManager struct {
	// lock used to secure the state of the ParticipationManager.
	syncutils.RWMutex

	// used to access the node storage.
	storage *storage.Storage

	// used to sync with the nodes status.
	syncManager *syncmanager.SyncManager

	// ProtocolParameters parameters including byte costs
	protoParas *iotago.ProtocolParameters

	// holds the ParticipationManager options.
	opts *Options

	participationStore       kvstore.KVStore
	participationStoreHealth *storage.StoreHealthTracker

	events map[EventID]*Event
}

// the default options applied to the ParticipationManager.
var defaultOptions = []Option{
	WithTagMessage("PARTICIPATE"),
}

// Options define options for the ParticipationManager.
type Options struct {
	// defines the tag payload to track
	tagMessage []byte
}

// applies the given Option.
func (so *Options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(so)
	}
}

// WithTagMessage defines the ParticipationManager tag payload to track.
func WithTagMessage(tagMessage string) Option {
	return func(opts *Options) {
		opts.tagMessage = []byte(tagMessage)
	}
}

// Option is a function setting a ParticipationManager option.
type Option func(opts *Options)

// NewManager creates a new ParticipationManager instance.
func NewManager(
	dbStorage *storage.Storage,
	syncManager *syncmanager.SyncManager,
	participationStore kvstore.KVStore,
	protoParas *iotago.ProtocolParameters,
	opts ...Option) (*ParticipationManager, error) {

	options := &Options{}
	options.apply(defaultOptions...)
	options.apply(opts...)

	healthTracker, err := storage.NewStoreHealthTracker(participationStore, DBVersionParticipation)
	if err != nil {
		return nil, err
	}

	manager := &ParticipationManager{
		storage:                  dbStorage,
		syncManager:              syncManager,
		participationStore:       participationStore,
		participationStoreHealth: healthTracker,
		protoParas:               protoParas,
		opts:                     options,
	}

	err = manager.init()
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
func (pm *ParticipationManager) Events() map[EventID]*Event {
	pm.RLock()
	defer pm.RUnlock()
	events := make(map[EventID]*Event)
	for id, e := range pm.events {
		events[id] = e
	}
	return events
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
func (pm *ParticipationManager) EventsAcceptingParticipation() map[EventID]*Event {
	return filterEvents(pm.Events(), pm.syncManager.ConfirmedMilestoneIndex(), func(e *Event, index milestone.Index) bool {
		return e.IsAcceptingParticipation(index)
	})
}

// EventsCountingParticipation returns the events that are currently actively counting participation, i.e. in the holding period
func (pm *ParticipationManager) EventsCountingParticipation() map[EventID]*Event {
	return filterEvents(pm.Events(), pm.syncManager.ConfirmedMilestoneIndex(), func(e *Event, index milestone.Index) bool {
		return e.IsCountingParticipation(index)
	})
}

// StoreEvent accepts a new Event the manager should track.
// The current confirmed milestone index needs to be provided, so that the manager can check if the event can be added.
func (pm *ParticipationManager) StoreEvent(event *Event) (EventID, error) {
	pm.Lock()
	defer pm.Unlock()

	eventID, err := event.ID()
	if err != nil {
		return NullEventID, err
	}

	if _, exists := pm.events[eventID]; exists {
		return NullEventID, ErrParticipationEventAlreadyExists
	}

	if event.BallotCanOverflow(pm.protoParas) {
		return NullEventID, ErrParticipationEventBallotCanOverflow
	}

	if event.StakingCanOverflow(pm.protoParas) {
		return NullEventID, ErrParticipationEventStakingCanOverflow
	}

	if pm.syncManager.ConfirmedMilestoneIndex() >= event.CommenceMilestoneIndex() {
		if err := pm.calculatePastParticipationForEvent(event); err != nil {
			return NullEventID, err
		}
	}

	if _, err = pm.storeEvent(event); err != nil {
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

	event := pm.events[eventID]
	if event == nil {
		return ErrEventNotFound
	}

	if err := pm.clearStorageForEventID(eventID); err != nil {
		return err
	}

	if err := pm.deleteEvent(eventID); err != nil {
		return err
	}

	delete(pm.events, eventID)
	return nil
}

func (pm *ParticipationManager) calculatePastParticipationForEvent(event *Event) error {

	snapshotInfo := pm.storage.SnapshotInfo()
	if snapshotInfo.PruningIndex >= event.CommenceMilestoneIndex() {
		return ErrParticipationEventStartedBeforePruningIndex
	}

	eventID, err := event.ID()
	if err != nil {
		return err
	}

	// Make sure we have no data from a previous import in the storage
	if err := pm.clearStorageForEventID(eventID); err != nil {
		return err
	}

	events := make(map[EventID]*Event)
	events[eventID] = event

	utxoManager := pm.storage.UTXOManager()

	//Lock the UTXO ledger so that the node cannot keep confirming until we are done here, else we might have gaps or process the milestones twice
	utxoManager.ReadLockLedger()
	defer utxoManager.ReadUnlockLedger()

	currentIndex := event.CommenceMilestoneIndex()
	for {
		msDiff, err := utxoManager.MilestoneDiffWithoutLocking(currentIndex)
		if err != nil {
			return err
		}

		for _, output := range msDiff.Outputs {
			if err := pm.applyNewUTXOForEvents(currentIndex, output, events); err != nil {
				return err
			}
		}

		for _, spent := range msDiff.Spents {
			if err := pm.applySpentUTXOForEvents(currentIndex, spent, events); err != nil {
				return err
			}
		}

		if err := pm.applyNewConfirmedMilestoneIndexForEvents(currentIndex, events); err != nil {
			return err
		}

		if currentIndex >= pm.syncManager.ConfirmedMilestoneIndex() || currentIndex >= event.EndMilestoneIndex() {
			// We are done
			break
		}
		currentIndex++
	}

	return nil
}

func (pm *ParticipationManager) ApplyNewLedgerUpdate(index milestone.Index, created utxo.Outputs, consumed utxo.Spents) error {

	acceptingEvents := filterEvents(pm.Events(), index, func(e *Event, index milestone.Index) bool {
		return e.ShouldAcceptParticipation(index)
	})

	// No events accepting participation, so no work to be done
	if len(acceptingEvents) == 0 {
		return nil
	}

	for _, newOutput := range created {
		if err := pm.applyNewUTXOForEvents(index, newOutput, acceptingEvents); err != nil {
			return err
		}
	}
	for _, spent := range consumed {
		if err := pm.applySpentUTXOForEvents(index, spent, acceptingEvents); err != nil {
			return err
		}
	}

	return pm.applyNewConfirmedMilestoneIndexForEvents(index, acceptingEvents)
}

// applyNewUTXOForEvents checks if the new UTXO is part of a participation transaction.
// The following rules must be satisfied:
// 	- Must be a value transaction
// 	- Inputs must all come from the same address. Multiple inputs are allowed.
// 	- Has a singular output going to the same address as all input addresses.
// 	- Output Type 0 (SigLockedSingleOutput) and Type 1 (SigLockedDustAllowanceOutput) are both valid for this.
// 	- The Indexation must match the configured Indexation.
//  - The participation data must be parseable.
func (pm *ParticipationManager) applyNewUTXOForEvents(index milestone.Index, newOutput *utxo.Output, events map[EventID]*Event) error {
	messageID := newOutput.MessageID()

	cachedMsg := pm.storage.CachedMessageOrNil(messageID) // message +1
	if cachedMsg == nil {
		// if the message was included, there must be a message
		return fmt.Errorf("message not found: %s", messageID.ToHex())
	}
	defer cachedMsg.Release(true) // message -1

	msg := cachedMsg.Message()

	depositOutput, participations, err := pm.ParticipationsFromMessage(msg, index)
	if err != nil {
		return err
	}

	if depositOutput == nil {
		// No output with participations, so ignore
		return nil
	}

	validParticipations := filterValidParticipationsForEvents(index, participations, events)

	if len(validParticipations) == 0 {
		// No participations for anything we are tracking
		return nil
	}

	mutations, err := pm.participationStore.Batched()
	if err != nil {
		return err
	}

	for _, participation := range validParticipations {

		// Store the message holding the participation for this event
		if err := pm.storeMessageForEvent(participation.EventID, msg, mutations); err != nil {
			mutations.Cancel()
			return err
		}

		// Store the participation started at this milestone
		if err := pm.startParticipationAtMilestone(participation.EventID, depositOutput, index, mutations); err != nil {
			mutations.Cancel()
			return err
		}

		event, ok := events[participation.EventID]
		if !ok {
			mutations.Cancel()
			return nil
		}

		switch payload := event.Payload.(type) {
		case *Ballot:
			// Count the new ballot votes by increasing the current vote balance
			if err := pm.startCountingBallotAnswers(event, participation, index, depositOutput.Deposit(), mutations); err != nil {
				mutations.Cancel()
				return err
			}
		case *Staking:
			// Increase the staked amount
			if err := pm.increaseStakedAmountForStakingEvent(participation.EventID, index, depositOutput.Deposit(), mutations); err != nil {
				mutations.Cancel()
				return err
			}
			// Increase the staking rewards
			if err := pm.increaseCurrentRewardsPerMilestoneForStakingEvent(participation.EventID, index, payload.rewardsPerMilestone(depositOutput.Deposit()), mutations); err != nil {
				mutations.Cancel()
				return err
			}
		}
	}

	return mutations.Commit()
}

// applySpentUTXOForEvents checks if the spent UTXO was part of a participation transaction.
func (pm *ParticipationManager) applySpentUTXOForEvents(index milestone.Index, spent *utxo.Spent, events map[EventID]*Event) error {

	// Fetch the message, this must have been stored for at least one of the events
	var msg *storage.Message
	for eID := range events {
		// Check if we tracked the participation initially, event.g. saved the Message that created this UTXO
		messageForEvent, err := pm.MessageForEventAndMessageID(eID, spent.MessageID())
		if err != nil {
			return err
		}
		if messageForEvent != nil {
			msg = messageForEvent
			break
		}
	}

	if msg == nil {
		// This UTXO had no valid participation, so we did not store the message for it
		return nil
	}

	txEssenceTaggedData := msg.TransactionEssenceTaggedData()
	if txEssenceTaggedData == nil {
		// We tracked this participation before, and now we don't have its taggedData, so something happened
		return ErrInvalidPreviouslyTrackedParticipation
	}

	participations, err := participationFromTaggedData(txEssenceTaggedData)
	if err != nil {
		return err
	}

	validParticipations := filterValidParticipationsForEvents(index, participations, events)

	if len(validParticipations) == 0 {
		// This might happen if the participation ended, and we spend the UTXO
		return nil
	}

	mutations, err := pm.participationStore.Batched()
	if err != nil {
		return err
	}

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

		event, ok := events[participation.EventID]
		if !ok {
			mutations.Cancel()
			return nil
		}

		switch payload := event.Payload.(type) {
		case *Ballot:
			// Count the spent votes by decreasing the current vote balance
			if err := pm.stopCountingBallotAnswers(event, participation, index, spent.Output().Deposit(), mutations); err != nil {
				mutations.Cancel()
				return err
			}
		case *Staking:
			// Decrease the staked amount
			if err := pm.decreaseStakedAmountForStakingEvent(participation.EventID, index, spent.Output().Deposit(), mutations); err != nil {
				mutations.Cancel()
				return err
			}
			// Decrease the staking rewards
			if err := pm.decreaseCurrentRewardsPerMilestoneForStakingEvent(participation.EventID, index, payload.rewardsPerMilestone(spent.Output().Deposit()), mutations); err != nil {
				mutations.Cancel()
				return err
			}
		}
	}

	return mutations.Commit()
}

// applyNewConfirmedMilestoneIndexForEvents iterates over each counting ballot participation and applies the current vote balance for each question to the total vote balance
func (pm *ParticipationManager) applyNewConfirmedMilestoneIndexForEvents(index milestone.Index, events map[EventID]*Event) error {

	mutations, err := pm.participationStore.Batched()
	if err != nil {
		return err
	}

	// Iterate over all known events and increase the one that are currently counting
	for eventID, event := range events {
		shouldCountParticipation := event.ShouldCountParticipation(index)

		processAnswerValueBalances := func(questionIndex uint8, answerValue uint8) error {

			// Read the accumulated value from the previous milestone, add the current vote and store accumulated for this milestone

			currentBalance, err := pm.CurrentBallotVoteBalanceForQuestionAndAnswer(eventID, index, questionIndex, answerValue)
			if err != nil {
				mutations.Cancel()
				return err
			}

			if event.EndMilestoneIndex() > index {
				// Event not ended yet, so copy the current for the next milestone already
				if err := setCurrentBallotVoteBalanceForQuestionAndAnswer(eventID, index+1, questionIndex, answerValue, currentBalance, mutations); err != nil {
					mutations.Cancel()
					return err
				}
			}

			if shouldCountParticipation {
				accumulatedBalance, err := pm.AccumulatedBallotVoteBalanceForQuestionAndAnswer(eventID, index-1, questionIndex, answerValue)
				if err != nil {
					mutations.Cancel()
					return err
				}

				// Add current vote balance to accumulated vote balance for each answer
				newAccumulatedBalance := accumulatedBalance + currentBalance

				if err := setAccumulatedBallotVoteBalanceForQuestionAndAnswer(eventID, index, questionIndex, answerValue, newAccumulatedBalance, mutations); err != nil {
					mutations.Cancel()
					return err
				}
			}

			return nil
		}

		// For each participation, iterate over all questions
		for idx, question := range event.BallotQuestions() {
			questionIndex := uint8(idx)

			// For each question, iterate over all answers values
			for _, answer := range question.QuestionAnswers() {
				if err := processAnswerValueBalances(questionIndex, answer.Value); err != nil {
					return err
				}
			}
			if err := processAnswerValueBalances(questionIndex, AnswerValueSkipped); err != nil {
				return err
			}
			if err := processAnswerValueBalances(questionIndex, AnswerValueInvalid); err != nil {
				return err
			}
		}

		staking := event.Staking()
		if staking != nil {

			currentRewardsPerMilestone, err := pm.currentRewardsPerMilestoneForStakingEvent(eventID, index)
			if err != nil {
				mutations.Cancel()
				return err
			}

			if event.EndMilestoneIndex() > index {
				// Event not ended yet, so copy the current rewards for the next milestone already
				if err := pm.setCurrentRewardsPerMilestoneForStakingEvent(eventID, index+1, currentRewardsPerMilestone, mutations); err != nil {
					mutations.Cancel()
					return err
				}
			}

			total, err := pm.totalStakingParticipationForEvent(eventID, index)
			if err != nil {
				mutations.Cancel()
				return err
			}

			if shouldCountParticipation {
				total.rewarded += currentRewardsPerMilestone
			}

			if err := pm.setTotalStakingParticipationForEvent(eventID, index, total, mutations); err != nil {
				mutations.Cancel()
				return err
			}

			if event.EndMilestoneIndex() > index {
				// Event not ended yet, so copy the current total for the next milestone already
				if err := pm.setTotalStakingParticipationForEvent(eventID, index+1, total, mutations); err != nil {
					mutations.Cancel()
					return err
				}
			}
		}

		// End all participation if event is ending this milestone
		if event.EndMilestoneIndex() == index {
			if err := pm.endAllParticipationsAtMilestone(eventID, index+1, mutations); err != nil {
				mutations.Cancel()
				return err
			}
		}
	}

	return mutations.Commit()
}

func filterValidParticipationsForEvents(index milestone.Index, votes []*Participation, events map[EventID]*Event) []*Participation {

	var validParticipations []*Participation
	for _, vote := range votes {

		// Check that we want to handle the event for the given participation
		event, found := events[vote.EventID]
		if !found {
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

func participationFromTaggedData(taggedData *iotago.TaggedData) ([]*Participation, error) {

	// try to parse the votes payload
	parsedVotes := &ParticipationPayload{}
	if _, err := parsedVotes.Deserialize(taggedData.Data, serializer.DeSeriModePerformValidation, nil); err != nil {
		// votes payload can't be parsed => ignore votes
		return nil, fmt.Errorf("no valid votes payload")
	}

	var votes []*Participation
	for _, vote := range parsedVotes.Participations {
		votes = append(votes, vote)
	}

	return votes, nil
}

func serializedAddressFromOutput(output *iotago.BasicOutput) ([]byte, error) {
	unlockConditions, err := output.UnlockConditions().Set()
	if err != nil {
		return nil, err
	}

	outputAddress, err := unlockConditions.Address().Address.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return nil, err
	}
	return outputAddress, nil
}

func (pm *ParticipationManager) ParticipationsFromMessage(msg *storage.Message, msIndex milestone.Index) (*utxo.Output, []*Participation, error) {
	transaction := msg.Transaction()
	if transaction == nil {
		// Do not handle outputs from migrations
		// This output was created by a migration in a milestone payload.
		return nil, nil, nil
	}

	txEssence := msg.TransactionEssence()
	if txEssence == nil {
		// if the message was included, there must be a transaction payload essence
		return nil, nil, fmt.Errorf("no transaction transactionEssence found: MsgID: %s", msg.MessageID().ToHex())
	}

	txEssenceTaggedData := msg.TransactionEssenceTaggedData()
	if txEssenceTaggedData == nil {
		// no need to check if there is not taggedData payload
		return nil, nil, nil
	}

	// the tag of the transaction payload must match our configured tag
	if !bytes.Equal(txEssenceTaggedData.Tag, pm.opts.tagMessage) {
		return nil, nil, nil
	}

	// collect outputs
	depositOutputs := utxo.Outputs{}
	for i := 0; i < len(txEssence.Outputs); i++ {
		output, err := utxo.NewOutput(msg.MessageID(), msIndex, 0, transaction, uint16(i))
		if err != nil {
			return nil, nil, err
		}
		depositOutputs = append(depositOutputs, output)
	}

	// only a single output is allowed
	if len(depositOutputs) != 1 {
		return nil, nil, nil
	}

	// only ExtendedOutput are allowed as output type
	var depositOutput *iotago.BasicOutput
	switch o := depositOutputs[0].Output().(type) {
	case *iotago.BasicOutput:
		depositOutput = o
	default:
		return nil, nil, nil
	}

	outputAddress, err := serializedAddressFromOutput(depositOutput)
	if err != nil {
		return nil, nil, nil
	}

	// collect inputs
	inputOutputs := utxo.Outputs{}
	for _, input := range msg.TransactionEssenceUTXOInputs() {
		output, err := pm.storage.UTXOManager().ReadOutputByOutputIDWithoutLocking(input)
		if err != nil {
			return nil, nil, err
		}
		inputOutputs = append(inputOutputs, output)
	}

	// check if at least 1 input comes from the same address as the output
	containsInputFromSameAddress := false
	for _, input := range inputOutputs {
		switch output := input.Output().(type) {
		case *iotago.BasicOutput:
			inputAddress, err := serializedAddressFromOutput(output)
			if err != nil {
				return nil, nil, nil
			}

			if bytes.Equal(outputAddress, inputAddress) {
				containsInputFromSameAddress = true
				break
			}
		default:
			continue
		}
	}

	if !containsInputFromSameAddress {
		// no input address match the output address =>  not a valid voting transaction
		return nil, nil, nil
	}

	participations, err := participationFromTaggedData(txEssenceTaggedData)
	if err != nil {
		return nil, nil, nil
	}

	return depositOutputs[0], participations, nil
}

func filterEvents(events map[EventID]*Event, index milestone.Index, includeFunc func(e *Event, index milestone.Index) bool) map[EventID]*Event {
	filtered := make(map[EventID]*Event)
	for id, event := range events {
		if includeFunc(event, index) {
			filtered[id] = event
		}
	}
	return filtered
}
