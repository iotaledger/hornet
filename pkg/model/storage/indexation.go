package storage

import (
	"fmt"

	"github.com/dchest/blake2b"
	"github.com/iotaledger/hive.go/objectstorage"
	iotago "github.com/iotaledger/iota.go/v2"

	"github.com/gohornet/hornet/pkg/model/hornet"
)

const (
	IndexationHashLength = 32
)

type Indexation struct {
	objectstorage.StorableObjectFlags
	indexationHash hornet.MessageID
	messageID      hornet.MessageID
}

func NewIndexation(index string, messageID hornet.MessageID) *Indexation {

	indexationHash := hornet.MessageIDFromArray((blake2b.Sum256([]byte(index))))

	return &Indexation{
		indexationHash: indexationHash,
		messageID:      messageID,
	}
}

func (i *Indexation) GetHash() hornet.MessageID {
	return i.indexationHash
}

func (i *Indexation) GetMessageID() hornet.MessageID {
	return i.messageID
}

// ObjectStorage interface

func (i *Indexation) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("Indexation should never be updated: %v, MessageID: %v", i.indexationHash.ToHex(), i.messageID.ToHex()))
}

func (i *Indexation) ObjectStorageKey() []byte {
	return append(i.indexationHash, i.messageID...)
}

func (i *Indexation) ObjectStorageValue() (_ []byte) {
	return nil
}

func CheckIfIndexation(msg *Message) (indexation *iotago.Indexation) {

	if msgIndexation := msg.GetIndexation(); msgIndexation != nil {
		return msgIndexation
	}

	if txIndexation := msg.GetTransactionEssenceIndexation(); txIndexation != nil {
		return txIndexation
	}

	return nil
}
