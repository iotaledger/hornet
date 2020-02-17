package hornet

import (
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"
)

type Address struct {
	objectstorage.StorableObjectFlags
	Address []byte
	TxHash  []byte
}

func (a *Address) GetAddress() trinary.Hash {
	return trinary.MustBytesToTrytes(a.Address, 81)
}

func (a *Address) GetTransactionHash() trinary.Hash {
	return trinary.MustBytesToTrytes(a.TxHash, 81)
}

// ObjectStorage interface

func (a *Address) Update(other objectstorage.StorableObject) {
	if obj, ok := other.(*Address); !ok {
		panic("invalid object passed to TxHashForAddress.Update()")
	} else {
		a.Address = obj.Address
		a.TxHash = obj.TxHash
	}
}

func (a *Address) GetStorageKey() []byte {
	return append(a.Address, a.TxHash...)
}

func (a *Address) MarshalBinary() (data []byte, err error) {
	return nil, nil
}

func (a *Address) UnmarshalBinary(data []byte) error {
	return nil
}
