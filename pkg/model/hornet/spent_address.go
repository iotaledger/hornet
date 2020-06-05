package hornet

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

type SpentAddress struct {
	objectstorage.StorableObjectFlags

	address Hash
}

func NewSpentAddress(address Hash) *SpentAddress {
	return &SpentAddress{
		address: address,
	}
}

func (sa *SpentAddress) GetAddress() Hash {
	return sa.address
}

// ObjectStorage interface

func (sa *SpentAddress) Update(_ objectstorage.StorableObject) {
	panic("SpentAddress should never be updated")
}

func (sa *SpentAddress) ObjectStorageKey() []byte {
	return sa.address
}

func (sa *SpentAddress) ObjectStorageValue() (_ []byte) {
	return nil
}

func (sa *SpentAddress) UnmarshalObjectStorageValue(_ []byte) (consumedBytes int, err error) {
	return 0, nil
}
