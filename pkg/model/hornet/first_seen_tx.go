package hornet

import (
	"encoding/binary"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

type UnconfirmedTx struct {
	objectstorage.StorableObjectFlags
	LatestMilestoneIndex milestone.Index
	TxHash               []byte
}

func (t *UnconfirmedTx) GetLatestMilestoneIndex() milestone.Index {
	return t.LatestMilestoneIndex
}

func (t *UnconfirmedTx) GetTransactionHash() trinary.Hash {
	return trinary.MustBytesToTrytes(t.TxHash, 81)
}

// ObjectStorage interface

func (t *UnconfirmedTx) Update(_ objectstorage.StorableObject) {
	panic("UnconfirmedTx should never be updated")
}

func (t *UnconfirmedTx) ObjectStorageKey() []byte {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(t.LatestMilestoneIndex))
	return append(key, t.TxHash...)
}

func (t *UnconfirmedTx) ObjectStorageValue() (data []byte) {
	return nil
}

func (t *UnconfirmedTx) UnmarshalObjectStorageValue(_ []byte) (err error, consumedBytes int) {
	return nil, 0
}
