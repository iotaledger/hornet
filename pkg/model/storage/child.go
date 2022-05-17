package storage

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/iotaledger/hive.go/objectstorage"
)

type Child struct {
	objectstorage.StorableObjectFlags
	parentBlockID hornet.BlockID
	childBlockID  hornet.BlockID
}

func NewChild(parentBlockID hornet.BlockID, childBlockID hornet.BlockID) *Child {
	return &Child{
		parentBlockID: parentBlockID,
		childBlockID:  childBlockID,
	}
}

func (a *Child) ParentBlockID() hornet.BlockID {
	return a.parentBlockID
}

func (a *Child) ChildBlockID() hornet.BlockID {
	return a.childBlockID
}

// ObjectStorage interface

func (a *Child) Update(_ objectstorage.StorableObject) {
	// do nothing, since the object is identical (consists of key only)
}

func (a *Child) ObjectStorageKey() []byte {
	return append(a.parentBlockID, a.childBlockID...)
}

func (a *Child) ObjectStorageValue() []byte {
	return nil
}
