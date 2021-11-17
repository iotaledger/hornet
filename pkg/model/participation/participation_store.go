package participation

import (
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer"
	iotago "github.com/iotaledger/iota.go/v2"
)

var (
	ErrUnknownParticipation                  = errors.New("no participation found")
	ErrEventNotFound                         = errors.New("referenced event does not exist")
	ErrInvalidEvent                          = errors.New("invalid event")
	ErrInvalidPreviouslyTrackedParticipation = errors.New("a previously tracked participation changed and is now invalid")
	ErrInvalidCurrentBallotVoteBalance       = errors.New("current ballot vote balance invalid")
)

// Events

func eventKeyForEventID(eventID EventID) []byte {
	m := marshalutil.New(33)
	m.WriteByte(ParticipationStoreKeyPrefixEvents) // 1 byte
	m.WriteBytes(eventID[:])                       // 32 bytes
	return m.Bytes()
}

func (pm *ParticipationManager) loadEvents() (map[EventID]*Event, error) {

	events := make(map[EventID]*Event)

	var innerErr error
	if err := pm.participationStore.Iterate(kvstore.KeyPrefix{ParticipationStoreKeyPrefixEvents}, func(key kvstore.Key, value kvstore.Value) bool {

		eventID := EventID{}
		copy(eventID[:], key[1:]) // Skip the prefix

		event := &Event{}
		_, innerErr = event.Deserialize(value, serializer.DeSeriModeNoValidation)
		if innerErr != nil {
			return false
		}

		events[eventID] = event
		return true
	}); err != nil {
		return nil, err
	}

	if innerErr != nil {
		return nil, innerErr
	}

	return events, nil
}

func (pm *ParticipationManager) storeEvent(event *Event) (EventID, error) {

	eventBytes, err := event.Serialize(serializer.DeSeriModePerformValidation)
	if err != nil {
		return NullEventID, err
	}

	eventID, err := event.ID()
	if err != nil {
		return NullEventID, err
	}

	if err := pm.participationStore.Set(eventKeyForEventID(eventID), eventBytes); err != nil {
		return NullEventID, err
	}

	return eventID, nil
}

func (pm *ParticipationManager) deleteEvent(eventID EventID) error {
	return pm.participationStore.Delete(eventKeyForEventID(eventID))
}

// Messages

func messageKeyForMessageID(messageID hornet.MessageID) []byte {
	m := marshalutil.New(33)
	m.WriteByte(ParticipationStoreKeyPrefixMessages) // 1 byte
	m.WriteBytes(messageID)                          // 32 bytes
	return m.Bytes()
}

func (pm *ParticipationManager) storeMessage(message *storage.Message, mutations kvstore.BatchedMutations) error {
	return mutations.Set(messageKeyForMessageID(message.MessageID()), message.Data())
}

