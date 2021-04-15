package storage

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/iotaledger/hive.go/objectstorage"
)

type Child struct {
	objectstorage.StorableObjectFlags
	parentMessageID hornet.MessageID
	childMessageID  hornet.MessageID
}

func NewChild(parentMessageID hornet.MessageID, childMessageId hornet.MessageID) *Child {
	return &Child{
		parentMessageID: parentMessageID,
		childMessageID:  childMessageId,
	}
}

func (a *Child) GetParentMessageID() hornet.MessageID {
	return a.parentMessageID
}

func (a *Child) GetChildMessageID() hornet.MessageID {
	return a.childMessageID
}

// ObjectStorage interface

func (a *Child) Update(_ objectstorage.StorableObject) {
	// do nothing, since the object is identical (consists of key only)
}

func (a *Child) ObjectStorageKey() []byte {
	return append(a.parentMessageID, a.childMessageID...)
}

func (a *Child) ObjectStorageValue() (_ []byte) {
	return nil
}
