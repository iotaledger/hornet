package tangle

import (
	"fmt"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
)

type Child struct {
	objectstorage.StorableObjectFlags
	parentMessageID hornet.Hash
	childMessageID  hornet.Hash
}

func NewChild(parentMessageID hornet.Hash, childMessageId hornet.Hash) *Child {
	return &Child{
		parentMessageID: parentMessageID,
		childMessageID:  childMessageId,
	}
}

func (a *Child) GetParentMessageID() hornet.Hash {
	return a.parentMessageID
}

func (a *Child) GetChildMessageID() hornet.Hash {
	return a.childMessageID
}

// ObjectStorage interface

func (a *Child) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("Child should never be updated: %v, MessageID: %v", a.childMessageID.Hex(), a.parentMessageID.Hex()))
}

func (a *Child) ObjectStorageKey() []byte {
	return append(a.parentMessageID, a.childMessageID...)
}

func (a *Child) ObjectStorageValue() (_ []byte) {
	return nil
}
