package storage

import (
	"github.com/iotaledger/hive.go/core/objectstorage"
	iotago "github.com/iotaledger/iota.go/v3"
)

type Child struct {
	objectstorage.StorableObjectFlags
	parentBlockID iotago.BlockID
	childBlockID  iotago.BlockID
}

func NewChild(parentBlockID iotago.BlockID, childBlockID iotago.BlockID) *Child {
	return &Child{
		parentBlockID: parentBlockID,
		childBlockID:  childBlockID,
	}
}

func (a *Child) ParentBlockID() iotago.BlockID {
	return a.parentBlockID
}

func (a *Child) ChildBlockID() iotago.BlockID {
	return a.childBlockID
}

// ObjectStorage interface

func (a *Child) Update(_ objectstorage.StorableObject) {
	// do nothing, since the object is identical (consists of key only)
}

func (a *Child) ObjectStorageKey() []byte {
	return append(a.parentBlockID[:], a.childBlockID[:]...)
}

func (a *Child) ObjectStorageValue() []byte {
	return nil
}
