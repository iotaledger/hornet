package storage

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/iotaledger/hive.go/objectstorage"
	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	IndexationIndexLength = 64
)

// PadIndexationIndex returns a padded indexation index.
func PadIndexationIndex(index []byte) []byte {
	return append(index, make([]byte, IndexationIndexLength-len(index))...)
}

type Indexation struct {
	objectstorage.StorableObjectFlags
	index     []byte
	messageID hornet.MessageID
}

func NewIndexation(index []byte, messageID hornet.MessageID) *Indexation {
	return &Indexation{
		index:     PadIndexationIndex(index),
		messageID: messageID,
	}
}

func (i *Indexation) GetHash() hornet.MessageID {
	return i.index
}

func (i *Indexation) GetMessageID() hornet.MessageID {
	return i.messageID
}

// ObjectStorage interface

func (i *Indexation) Update(_ objectstorage.StorableObject) {
	// do nothing, since the object is identical (consists of key only)
}

func (i *Indexation) ObjectStorageKey() []byte {
	return append(i.index, i.messageID...)
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
