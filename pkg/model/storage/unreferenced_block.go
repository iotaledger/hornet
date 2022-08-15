package storage

import (
	"encoding/binary"

	"github.com/iotaledger/hive.go/core/objectstorage"
	iotago "github.com/iotaledger/iota.go/v3"
)

type UnreferencedBlock struct {
	objectstorage.StorableObjectFlags
	latestMilestoneIndex iotago.MilestoneIndex
	blockID              iotago.BlockID
}

func NewUnreferencedBlock(msIndex iotago.MilestoneIndex, blockID iotago.BlockID) *UnreferencedBlock {
	return &UnreferencedBlock{
		latestMilestoneIndex: msIndex,
		blockID:              blockID,
	}
}

func (t *UnreferencedBlock) LatestMilestoneIndex() iotago.MilestoneIndex {
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
	binary.LittleEndian.PutUint32(key, t.latestMilestoneIndex)

	return append(key, t.blockID[:]...)
}

func (t *UnreferencedBlock) ObjectStorageValue() []byte {
	return nil
}
