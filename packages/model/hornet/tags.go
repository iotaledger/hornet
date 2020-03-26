package hornet

import (
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/iota.go/trinary"
)

type Tag struct {
	objectstorage.StorableObjectFlags
	Tag    []byte
	TxHash []byte
}

func (t *Tag) GetTag() trinary.Hash {
	return trinary.MustBytesToTrytes(t.Tag, 27)
}

func (t *Tag) GetTransactionHash() trinary.Hash {
	return trinary.MustBytesToTrytes(t.TxHash, 81)
}

// ObjectStorage interface

func (t *Tag) Update(other objectstorage.StorableObject) {
	panic("Tag should never be updated")
}

func (t *Tag) ObjectStorageKey() []byte {
	return append(t.Tag, t.TxHash...)
}

func (t *Tag) ObjectStorageValue() (data []byte) {
	return nil
}

func (t *Tag) UnmarshalObjectStorageValue(data []byte) (err error, consumedBytes int) {
	return nil, 0
}
