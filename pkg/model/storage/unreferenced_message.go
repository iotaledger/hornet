package storage

import (
	"encoding/binary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/objectstorage"
)

type UnreferencedBlock struct {
	objectstorage.StorableObjectFlags
	latestMilestoneIndex milestone.Index
	blockID              hornet.BlockID
}

func NewUnreferencedBlock(msIndex milestone.Index, blockID hornet.BlockID) *UnreferencedBlock {
	return &UnreferencedBlock{
		latestMilestoneIndex: msIndex,
		blockID:              blockID,
	}
}

func (t *UnreferencedBlock) LatestMilestoneIndex() milestone.Index {
	return t.latestMilestoneIndex
}

func (t *UnreferencedBlock) BlockID() hornet.BlockID {
	return t.blockID
}

// ObjectStorage interface

func (t *UnreferencedBlock) Update(_ objectstorage.StorableObject) {
	// do nothing, since the object is identical (consists of key only)
}

func (t *UnreferencedBlock) ObjectStorageKey() []byte {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(t.latestMilestoneIndex))
	return append(key, t.blockID...)
}

func (t *UnreferencedBlock) ObjectStorageValue() []byte {
	return nil
}
