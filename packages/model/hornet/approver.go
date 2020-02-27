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

func (a *Approver) Update(other objectstorage.StorableObject) {
	panic("Approver should never be updated")
}

func (a *Approver) GetStorageKey() []byte {
	return append(a.TxHash, a.ApproverHash...)
}

func (a *Approver) MarshalBinary() (data []byte, err error) {
	return nil, nil
}

func (a *Approver) UnmarshalBinary(data []byte) error {
	return nil
}
