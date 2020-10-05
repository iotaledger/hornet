package tangle

import (
	"fmt"
	"log"

	iotago "github.com/iotaledger/iota.go"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
)

// Storable Object
type Message struct {
	objectstorage.StorableObjectFlags

	// Key
	messageID *hornet.MessageID

	// Value
	message *iotago.Message
}

func NewMessage(iotaMsg *iotago.Message) (*Message, error) {

	msgHash, err := iotaMsg.Hash()
	if err != nil {
		return nil, err
	}

	messageID := hornet.MessageID(*msgHash)

	return &Message{
		messageID: &messageID,
		message:   iotaMsg,
	}, nil
}

func MessageFromBytes(data []byte, deSeriMode iotago.DeSerializationMode) (*Message, error) {
	msg := &iotago.Message{}
	if _, err := msg.Deserialize(data, deSeriMode); err != nil {
		return nil, err
	}

	msgHash, err := msg.Hash()
	if err != nil {
		return nil, err
	}

	messageID := hornet.MessageID(*msgHash)

	return &Message{
		messageID: &messageID,
		message:   msg,
	}, nil
}

func (msg *Message) GetMessageID() *hornet.MessageID {
	return msg.messageID
}

func (msg *Message) GetMessage() *iotago.Message {
	return msg.message
}

func (msg *Message) GetParent1MessageID() *hornet.MessageID {
	parent1 := hornet.MessageID(msg.message.Parent1)
	return &parent1
}

func (msg *Message) GetParent2MessageID() *hornet.MessageID {
	parent2 := hornet.MessageID(msg.message.Parent2)
	return &parent2
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

func (msg *Message) IsTransaction() bool {
	switch msg.GetMessage().Payload.(type) {
	case *iotago.SignedTransactionPayload:
		return true
	default:
	}

	return false
}

func (msg *Message) GetSignedTransactionPayload() *iotago.SignedTransactionPayload {

	switch payload := msg.GetMessage().Payload.(type) {
	case *iotago.SignedTransactionPayload:
		return payload
	default:
		return nil
	}
}

func (msg *Message) GetUnsignedTransaction() *iotago.UnsignedTransaction {

	if signedTransaction := msg.GetSignedTransactionPayload(); signedTransaction != nil {
		switch unsignedTransaction := signedTransaction.Transaction.(type) {
		case *iotago.UnsignedTransaction:
			return unsignedTransaction
		default:
			return nil
		}
	}
	return nil
}

func (msg *Message) GetUnsignedTransactionUTXOInputs() []iotago.UTXOInputID {

	var inputs []iotago.UTXOInputID
	if unsignedTransaction := msg.GetUnsignedTransaction(); unsignedTransaction != nil {
		for _, input := range unsignedTransaction.Inputs {
			switch utxoInput := input.(type) {
			case *iotago.UTXOInput:
				inputs = append(inputs, utxoInput.ID())
			default:
				return nil
			}
		}
	}
	return inputs
}

func (msg *Message) GetSignatureForInputIndex(inputIndex uint16) *iotago.Ed25519Signature {

	if signedTransaction := msg.GetSignedTransactionPayload(); signedTransaction != nil {
		switch unlockBlock := signedTransaction.UnlockBlocks[inputIndex].(type) {
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
	panic(fmt.Sprintf("Message should never be updated: %v", msg.messageID.Hex()))
}

func (msg *Message) ObjectStorageKey() []byte {
	return msg.messageID.Slice()
}

func (msg *Message) ObjectStorageValue() (_ []byte) {
	data, err := msg.message.Serialize(iotago.DeSeriModePerformValidation) //TODO: should we skip verification?
	if err != nil {
		log.Fatalf("Error serializing message: %v", err)
	}
	return data
}
