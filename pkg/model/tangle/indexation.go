package tangle

import (
	"fmt"

	"github.com/dchest/blake2b"
	"github.com/iotaledger/hive.go/objectstorage"
	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/hornet"
)

type Indexation struct {
	objectstorage.StorableObjectFlags
	indexationHash hornet.Hash
	messageID      hornet.Hash
}

func NewIndexation(index string, messageID hornet.Hash) *Indexation {

	indexationHash := blake2b.Sum256([]byte(index))

	return &Indexation{
		indexationHash: indexationHash[:],
		messageID:      messageID,
	}
}

func (i *Indexation) GetHash() hornet.Hash {
	return i.indexationHash
}

func (i *Indexation) GetMessageID() hornet.Hash {
	return i.messageID
}

// ObjectStorage interface

func (i *Indexation) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("Indexation should never be updated: %v, MessageID: %v", i.indexationHash.Hex(), i.messageID.Hex()))
}

func (i *Indexation) ObjectStorageKey() []byte {
	return append(i.indexationHash, i.messageID...)
}

func (i *Indexation) ObjectStorageValue() (_ []byte) {
	return nil
}

func CheckIfIndexation(msg *Message) (indexation *iotago.IndexationPayload) {

	// check if the message contains an indexation payload
	switch payload := msg.GetMessage().Payload.(type) {
	case *iotago.IndexationPayload:
		return payload
	case *iotago.UnsignedTransaction:
		// check optional payload of unsigned transaction payload
		switch optionalPayload := payload.Payload.(type) {
		case *iotago.IndexationPayload:
			return optionalPayload
		default:
			// do nothing
		}
	default:
		// do nothing
	}

	return nil
}
