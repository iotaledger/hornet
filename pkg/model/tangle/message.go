package tangle

import (
	"fmt"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/iotaledger/hive.go/objectstorage"
	iotago "github.com/iotaledger/iota.go"
	"log"
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
