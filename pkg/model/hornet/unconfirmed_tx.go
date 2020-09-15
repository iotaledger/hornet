package hornet

import (
	"encoding/binary"
	"fmt"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

type UnconfirmedTx struct {
	objectstorage.StorableObjectFlags
	latestMilestoneIndex milestone.Index
	txHash               Hash
}

func NewUnconfirmedTx(msIndex milestone.Index, txHash Hash) *UnconfirmedTx {
	return &UnconfirmedTx{
		latestMilestoneIndex: msIndex,
		txHash:               txHash,
	}
}

func (t *UnconfirmedTx) GetLatestMilestoneIndex() milestone.Index {
	return t.latestMilestoneIndex
}

func (t *UnconfirmedTx) GetTxHash() Hash {
	return t.txHash
}

// ObjectStorage interface

func (t *UnconfirmedTx) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("UnconfirmedTx should never be updated: %v", t.txHash.Trytes()))
}

func (t *UnconfirmedTx) ObjectStorageKey() []byte {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(t.latestMilestoneIndex))
	return append(key, t.txHash...)
}

func (t *UnconfirmedTx) ObjectStorageValue() (_ []byte) {
	return nil
}
