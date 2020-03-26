package hornet

import (
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/iota.go/trinary"
)

type SpentAddress struct {
	objectstorage.StorableObjectFlags

	Address []byte
}

func (sa *SpentAddress) GetAddress() trinary.Hash {
	return trinary.MustBytesToTrytes(sa.Address, 81)
}

// ObjectStorage interface

func (sa *SpentAddress) Update(other objectstorage.StorableObject) {
	panic("SpentAddress should never be updated")
}

func (sa *SpentAddress) ObjectStorageKey() []byte {
	return sa.Address
}

func (sa *SpentAddress) ObjectStorageValue() (data []byte) {
	return nil
}

func (sa *SpentAddress) UnmarshalObjectStorageValue(data []byte) error {
	return nil
}
