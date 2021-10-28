package referendum

import (
	"errors"

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
	ErrUnknownVote                  = errors.New("no vote found")
	ErrReferendumNotFound           = errors.New("referenced referendum does not exist")
	ErrInvalidReferendum            = errors.New("invalid referendum")
	ErrInvalidPreviouslyTrackedVote = errors.New("a previously tracked vote changed and is now invalid")
	ErrInvalidCurrentVoteBalance    = errors.New("current vote balance invalid")
)

// Referendums

func referendumKeyForReferendumID(referendumID ReferendumID) []byte {
	m := marshalutil.New(33)
	m.WriteByte(ReferendumStoreKeyPrefixReferendums) // 1 byte
	m.WriteBytes(referendumID[:])                    // 32 bytes
	return m.Bytes()
}

func (rm *ReferendumManager) loadReferendums() (map[ReferendumID]*Referendum, error) {

	referendums := make(map[ReferendumID]*Referendum)

	var innerErr error
	if err := rm.referendumStore.Iterate(kvstore.KeyPrefix{ReferendumStoreKeyPrefixReferendums}, func(key kvstore.Key, value kvstore.Value) bool {

		referendumID := ReferendumID{}
		copy(referendumID[:], key[1:]) // Skip the prefix

		referendum := &Referendum{}
		_, innerErr = referendum.Deserialize(value, serializer.DeSeriModeNoValidation)
		if innerErr != nil {
			return false
		}

		referendums[referendumID] = referendum
		return true
	}); err != nil {
		return nil, err
	}

	if innerErr != nil {
		return nil, innerErr
	}

	return referendums, nil
}

func (rm *ReferendumManager) storeReferendum(referendum *Referendum) (ReferendumID, error) {

	referendumBytes, err := referendum.Serialize(serializer.DeSeriModePerformValidation)
	if err != nil {
		return NullReferendumID, err
	}

	referendumID, err := referendum.ID()
	if err != nil {
		return NullReferendumID, err
	}

	if err := rm.referendumStore.Set(referendumKeyForReferendumID(referendumID), referendumBytes); err != nil {
		return NullReferendumID, err
	}

	return referendumID, nil
}

func (rm *ReferendumManager) deleteReferendum(referendumID ReferendumID) error {
	return rm.referendumStore.Delete(referendumKeyForReferendumID(referendumID))
}

// Messages

func messageKeyForMessageID(messageID hornet.MessageID) []byte {
	m := marshalutil.New(33)
	m.WriteByte(ReferendumStoreKeyPrefixMessages) // 1 byte
	m.WriteBytes(messageID)                       // 32 bytes
	return m.Bytes()
}

func (rm *ReferendumManager) storeMessage(message *storage.Message, mutations kvstore.BatchedMutations) error {
	return mutations.Set(messageKeyForMessageID(message.MessageID()), message.Data())
}

