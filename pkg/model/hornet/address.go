package hornet

import (
	"fmt"

	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	AddressTxIsValue = 1
)

type Address struct {
	objectstorage.StorableObjectFlags
	address Hash
	isValue bool
	txHash  Hash
}

func NewAddress(address Hash, txHash Hash, isValue bool) *Address {
	return &Address{
		address: address,
		isValue: isValue,
		txHash:  txHash,
	}
}

func (a *Address) GetAddress() Hash {
	return a.address
}

func (a *Address) GetTxHash() Hash {
	return a.txHash
}

func (a *Address) IsValue() bool {
	return a.isValue
}

// ObjectStorage interface

func (a *Address) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("Address should never be updated: %v, TxHash: %v", a.address.Trytes(), a.txHash.Trytes()))
}

func (a *Address) ObjectStorageKey() []byte {

	var isValueByte byte
	if a.isValue {
		isValueByte = AddressTxIsValue
	}

	result := append(a.address, isValueByte)
	return append(result, a.txHash...)
}

func (a *Address) ObjectStorageValue() (_ []byte) {
	return nil
}
