package hornet

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

type TransactionRawData struct {
	objectstorage.StorableObjectFlags

	TxHash []byte

	// Compressed bytes as received via gossip
	RawBytes []byte
}

// ObjectStorage interface
func (txRaw *TransactionRawData) Update(other objectstorage.StorableObject) {
	panic("TransactionRawData should never be updated")
}

func (txRaw *TransactionRawData) GetStorageKey() []byte {
	return txRaw.TxHash
}

func (txRaw *TransactionRawData) MarshalBinary() (data []byte, err error) {

	/*
		x bytes RawBytes
	*/

	value := make([]byte, 0, len(txRaw.RawBytes))
	copy(value, txRaw.RawBytes)

	return value, nil
}

func (txRaw *TransactionRawData) UnmarshalBinary(data []byte) error {

	/*
		x bytes RawBytes
	*/

	txRaw.RawBytes = data

	return nil
}

// Cached Object
type CachedTransactionRawData struct {
	objectstorage.CachedObject
}

func (c *CachedTransactionRawData) GetTransactionRawData() *TransactionRawData {
	return c.Get().(*TransactionRawData)
}
