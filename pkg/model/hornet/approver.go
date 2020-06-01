package hornet

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

type Approver struct {
	objectstorage.StorableObjectFlags
	txHash       Hash
	approverHash Hash
}

func NewApprover(txHash Hash, approverHash Hash) *Approver {
	return &Approver{
		txHash:       txHash,
		approverHash: approverHash,
	}
}

func (a *Approver) GetTxHash() Hash {
	return a.txHash
}

func (a *Approver) GetApproverHash() Hash {
	return a.approverHash
}

// ObjectStorage interface

func (a *Approver) Update(_ objectstorage.StorableObject) {
	panic("Approver should never be updated")
}

func (a *Approver) ObjectStorageKey() []byte {
	return append(a.txHash, a.approverHash...)
}

func (a *Approver) ObjectStorageValue() (_ []byte) {
	return nil
}

func (a *Approver) UnmarshalObjectStorageValue(_ []byte) (consumedBytes int, err error) {
	return 0, nil
}
