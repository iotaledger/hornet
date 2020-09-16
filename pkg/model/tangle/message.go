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
	data, err := msg.message.Serialize(iotago.DeSeriModePerformValidation)
	if err != nil {
		log.Fatalf("Error serializing message: %v", err)
	}
	return data
}
