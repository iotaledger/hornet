package hornet

import (
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/iota.go/trinary"
)

type Approver struct {
	objectstorage.StorableObjectFlags
	TxHash       []byte
	ApproverHash []byte
}

func (a *Approver) GetTransactionHash() trinary.Hash {
	return trinary.MustBytesToTrytes(a.TxHash, 81)
}

func (a *Approver) GetApproverHash() trinary.Hash {
	return trinary.MustBytesToTrytes(a.ApproverHash, 81)
}

// ObjectStorage interface

func (a *Approver) Update(_ objectstorage.StorableObject) {
	panic("Approver should never be updated")
}

func (a *Approver) ObjectStorageKey() []byte {
	return append(a.TxHash, a.ApproverHash...)
}

func (a *Approver) ObjectStorageValue() (data []byte) {
	return nil
}

func (a *Approver) UnmarshalObjectStorageValue(_ []byte) (err error, consumedBytes int) {
	return nil, 0
}
