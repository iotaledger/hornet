package hornet

import (
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/iota.go/trinary"
)

type Approver struct {
	objectstorage.StorableObjectFlags
	TxHash []byte
	Hash   []byte
}

func (a *Approver) GetTransactionHash() trinary.Hash {
	return trinary.MustBytesToTrytes(a.TxHash, 81)
}

func (a *Approver) GetHash() trinary.Hash {
	return trinary.MustBytesToTrytes(a.Hash, 81)
}

// ObjectStorage interface

func (a *Approver) Update(other objectstorage.StorableObject) {
	if obj, ok := other.(*Approver); !ok {
		panic("invalid object passed to Approver.Update()")
	} else {
		a.TxHash = obj.TxHash
		a.Hash = obj.Hash
	}
}

func (a *Approver) GetStorageKey() []byte {
	return append(a.TxHash, a.Hash...)
}

func (a *Approver) MarshalBinary() (data []byte, err error) {
	return nil, nil
}

func (a *Approver) UnmarshalBinary(data []byte) error {
	return nil
}