func (pm *ParticipationManager) MessageForMessageID(messageId hornet.MessageID) (*storage.Message, error) {
	value, err := pm.participationStore.Get(messageKeyForMessageID(messageId))
	if errors.Is(err, kvstore.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return storage.MessageFromBytes(value, serializer.DeSeriModeNoValidation)
}

// Outputs

func participationKeyForEventOutputsPrefix(eventID EventID) []byte {
	m := marshalutil.New(33)
	m.WriteByte(ParticipationStoreKeyPrefixTrackedOutputs) // 1 byte
	m.WriteBytes(eventID[:])                               // 32 bytes
	return m.Bytes()
}

func participationKeyForEventAndOutputID(eventID EventID, outputID *iotago.UTXOInputID) []byte {
	m := marshalutil.New(67)
	m.WriteBytes(participationKeyForEventOutputsPrefix(eventID)) // 32 bytes
	m.WriteBytes(outputID[:])                                    // 34 bytes
	return m.Bytes()
}

func participationKeyForEventSpentOutputsPrefix(eventID EventID) []byte {
	m := marshalutil.New(33)
	m.WriteByte(ParticipationStoreKeyPrefixTrackedSpentOutputs) // 1 byte
	m.WriteBytes(eventID[:])                                    // 32 bytes
	return m.Bytes()
}

func participationKeyForEventAndSpentOutputID(eventID EventID, outputID *iotago.UTXOInputID) []byte {
	m := marshalutil.New(67)
	m.WriteBytes(participationKeyForEventSpentOutputsPrefix(eventID)) // 33 bytes
	m.WriteBytes(outputID[:])                                         // 34 bytes
	return m.Bytes()
}

func (pm *ParticipationManager) ParticipationsForOutputID(outputID *iotago.UTXOInputID) ([]*TrackedParticipation, error) {
	eventIDs := pm.EventIDs()
	trackedParticipations := []*TrackedParticipation{}
	for _, eventID := range eventIDs {
		participation, err := pm.ParticipationForOutputID(eventID, outputID)
		if err != nil {
			if errors.Is(err, ErrUnknownParticipation) {
				continue
			}
			return nil, err
		}
		trackedParticipations = append(trackedParticipations, participation)
	}
	return trackedParticipations, nil
}

func (pm *ParticipationManager) ParticipationForOutputID(eventID EventID, outputID *iotago.UTXOInputID) (*TrackedParticipation, error) {
	readOutput := func(eventID EventID, outputID *iotago.UTXOInputID) (kvstore.Key, kvstore.Value, error) {
		key := participationKeyForEventAndOutputID(eventID, outputID)
		value, err := pm.participationStore.Get(key)
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, nil, ErrUnknownParticipation
		}
		if err != nil {
			return nil, nil, err
		}
		return key, value, nil
	}

	readSpent := func(eventID EventID, outputID *iotago.UTXOInputID) (kvstore.Key, kvstore.Value, error) {
		key := participationKeyForEventAndSpentOutputID(eventID, outputID)
		value, err := pm.participationStore.Get(key)
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, nil, ErrUnknownParticipation
		}
		if err != nil {
			return nil, nil, err
		}
		return key, value, nil
	}

	var key kvstore.Key
	var value kvstore.Value
	var err error

	key, value, err = readOutput(eventID, outputID)
	if errors.Is(err, ErrUnknownParticipation) {
		key, value, err = readSpent(eventID, outputID)
	}

	if err != nil {
		return nil, err
	}

	return TrackedParticipationFromBytes(key, value)
}

type IterateOptions struct {
	maxResultCount int
}

type IterateOption func(*IterateOptions)

func MaxResultCount(count int) IterateOption {
	return func(args *IterateOptions) {
		args.maxResultCount = count
	}
}

func iterateOptions(optionalOptions []IterateOption) *IterateOptions {
	result := &IterateOptions{
		maxResultCount: 0,
	}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}
	return result
}

type TrackedParticipationConsumer func(trackedParticipation *TrackedParticipation) bool

func (pm *ParticipationManager) ForEachActiveParticipation(eventID EventID, consumer TrackedParticipationConsumer, options ...IterateOption) error {
	opt := iterateOptions(options)
	consumerFunc := consumer

	var innerErr error
	var i int
	if err := pm.participationStore.Iterate(participationKeyForEventOutputsPrefix(eventID), func(key kvstore.Key, value kvstore.Value) bool {

		if (opt.maxResultCount > 0) && (i >= opt.maxResultCount) {
			return false
		}

		i++

		participation, err := TrackedParticipationFromBytes(key, value)
		if err != nil {
			innerErr = err
			return false
		}

		return consumerFunc(participation)
	}); err != nil {
		return err
	}

	return innerErr
}

func (pm *ParticipationManager) ForEachPastParticipation(eventID EventID, consumer TrackedParticipationConsumer, options ...IterateOption) error {
	opt := iterateOptions(options)
	consumerFunc := consumer

	var innerErr error
	var i int
	if err := pm.participationStore.Iterate(participationKeyForEventSpentOutputsPrefix(eventID), func(key kvstore.Key, value kvstore.Value) bool {

		if (opt.maxResultCount > 0) && (i >= opt.maxResultCount) {
			return false
		}

		i++

		participation, err := TrackedParticipationFromBytes(key, value)
		if err != nil {
			innerErr = err
			return false
		}

		return consumerFunc(participation)
	}); err != nil {
		return err
	}

	return innerErr
}

// Ballot answers

func currentBallotVoteBalanceKeyForQuestionAndAnswer(eventID EventID, milestone milestone.Index, questionIndex uint8, answerIndex uint8) []byte {
	m := marshalutil.New(39)
	m.WriteByte(ParticipationStoreKeyPrefixBallotCurrentVoteBalanceForQuestionAndAnswer) // 1 byte
	m.WriteBytes(eventID[:])                                                             // 32 bytes
	m.WriteUint32(uint32(milestone))                                                     // 4 bytes
	m.WriteUint8(questionIndex)                                                          // 1 byte
	m.WriteUint8(answerIndex)                                                            // 1 byte
	return m.Bytes()
}

