package storage

import (
	"fmt"
	"sync"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/iotaledger/hive.go/objectstorage"
	iotago "github.com/iotaledger/iota.go/v2"
)

// Storable Object
type Message struct {
	objectstorage.StorableObjectFlags

	// Key
	messageID hornet.MessageID

	// Value
	data        []byte
	messageOnce sync.Once
	message     *iotago.Message
}

func NewMessage(iotaMsg *iotago.Message, deSeriMode iotago.DeSerializationMode) (*Message, error) {

	data, err := iotaMsg.Serialize(deSeriMode)
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

func MessageFromBytes(data []byte, deSeriMode iotago.DeSerializationMode) (*Message, error) {

	iotaMsg := &iotago.Message{}
	if _, err := iotaMsg.Deserialize(data, deSeriMode); err != nil {
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

func (msg *Message) GetMessageID() hornet.MessageID {
	return msg.messageID
}

func (msg *Message) GetData() []byte {
	return msg.data
}

func (msg *Message) GetMessage() *iotago.Message {
	msg.messageOnce.Do(func() {
		iotaMsg := &iotago.Message{}
		if _, err := iotaMsg.Deserialize(msg.data, iotago.DeSeriModeNoValidation); err != nil {
			panic(fmt.Sprintf("failed to deserialize message: %v, error: %s", msg.messageID.ToHex(), err))
		}

		msg.message = iotaMsg
	})
	return msg.message
}

func (msg *Message) GetNetworkID() uint64 {
	return msg.GetMessage().NetworkID
}

func (msg *Message) GetParents() hornet.MessageIDs {
	return hornet.MessageIDsFromSliceOfArrays(msg.GetMessage().Parents)
}

func (msg *Message) IsMilestone() bool {
	switch msg.GetMessage().Payload.(type) {
	case *iotago.Milestone:
		return true
	default:
	}

	return false
}

func (msg *Message) GetMilestone() (ms *iotago.Milestone) {
	switch ms := msg.GetMessage().Payload.(type) {
	case *iotago.Milestone:
		return ms
	default:
	}

	return nil
}

func (msg *Message) IsTransaction() bool {
	switch msg.GetMessage().Payload.(type) {
	case *iotago.Transaction:
		return true
	default:
	}

	return false
}

func (msg *Message) GetIndexation() *iotago.Indexation {

	switch payload := msg.GetMessage().Payload.(type) {
	case *iotago.Indexation:
		return payload
	default:
		return nil
	}
}

func (msg *Message) GetTransaction() *iotago.Transaction {

	switch payload := msg.GetMessage().Payload.(type) {
	case *iotago.Transaction:
		return payload
	default:
		return nil
	}
}

func (msg *Message) GetTransactionEssence() *iotago.TransactionEssence {

	if transaction := msg.GetTransaction(); transaction != nil {
		switch essence := transaction.Essence.(type) {
		case *iotago.TransactionEssence:
			return essence
		default:
			return nil
		}
	}
	return nil
}

func (msg *Message) GetTransactionEssenceIndexation() *iotago.Indexation {

	if essence := msg.GetTransactionEssence(); essence != nil {
		switch payload := essence.Payload.(type) {
		case *iotago.Indexation:
			return payload
		default:
			return nil
		}
	}
	return nil
}

func (msg *Message) GetTransactionEssenceUTXOInputs() []*iotago.UTXOInputID {

	var inputs []*iotago.UTXOInputID
	if essence := msg.GetTransactionEssence(); essence != nil {
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

func (msg *Message) GetSignatureForInputIndex(inputIndex uint16) *iotago.Ed25519Signature {

	if transaction := msg.GetTransaction(); transaction != nil {
		switch unlockBlock := transaction.UnlockBlocks[inputIndex].(type) {
		case *iotago.SignatureUnlockBlock:
			switch signature := unlockBlock.Signature.(type) {
			case *iotago.Ed25519Signature:
				return signature
			default:
				return nil
			}
		case *iotago.ReferenceUnlockBlock:
			return msg.GetSignatureForInputIndex(unlockBlock.Reference)
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
