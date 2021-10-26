package referendum

import (
	"errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
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
	m.WriteByte(ReferendumStoreKeyPrefixMessages) // 1 byte
	m.WriteBytes(referendumID[:])                 // 32 bytes
	return m.Bytes()
}

func (rm *ReferendumManager) loadReferendums() (map[ReferendumID]*Referendum, error) {

	referendums := make(map[ReferendumID]*Referendum)

	var innerErr error
	if err := rm.referendumStore.Iterate(kvstore.KeyPrefix{ReferendumStoreKeyPrefixMessages}, func(key kvstore.Key, value kvstore.Value) bool {

		referendumID := ReferendumID{}
		copy(referendumID[:], key[1:]) // Skip the prefix

		referendum := &Referendum{}
		_, innerErr = referendum.Deserialize(value, iotago.DeSeriModeNoValidation)
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

	referendumBytes, err := referendum.Serialize(iotago.DeSeriModePerformValidation)
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

func (rm *ReferendumManager) messageForMessageID(messageId hornet.MessageID) (*storage.Message, error) {
	value, err := rm.referendumStore.Get(messageKeyForMessageID(messageId))
	if errors.Is(err, kvstore.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return storage.MessageFromBytes(value, iotago.DeSeriModeNoValidation)
}

// Outputs

func voteKeyForOutputID(outputID *iotago.UTXOInputID) []byte {
	m := marshalutil.New(35)
	m.WriteByte(ReferendumStoreKeyPrefixOutputs) // 1 byte
	m.WriteBytes(outputID[:])                    // 34 bytes
	return m.Bytes()
}

func (rm *ReferendumManager) VoteForOutputID(outputID *iotago.UTXOInputID) (messageID hornet.MessageID, startIndex milestone.Index, endIndex milestone.Index, err error) {

	key := voteKeyForOutputID(outputID)
	value, err := rm.referendumStore.Get(key)
	if errors.Is(err, kvstore.ErrKeyNotFound) {
		err = ErrUnknownVote
		return
	}
	if err != nil {
		return
	}

	if len(value) != 40 {
		err = ErrInvalidPreviouslyTrackedVote
		return
	}

	m := marshalutil.New(value)
	if messageID, err = m.ReadBytes(32); err != nil {
		return
	}

	start, err := m.ReadUint32()
	if err != nil {
		return
	}
	startIndex = milestone.Index(start)

	end, err := m.ReadUint32()
	if err != nil {
		return
	}
	endIndex = milestone.Index(end)

	return
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

func (rm *ReferendumManager) startVoteAtMilestone(output *utxo.Output, startIndex milestone.Index, mutations kvstore.BatchedMutations) error {

	// Store the OutputId with the milestone index to track that this vote was counted for this milestone
	m := marshalutil.New(40)
	m.WriteBytes(output.MessageID())  // 32 bytes
	m.WriteUint32(uint32(startIndex)) // 4 bytes
	m.WriteUint32(0)                  // 4 bytes. Empty end index, since the vote just started to be counted

	return mutations.Set(voteKeyForOutputID(output.OutputID()), m.Bytes())
}

func (rm *ReferendumManager) endVoteAtMilestone(output *utxo.Output, endIndex milestone.Index, mutations kvstore.BatchedMutations) error {

	key := voteKeyForOutputID(output.OutputID())

	value, err := rm.referendumStore.Get(key)
	if err != nil {
		return err
	}

	if len(value) != 40 {
		return ErrInvalidPreviouslyTrackedVote
	}

	m := marshalutil.New(value[:36])
	m.WriteUint32(uint32(endIndex))

	return mutations.Set(key, m.Bytes())
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
