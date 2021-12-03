package storage

import (
	"fmt"
	"sync"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

type Message struct {
	objectstorage.StorableObjectFlags

	// Key
	messageID hornet.MessageID

	// Value
	data        []byte
	messageOnce sync.Once
	message     *iotago.Message
}

func NewMessage(iotaMsg *iotago.Message, deSeriMode serializer.DeSerializationMode) (*Message, error) {

	//TODO: deSeriCtx
	data, err := iotaMsg.Serialize(deSeriMode, nil)
	if err != nil {
		return nil, err
	}

	msgHash, err := iotaMsg.ID()
	if err != nil {
		return nil, err
	}
	messageID := hornet.MessageIDFromArray(*msgHash)

	msg := &Message{messageID: messageID, data: data}

	msg.messageOnce.Do(func() {
		msg.message = iotaMsg
	})

	return msg, nil
}

func MessageFromBytes(data []byte, deSeriMode serializer.DeSerializationMode) (*Message, error) {

	iotaMsg := &iotago.Message{}
	//TODO: deSeriCtx
	if _, err := iotaMsg.Deserialize(data, deSeriMode, nil); err != nil {
		return nil, err
	}

	msgHash, err := iotaMsg.ID()
	if err != nil {
		return nil, err
	}
	messageID := hornet.MessageIDFromArray(*msgHash)

	msg := &Message{messageID: messageID, data: data}

	msg.messageOnce.Do(func() {
		msg.message = iotaMsg
	})

	return msg, nil
}

func (msg *Message) MessageID() hornet.MessageID {
	return msg.messageID
}

func (msg *Message) Data() []byte {
	return msg.data
}

func (msg *Message) Message() *iotago.Message {
	msg.messageOnce.Do(func() {
		iotaMsg := &iotago.Message{}
		//TODO: deSeriCtx
		if _, err := iotaMsg.Deserialize(msg.data, serializer.DeSeriModeNoValidation, nil); err != nil {
			panic(fmt.Sprintf("failed to deserialize message: %v, error: %s", msg.messageID.ToHex(), err))
		}

		msg.message = iotaMsg
	})
	return msg.message
}

func (msg *Message) NetworkID() uint64 {
	return msg.Message().NetworkID
}

func (msg *Message) Parents() hornet.MessageIDs {
	return hornet.MessageIDsFromSliceOfArrays(msg.Message().Parents)
}

func (msg *Message) IsMilestone() bool {
	switch msg.Message().Payload.(type) {
	case *iotago.Milestone:
		return true
	default:
	}

	return false
}

func (msg *Message) Milestone() (ms *iotago.Milestone) {
	switch ms := msg.Message().Payload.(type) {
	case *iotago.Milestone:
		return ms
	default:
	}

	return nil
}

func (msg *Message) IsTransaction() bool {
	switch msg.Message().Payload.(type) {
	case *iotago.Transaction:
		return true
	default:
	}

	return false
}

func (msg *Message) Indexation() *iotago.Indexation {

	switch payload := msg.Message().Payload.(type) {
	case *iotago.Indexation:
		return payload
	default:
		return nil
	}
}

func (msg *Message) Transaction() *iotago.Transaction {
	switch payload := msg.Message().Payload.(type) {
	case *iotago.Transaction:
		return payload
	default:
		return nil
	}
}

func (msg *Message) TransactionEssence() *iotago.TransactionEssence {
	if transaction := msg.Transaction(); transaction != nil {
		return transaction.Essence
	}
	return nil
}

func (msg *Message) TransactionEssenceIndexation() *iotago.Indexation {

	if essence := msg.TransactionEssence(); essence != nil {
		switch payload := essence.Payload.(type) {
		case *iotago.Indexation:
			return payload
		default:
			return nil
		}
	}
	return nil
}

func (msg *Message) TransactionEssenceUTXOInputs() []*iotago.OutputID {

	var inputs []*iotago.OutputID
	if essence := msg.TransactionEssence(); essence != nil {
		for _, input := range essence.Inputs {
			switch utxoInput := input.(type) {
			case *iotago.UTXOInput:
				id := utxoInput.ID()
				inputs = append(inputs, &id)
			default:
				return nil
			}
		}
	}
	return inputs
}

func (msg *Message) SignatureForInputIndex(inputIndex uint16) *iotago.Ed25519Signature {

	if transaction := msg.Transaction(); transaction != nil {
		switch unlockBlock := transaction.UnlockBlocks[inputIndex].(type) {
		case *iotago.SignatureUnlockBlock:
			switch signature := unlockBlock.Signature.(type) {
			case *iotago.Ed25519Signature:
				return signature
			default:
				return nil
			}
		case *iotago.ReferenceUnlockBlock:
			return msg.SignatureForInputIndex(unlockBlock.Reference)
		default:
			return nil
		}
	}
	return nil
}

// ObjectStorage interface

func (msg *Message) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("Message should never be updated: %v", msg.messageID.ToHex()))
}

func (msg *Message) ObjectStorageKey() []byte {
	return msg.messageID
}

func (msg *Message) ObjectStorageValue() (_ []byte) {
	return msg.data
}
