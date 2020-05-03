package hornet

import (
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	AddressTxIsValue = 1
)

type Address struct {
	objectstorage.StorableObjectFlags
	Address []byte
	IsValue bool
	TxHash  []byte
}

func (a *Address) GetAddress() trinary.Hash {
	return trinary.MustBytesToTrytes(a.Address, 81)
}

func (a *Address) GetTransactionHash() trinary.Hash {
	return trinary.MustBytesToTrytes(a.TxHash, 81)
}

// ObjectStorage interface

func (a *Address) Update(_ objectstorage.StorableObject) {
	panic("Address should never be updated")
}

func (a *Address) ObjectStorageKey() []byte {

	var isValueByte byte
	if a.IsValue {
		isValueByte = AddressTxIsValue
	}

	result := append(a.Address, isValueByte)
	return append(result, a.TxHash...)
}

func (a *Address) ObjectStorageValue() (_ []byte) {
	return nil
}

func (a *Address) UnmarshalObjectStorageValue(_ []byte) (consumedBytes int, err error) {
	return 0, nil
}
