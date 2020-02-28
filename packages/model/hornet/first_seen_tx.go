package hornet

import (
	"encoding/binary"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/model/milestone_index"
)

type FirstSeenTx struct {
	objectstorage.StorableObjectFlags
	FirstSeenLatestMilestoneIndex milestone_index.MilestoneIndex
	TxHash                        []byte
}

func (t *FirstSeenTx) GetFirstSeenLatestMilestoneIndex() milestone_index.MilestoneIndex {
	return t.FirstSeenLatestMilestoneIndex
}

func (t *FirstSeenTx) GetTransactionHash() trinary.Hash {
	return trinary.MustBytesToTrytes(t.TxHash, 81)
}

// ObjectStorage interface

func (t *FirstSeenTx) Update(other objectstorage.StorableObject) {
	panic("FirstSeenTx should never be updated")
}

func (t *FirstSeenTx) GetStorageKey() []byte {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(t.FirstSeenLatestMilestoneIndex))
	return append(key, t.TxHash...)
}

func (t *FirstSeenTx) MarshalBinary() (data []byte, err error) {
	return nil, nil
}

func (t *FirstSeenTx) UnmarshalBinary(data []byte) error {
	return nil
}
