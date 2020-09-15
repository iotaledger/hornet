package hornet

import (
	"fmt"

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
	panic(fmt.Sprintf("SpentAddress should never be updated: %v", sa.address.Trytes()))
}

func (sa *SpentAddress) ObjectStorageKey() []byte {
	return sa.address
}

func (sa *SpentAddress) ObjectStorageValue() (_ []byte) {
	return nil
}
