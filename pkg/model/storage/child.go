package storage

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/iotaledger/hive.go/objectstorage"
)

type Child struct {
	objectstorage.StorableObjectFlags
	parentMessageID hornet.BlockID
	childMessageID  hornet.BlockID
}

func NewChild(parentMessageID hornet.BlockID, childMessageID hornet.BlockID) *Child {
	return &Child{
		parentMessageID: parentMessageID,
		childMessageID:  childMessageID,
	}
}

func (a *Child) ParentMessageID() hornet.BlockID {
	return a.parentMessageID
}

func (a *Child) ChildMessageID() hornet.BlockID {
	return a.childMessageID
}

// ObjectStorage interface

func (a *Child) Update(_ objectstorage.StorableObject) {
	// do nothing, since the object is identical (consists of key only)
}

func (a *Child) ObjectStorageKey() []byte {
	return append(a.parentMessageID, a.childMessageID...)
}

func (a *Child) ObjectStorageValue() []byte {
	return nil
}
