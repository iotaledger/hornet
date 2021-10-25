package referendum

import (
	"errors"

	iotago "github.com/iotaledger/iota.go/v2"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
)

var (
	ErrInvalidReferendum            = errors.New("invalid referendum")
	ErrInvalidPreviouslyTrackedVote = errors.New("a previously tracked vote changed and is now invalid")
	ErrInvalidCurrentVoteBalance    = errors.New("current vote balance invalid")
)

func messageKeyForMessageId(messageId hornet.MessageID) []byte {
	m := marshalutil.New(33)
	m.WriteByte(ReferendumStoreKeyPrefixMessages) // 1 byte
	m.WriteBytes(messageId)                       // 32 bytes
	return m.Bytes()
}

func currentVoteBalanceKeyForQuestionAndAnswer(referendumID ReferendumID, questionIndex uint8, answerIndex uint8) []byte {
	ms := marshalutil.New(35)
	ms.WriteByte(ReferendumStoreKeyPrefixCurrentVoteBalanceForQuestionAndAnswer) // 1 byte
	ms.WriteBytes(referendumID[:])                                               // 32 bytes
	ms.WriteUint8(questionIndex)                                                 // 1 byte
	ms.WriteUint8(answerIndex)                                                   // 1 byte
	return ms.Bytes()
}

func totalVoteBalanceKeyForQuestionAndAnswer(referendumID ReferendumID, questionIndex uint8, answerIndex uint8) []byte {
	ms := marshalutil.New(35)
	ms.WriteByte(ReferendumStoreKeyPrefixTotalVoteBalanceForQuestionAndAnswer) // 1 byte
	ms.WriteBytes(referendumID[:])                                             // 32 bytes
	ms.WriteUint8(questionIndex)                                               // 1 byte
	ms.WriteUint8(answerIndex)                                                 // 1 byte
	return ms.Bytes()
}

func (rm *ReferendumManager) storeMessage(message *storage.Message, mutations kvstore.BatchedMutations) error {
	return mutations.Set(messageKeyForMessageId(message.MessageID()), message.Data())
}

func (rm *ReferendumManager) messageForMessageId(messageId hornet.MessageID) (*storage.Message, error) {
	value, err := rm.referendumStore.Get(messageKeyForMessageId(messageId))
	if errors.Is(err, kvstore.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return storage.MessageFromBytes(value, iotago.DeSeriModeNoValidation)
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
