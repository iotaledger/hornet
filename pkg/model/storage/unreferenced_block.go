package storage

import (
	"encoding/binary"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/objectstorage"
	iotago "github.com/iotaledger/iota.go/v3"
)

type UnreferencedBlock struct {
	objectstorage.StorableObjectFlags
	latestMilestoneIndex milestone.Index
	blockID              iotago.BlockID
}

func NewUnreferencedBlock(msIndex milestone.Index, blockID iotago.BlockID) *UnreferencedBlock {
	return &UnreferencedBlock{
		latestMilestoneIndex: msIndex,
		blockID:              blockID,
	}
}

func (t *UnreferencedBlock) LatestMilestoneIndex() milestone.Index {
	return t.latestMilestoneIndex
}

func (t *UnreferencedBlock) BlockID() iotago.BlockID {
	return t.blockID
}

// ObjectStorage interface

func (t *UnreferencedBlock) Update(_ objectstorage.StorableObject) {
	// do nothing, since the object is identical (consists of key only)
}

func (t *UnreferencedBlock) ObjectStorageKey() []byte {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(t.latestMilestoneIndex))
	return append(key, t.blockID[:]...)
}

func (t *UnreferencedBlock) ObjectStorageValue() []byte {
	return nil
}