func (rm *ReferendumManager) MessageForMessageID(messageId hornet.MessageID) (*storage.Message, error) {
	value, err := rm.referendumStore.Get(messageKeyForMessageID(messageId))
	if errors.Is(err, kvstore.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return storage.MessageFromBytes(value, serializer.DeSeriModeNoValidation)
}

// Outputs

func voteKeyForReferendumIDOutputsPrefix(referendumID ReferendumID) []byte {
	m := marshalutil.New(33)
	m.WriteByte(ReferendumStoreKeyPrefixReferendumOutputs) // 1 byte
	m.WriteBytes(referendumID[:])                          // 32 bytes
	return m.Bytes()
}

func voteKeyForReferendumIDAndOutputID(referendumID ReferendumID, outputID *iotago.UTXOInputID) []byte {
	m := marshalutil.New(67)
	m.WriteBytes(voteKeyForReferendumIDOutputsPrefix(referendumID)) // 32 bytes
	m.WriteBytes(outputID[:])                                       // 34 bytes
	return m.Bytes()
}

func voteKeyForReferendumIDSpentOutputsPrefix(referendumID ReferendumID) []byte {
	m := marshalutil.New(33)
	m.WriteByte(ReferendumStoreKeyPrefixReferendumSpentOutputs) // 1 byte
	m.WriteBytes(referendumID[:])                               // 32 bytes
	return m.Bytes()
}

func voteKeyForReferendumIDAndSpentOutputID(referendumID ReferendumID, outputID *iotago.UTXOInputID) []byte {
	m := marshalutil.New(67)
	m.WriteBytes(voteKeyForReferendumIDSpentOutputsPrefix(referendumID)) // 33 bytes
	m.WriteBytes(outputID[:])                                            // 34 bytes
	return m.Bytes()
}

type TrackedVote struct {
	ReferendumID ReferendumID
	OutputID     *iotago.UTXOInputID
	MessageID    hornet.MessageID
	Amount       uint64
	StartIndex   milestone.Index
	EndIndex     milestone.Index
}

func ParseReferendumID(ms *marshalutil.MarshalUtil) (ReferendumID, error) {
	bytes, err := ms.ReadBytes(ReferendumIDLength)
	if err != nil {
		return NullReferendumID, err
	}
	o := ReferendumID{}
	copy(o[:], bytes)
	return o, nil
}

func trackedVote(key []byte, value []byte) (*TrackedVote, error) {

	if len(key) != 67 {
		return nil, ErrInvalidPreviouslyTrackedVote
	}

	if len(value) != 48 {
		return nil, ErrInvalidPreviouslyTrackedVote
	}

	mKey := marshalutil.New(key)

	// Skip prefix
	if _, err := mKey.ReadByte(); err != nil {
		return nil, err
	}

	// Read ReferendumID
	referendumID, err := ParseReferendumID(mKey)
	if err != nil {
		return nil, err
	}

	// Read OutputID
	outputID, err := utxo.ParseOutputID(mKey)
	if err != nil {
		return nil, err
	}

	mValue := marshalutil.New(value)

	messageID, err := utxo.ParseMessageID(mValue)
	if err != nil {
		return nil, err
	}

	amount, err := mValue.ReadUint64()
	if err != nil {
		return nil, err
	}

	start, err := mValue.ReadUint32()
	if err != nil {
		return nil, err
	}

	end, err := mValue.ReadUint32()
	if err != nil {
		return nil, err
	}

	return &TrackedVote{
		ReferendumID: referendumID,
		OutputID:     outputID,
		MessageID:    messageID,
		Amount:       amount,
		StartIndex:   milestone.Index(start),
		EndIndex:     milestone.Index(end),
	}, nil
}

func (t *TrackedVote) valueBytes() []byte {
	m := marshalutil.New(48)
	m.WriteBytes(t.MessageID) // 32 bytes
	m.WriteUint64(t.Amount)
	m.WriteUint32(uint32(t.StartIndex)) // 4 bytes
	m.WriteUint32(uint32(t.EndIndex))   // 4 bytes
	return m.Bytes()
}

func (rm *ReferendumManager) VotesForOutputID(outputID *iotago.UTXOInputID) ([]*TrackedVote, error) {

	referendumIDs := rm.ReferendumIDs()

	trackedVotes := []*TrackedVote{}
	for _, referendumID := range referendumIDs {
		vote, err := rm.VoteForOutputID(referendumID, outputID)
		if err != nil {
			if errors.Is(err, ErrUnknownVote) {
				continue
			}
			return nil, err
		}
		trackedVotes = append(trackedVotes, vote)
	}

	return trackedVotes, nil
}

func (rm *ReferendumManager) VoteForOutputID(referendumID ReferendumID, outputID *iotago.UTXOInputID) (*TrackedVote, error) {

	readOutput := func(referendumID ReferendumID, outputID *iotago.UTXOInputID) (kvstore.Key, kvstore.Value, error) {
		key := voteKeyForReferendumIDAndOutputID(referendumID, outputID)
		value, err := rm.referendumStore.Get(key)
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, nil, ErrUnknownVote
		}
		if err != nil {
			return nil, nil, err
		}
		return key, value, nil
	}

	readSpent := func(referendumID ReferendumID, outputID *iotago.UTXOInputID) (kvstore.Key, kvstore.Value, error) {
		key := voteKeyForReferendumIDAndSpentOutputID(referendumID, outputID)
		value, err := rm.referendumStore.Get(key)
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, nil, ErrUnknownVote
		}
		if err != nil {
			return nil, nil, err
		}
		return key, value, nil
	}

	var key kvstore.Key
	var value kvstore.Value
	var err error

	key, value, err = readOutput(referendumID, outputID)
	if errors.Is(err, ErrUnknownVote) {
		key, value, err = readSpent(referendumID, outputID)
	}

	if err != nil {
		return nil, err
	}

	return trackedVote(key, value)
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

