package tangle

import (
	"fmt"
	"log"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/iotaledger/hive.go/objectstorage"
	iotago "github.com/iotaledger/iota.go"
)

// Storable Object
type Message struct {
	objectstorage.StorableObjectFlags

	// Key
	messageID hornet.Hash

	// Value
	message *iotago.Message
}

func NewMessage(iotaMsg *iotago.Message) (*Message, error) {

	hash, err := iotaMsg.Hash()
	if err != nil {
		return nil, err
	}

	return &Message{
		messageID: hash[:],
		message:   iotaMsg,
	}, nil
}

func MessageFromBytes(data []byte, deSeriMode iotago.DeSerializationMode) (*Message, error) {
	msg := &iotago.Message{}
	if _, err := msg.Deserialize(data, deSeriMode); err != nil {
		return nil, err
	}

	hash, err := msg.Hash()
	if err != nil {
		return nil, err
	}

	return &Message{
		messageID: hash[:],
		message:   msg,
	}, nil
}

func (msg *Message) GetMessageID() hornet.Hash {
	return msg.messageID
}

func (msg *Message) GetMessage() *iotago.Message {
	return msg.message
}

func (msg *Message) GetParent1MessageID() hornet.Hash {
	return msg.message.Parent1[:]
}

func (msg *Message) GetParent2MessageID() hornet.Hash {
	return msg.message.Parent2[:]
}

func (msg *Message) IsMilestone() bool {
	switch ms := msg.GetMessage().Payload.(type) {
	case *iotago.MilestonePayload:
		if err := ms.VerifySignature(msg.GetMessage(), coordinatorPublicKey); err != nil {
			return true
		}
	default:
	}

	return false
}

func (msg *Message) IsValue() bool {
	switch msg.GetMessage().Payload.(type) {
	case *iotago.SignedTransactionPayload:
		return true
	default:
	}

	return false
}

// ObjectStorage interface

func (msg *Message) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("Message should never be updated: %v", msg.messageID.Hex()))
}

func (msg *Message) ObjectStorageKey() []byte {
	return msg.messageID
}

func (msg *Message) ObjectStorageValue() (_ []byte) {
	data, err := msg.message.Serialize(iotago.DeSeriModePerformValidation) //TODO: should we skip verification?
	if err != nil {
		log.Fatalf("Error serializing message: %v", err)
	}
	return data
}
