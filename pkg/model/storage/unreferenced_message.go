package storage

import (
	"encoding/binary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/objectstorage"
)

type UnreferencedMessage struct {
	objectstorage.StorableObjectFlags
	latestMilestoneIndex milestone.Index
	blockID              hornet.BlockID
}

func NewUnreferencedMessage(msIndex milestone.Index, blockID hornet.BlockID) *UnreferencedMessage {
	return &UnreferencedMessage{
		latestMilestoneIndex: msIndex,
		blockID:              blockID,
	}
}

func (t *UnreferencedMessage) LatestMilestoneIndex() milestone.Index {
	return t.latestMilestoneIndex
}

func (t *UnreferencedMessage) MessageID() hornet.BlockID {
	return t.blockID
}

// ObjectStorage interface

func (t *UnreferencedMessage) Update(_ objectstorage.StorableObject) {
	// do nothing, since the object is identical (consists of key only)
}

func (t *UnreferencedMessage) ObjectStorageKey() []byte {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(t.latestMilestoneIndex))
	return append(key, t.blockID...)
}

func (t *UnreferencedMessage) ObjectStorageValue() []byte {
	return nil
}