type TrackedVoteConsumer func(trackedVote *TrackedVote) bool

func (rm *ReferendumManager) ForEachActiveVote(referendumID ReferendumID, consumer TrackedVoteConsumer, options ...IterateOption) error {

	opt := iterateOptions(options)

	consumerFunc := consumer

	var innerErr error
	var i int
	if err := rm.referendumStore.Iterate(voteKeyForReferendumIDOutputsPrefix(referendumID), func(key kvstore.Key, value kvstore.Value) bool {

		if (opt.maxResultCount > 0) && (i >= opt.maxResultCount) {
			return false
		}

		i++

		trackedVote, err := trackedVote(key, value)
		if err != nil {
			innerErr = err
			return false
		}

		return consumerFunc(trackedVote)
	}); err != nil {
		return err
	}

	return innerErr
}

func (rm *ReferendumManager) ForEachPastVote(referendumID ReferendumID, consumer TrackedVoteConsumer, options ...IterateOption) error {

	opt := iterateOptions(options)

	consumerFunc := consumer

	var innerErr error
	var i int
	if err := rm.referendumStore.Iterate(voteKeyForReferendumIDSpentOutputsPrefix(referendumID), func(key kvstore.Key, value kvstore.Value) bool {

		if (opt.maxResultCount > 0) && (i >= opt.maxResultCount) {
			return false
		}

		i++

		trackedVote, err := trackedVote(key, value)
		if err != nil {
			innerErr = err
			return false
		}

		return consumerFunc(trackedVote)
	}); err != nil {
		return err
	}

	return innerErr
}

// Votes

func currentVoteBalanceKeyForQuestionAndAnswer(referendumID ReferendumID, questionIndex uint8, answerIndex uint8) []byte {
	m := marshalutil.New(35)
	m.WriteByte(ReferendumStoreKeyPrefixCurrentVoteBalanceForQuestionAndAnswer) // 1 byte
	m.WriteBytes(referendumID[:])                                               // 32 bytes
	m.WriteUint8(questionIndex)                                                 // 1 byte
	m.WriteUint8(answerIndex)                                                   // 1 byte
	return m.Bytes()
}

func totalVoteBalanceKeyForQuestionAndAnswer(referendumID ReferendumID, questionIndex uint8, answerIndex uint8) []byte {
	ms := marshalutil.New(35)
	ms.WriteByte(ReferendumStoreKeyPrefixTotalVoteBalanceForQuestionAndAnswer) // 1 byte
	ms.WriteBytes(referendumID[:])                                             // 32 bytes
	ms.WriteUint8(questionIndex)                                               // 1 byte
	ms.WriteUint8(answerIndex)                                                 // 1 byte
	return ms.Bytes()
}

func (rm *ReferendumManager) startVoteAtMilestone(referendumID ReferendumID, output *utxo.Output, startIndex milestone.Index, mutations kvstore.BatchedMutations) error {

	trackedVote := &TrackedVote{
		ReferendumID: referendumID,
		OutputID:     output.OutputID(),
		MessageID:    output.MessageID(),
		Amount:       output.Amount(),
		StartIndex:   startIndex,
		EndIndex:     0,
	}

	return mutations.Set(voteKeyForReferendumIDAndOutputID(referendumID, output.OutputID()), trackedVote.valueBytes())
}

