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
	panic("Address should never be updated")
}

func (a *Address) ObjectStorageKey() []byte {
	return append(a.Address, a.TxHash...)
}

func (a *Address) ObjectStorageValue() (data []byte) {
	return nil
}

func (a *Address) UnmarshalObjectStorageValue(data []byte) (err error, consumedBytes int) {
	return nil, 0
}
