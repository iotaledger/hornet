package hornet

import (
	"fmt"

	"github.com/iotaledger/hive.go/objectstorage"
)

type Child struct {
	objectstorage.StorableObjectFlags
	parentMessageID Hash
	childMessageID  Hash
}

func NewChild(parentMessageID Hash, childMessageId Hash) *Child {
	return &Child{
		parentMessageID: parentMessageID,
		childMessageID:  childMessageId,
	}
}

func (a *Child) GetParentMessageID() Hash {
	return a.parentMessageID
}

func (a *Child) GetChildMessageID() Hash {
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