func (rm *ReferendumManager) endVoteAtMilestone(referendumID ReferendumID, output *utxo.Output, endIndex milestone.Index, mutations kvstore.BatchedMutations) error {

	key := voteKeyForReferendumIDAndOutputID(referendumID, output.OutputID())

	value, err := rm.referendumStore.Get(key)
	if err != nil {
		return err
	}

	trackedVote, err := trackedVote(key, value)
	if err != nil {
		return err
	}

	trackedVote.EndIndex = endIndex

	// Delete the entry from the Outputs list
	if err := mutations.Delete(key); err != nil {
		return err
	}

	// Add the entry to the Spent list
	return mutations.Set(voteKeyForReferendumIDAndSpentOutputID(referendumID, output.OutputID()), trackedVote.valueBytes())
}

func (rm *ReferendumManager) endAllVotesAtMilestone(referendumID ReferendumID, endIndex milestone.Index, mutations kvstore.BatchedMutations) error {

	var innerErr error
	if err := rm.referendumStore.Iterate(voteKeyForReferendumIDOutputsPrefix(referendumID), func(key kvstore.Key, value kvstore.Value) bool {

		trackedVote, err := trackedVote(key, value)
		if err != nil {
			innerErr = err
			return false
		}

		trackedVote.EndIndex = endIndex

		// Delete the entry from the Outputs list
		if err := mutations.Delete(key); err != nil {
			innerErr = err
			return false
		}

		// Add the entry to the Spent list
		if err := mutations.Set(voteKeyForReferendumIDAndSpentOutputID(referendumID, trackedVote.OutputID), trackedVote.valueBytes()); err != nil {
			innerErr = err
			return false
		}

		return true

	}); err != nil {
		return err
	}

	return innerErr
}

func (rm *ReferendumManager) CurrentBalanceForReferendum(referendumID ReferendumID, questionIdx uint8, answerIdx uint8) (uint64, error) {

	val, err := rm.referendumStore.Get(currentVoteBalanceKeyForQuestionAndAnswer(referendumID, questionIdx, answerIdx))

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

func (rm *ReferendumManager) TotalBalanceForReferendum(referendumID ReferendumID, questionIdx uint8, answerIdx uint8) (uint64, error) {

	val, err := rm.referendumStore.Get(totalVoteBalanceKeyForQuestionAndAnswer(referendumID, questionIdx, answerIdx))

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

func setTotalBalanceForReferendum(referendumID ReferendumID, questionIdx uint8, answerIdx uint8, total uint64, mutations kvstore.BatchedMutations) error {

	ms := marshalutil.New(8)
	ms.WriteUint64(total)

	return mutations.Set(totalVoteBalanceKeyForQuestionAndAnswer(referendumID, questionIdx, answerIdx), ms.Bytes())
}

func (rm *ReferendumManager) countCurrentVote(output *utxo.Output, vote *Vote, increase bool, mutations kvstore.BatchedMutations) error {

	votePower := output.Amount()

	for idx, answerValue := range vote.Answers {
		questionIndex := uint8(idx)
		currentVoteBalance, err := rm.CurrentBalanceForReferendum(vote.ReferendumID, questionIndex, answerValue)
		if err != nil {
			return err
		}

		if increase {
			currentVoteBalance += votePower
		} else {
			if currentVoteBalance < votePower {
				// Votes can't be less than 0
				return ErrInvalidCurrentVoteBalance
			}
			currentVoteBalance -= votePower
		}

		ms := marshalutil.New(8)
		ms.WriteUint64(currentVoteBalance)

		if err := mutations.Set(currentVoteBalanceKeyForQuestionAndAnswer(vote.ReferendumID, questionIndex, answerValue), ms.Bytes()); err != nil {
			return err
		}
	}

	return nil
}