func accumulatedBallotVoteBalanceKeyForQuestionAndAnswer(eventID EventID, milestone milestone.Index, questionIndex uint8, answerIndex uint8) []byte {
	m := marshalutil.New(39)
	m.WriteByte(ParticipationStoreKeyPrefixBallotAccululatedVoteBalanceForQuestionAndAnswer) // 1 byte
	m.WriteBytes(eventID[:])                                                                 // 32 bytes
	m.WriteUint32(uint32(milestone))                                                         // 4 bytes
	m.WriteUint8(questionIndex)                                                              // 1 byte
	m.WriteUint8(answerIndex)                                                                // 1 byte
	return m.Bytes()
}

func (pm *ParticipationManager) startParticipationAtMilestone(eventID EventID, output *utxo.Output, startIndex milestone.Index, mutations kvstore.BatchedMutations) error {
	trackedVote := &TrackedParticipation{
		EventID:    eventID,
		OutputID:   output.OutputID(),
		MessageID:  output.MessageID(),
		Amount:     output.Amount(),
		StartIndex: startIndex,
		EndIndex:   0,
	}
	return mutations.Set(participationKeyForEventAndOutputID(eventID, output.OutputID()), trackedVote.ValueBytes())
}

func (pm *ParticipationManager) endParticipationAtMilestone(eventID EventID, output *utxo.Output, endIndex milestone.Index, mutations kvstore.BatchedMutations) error {
	key := participationKeyForEventAndOutputID(eventID, output.OutputID())

	value, err := pm.participationStore.Get(key)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return ErrUnknownParticipation
		}
		return err
	}

	participation, err := TrackedParticipationFromBytes(key, value)
	if err != nil {
		return err
	}

	participation.EndIndex = endIndex

	// Delete the entry from the Outputs list
	if err := mutations.Delete(key); err != nil {
		return err
	}

	// Add the entry to the Spent list
	return mutations.Set(participationKeyForEventAndSpentOutputID(eventID, output.OutputID()), participation.ValueBytes())
}

func (pm *ParticipationManager) endAllParticipationsAtMilestone(eventID EventID, endIndex milestone.Index, mutations kvstore.BatchedMutations) error {
	var innerErr error
	if err := pm.participationStore.Iterate(participationKeyForEventOutputsPrefix(eventID), func(key kvstore.Key, value kvstore.Value) bool {

		participation, err := TrackedParticipationFromBytes(key, value)
		if err != nil {
			innerErr = err
			return false
		}

		participation.EndIndex = endIndex

		// Delete the entry from the Outputs list
		if err := mutations.Delete(key); err != nil {
			innerErr = err
			return false
		}

		// Add the entry to the Spent list
		if err := mutations.Set(participationKeyForEventAndSpentOutputID(eventID, participation.OutputID), participation.ValueBytes()); err != nil {
			innerErr = err
			return false
		}

		return true

	}); err != nil {
		return err
	}

	return innerErr
}

