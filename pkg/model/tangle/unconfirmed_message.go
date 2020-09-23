package tangle

import (
	"encoding/binary"
	"fmt"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

type UnconfirmedMessage struct {
	objectstorage.StorableObjectFlags
	latestMilestoneIndex milestone.Index
	messageID            hornet.Hash
}

func NewUnconfirmedMessage(msIndex milestone.Index, messageID hornet.Hash) *UnconfirmedMessage {
	return &UnconfirmedMessage{
		latestMilestoneIndex: msIndex,
		messageID:            messageID,
	}
}

func (t *UnconfirmedMessage) GetLatestMilestoneIndex() milestone.Index {
	return t.latestMilestoneIndex
}

func (t *UnconfirmedMessage) GetMessageID() hornet.Hash {
	return t.messageID
}

// ObjectStorage interface

func (t *UnconfirmedMessage) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("UnconfirmedMessage should never be updated: %v", t.messageID.Hex()))
}

func (t *UnconfirmedMessage) ObjectStorageKey() []byte {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(t.latestMilestoneIndex))
	return append(key, t.messageID...)
}

func (t *UnconfirmedMessage) ObjectStorageValue() (_ []byte) {
	return nil
}