func (pm *ParticipationManager) CurrentBallotVoteBalanceForQuestionAndAnswer(eventID EventID, milestone milestone.Index, questionIdx uint8, answerIdx uint8) (uint64, error) {
	val, err := pm.participationStore.Get(currentBallotVoteBalanceKeyForQuestionAndAnswer(eventID, milestone, questionIdx, answerIdx))

	if errors.Is(err, kvstore.ErrKeyNotFound) {
		// No votes for this answer yet
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	ms := marshalutil.New(val)
	return ms.ReadUint64()
}

func (pm *ParticipationManager) AccumulatedBallotVoteBalanceForQuestionAndAnswer(eventID EventID, milestone milestone.Index, questionIdx uint8, answerIdx uint8) (uint64, error) {
	val, err := pm.participationStore.Get(accumulatedBallotVoteBalanceKeyForQuestionAndAnswer(eventID, milestone, questionIdx, answerIdx))

	if errors.Is(err, kvstore.ErrKeyNotFound) {
		// No votes for this answer yet
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	ms := marshalutil.New(val)
	return ms.ReadUint64()
}

func setCurrentBallotVoteBalanceForQuestionAndAnswer(eventID EventID, milestone milestone.Index, questionIdx uint8, answerIdx uint8, current uint64, mutations kvstore.BatchedMutations) error {
	ms := marshalutil.New(8)
	ms.WriteUint64(current)
	return mutations.Set(currentBallotVoteBalanceKeyForQuestionAndAnswer(eventID, milestone, questionIdx, answerIdx), ms.Bytes())
}

func setAccumulatedBallotVoteBalanceForQuestionAndAnswer(eventID EventID, milestone milestone.Index, questionIdx uint8, answerIdx uint8, total uint64, mutations kvstore.BatchedMutations) error {
	ms := marshalutil.New(8)
	ms.WriteUint64(total)
	return mutations.Set(accumulatedBallotVoteBalanceKeyForQuestionAndAnswer(eventID, milestone, questionIdx, answerIdx), ms.Bytes())
}

func (pm *ParticipationManager) startCountingBallotAnswers(vote *Participation, milestone milestone.Index, amount uint64, mutations kvstore.BatchedMutations) error {
	event := pm.Event(vote.EventID)
	if event == nil {
		return nil
	}
	if event.payloadType() == BallotPayloadTypeID {
		questions := event.BallotQuestions()
		for idx, answerByte := range vote.Answers {
			questionIndex := uint8(idx)
			// We already verified, that there are exactly as many answers as questions in the ballot, so no need to check here again
			answerValue := questions[idx].answerValueForByte(answerByte)

			currentVoteBalance, err := pm.CurrentBallotVoteBalanceForQuestionAndAnswer(vote.EventID, milestone, questionIndex, answerValue)
			if err != nil {
				return err
			}

			voteCount := amount / 1000
			currentVoteBalance += voteCount

			if err := setCurrentBallotVoteBalanceForQuestionAndAnswer(vote.EventID, milestone, questionIndex, answerValue, currentVoteBalance, mutations); err != nil {
				return err
			}
		}
	}
	return nil
}

func (pm *ParticipationManager) stopCountingBallotAnswers(vote *Participation, milestone milestone.Index, amount uint64, mutations kvstore.BatchedMutations) error {
	event := pm.Event(vote.EventID)
	if event == nil {
		return nil
	}
	if event.payloadType() == BallotPayloadTypeID {
		questions := event.BallotQuestions()
		for idx, answerByte := range vote.Answers {
			questionIndex := uint8(idx)
			// We already verified, that there are exactly as many answers as questions in the ballot, so no need to check here again
			answerValue := questions[idx].answerValueForByte(answerByte)

			currentVoteBalance, err := pm.CurrentBallotVoteBalanceForQuestionAndAnswer(vote.EventID, milestone, questionIndex, answerValue)
			if err != nil {
				return err
			}

			voteCount := amount / 1000
			if currentVoteBalance < voteCount {
				// currentVoteBalance can't be less than 0
				return ErrInvalidCurrentBallotVoteBalance
			}
			currentVoteBalance -= voteCount

			if err := setCurrentBallotVoteBalanceForQuestionAndAnswer(vote.EventID, milestone, questionIndex, answerValue, currentVoteBalance, mutations); err != nil {
				return err
			}
		}
	}
	return nil
}

// Staking

func stakingKeyForEventAndAddress(eventID EventID, addressBytes []byte) []byte {
	m := marshalutil.New(66)
	m.WriteByte(ParticipationStoreKeyPrefixStakingAddress) // 1 byte
	m.WriteBytes(eventID[:])                               // 32 bytes
	m.WriteBytes(addressBytes)                             // 33 bytes
	return m.Bytes()
}

func (pm *ParticipationManager) StakingRewardForAddress(eventID EventID, address iotago.Address) (uint64, error) {

	addressBytes, err := address.Serialize(serializer.DeSeriModeNoValidation)
	if err != nil {
		return 0, err
	}

	return pm.stakingRewardForEventAndAddress(eventID, addressBytes)
}

func (pm *ParticipationManager) stakingRewardForEventAndAddress(eventID EventID, addressBytes []byte) (uint64, error) {
	key := stakingKeyForEventAndAddress(eventID, addressBytes)
	value, err := pm.participationStore.Get(key)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return 0, nil
		}
		return 0, err
	}
	m := marshalutil.New(value)
	balance, err := m.ReadUint64()
	if err != nil {
		return 0, err
	}
	return balance, err
}

func (pm *ParticipationManager) increaseStakingRewardForEventAndAddress(eventID EventID, addressBytes []byte, amountToIncrease uint64, mutations kvstore.BatchedMutations) error {
	balance, err := pm.stakingRewardForEventAndAddress(eventID, addressBytes)
	if err != nil {
		return err
	}

	newBalance := balance + amountToIncrease
	m := marshalutil.New(8)
	m.WriteUint64(newBalance)

	return mutations.Set(stakingKeyForEventAndAddress(eventID, addressBytes), m.Bytes())
}
