package hornet

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

type Tag struct {
	objectstorage.StorableObjectFlags
	tag    Hash
	txHash Hash
}

func NewTag(tag Hash, txHash Hash) *Tag {
	return &Tag{
		tag:    tag,
		txHash: txHash,
	}
}

func (t *Tag) GetTag() Hash {
	return t.tag
}

func (t *Tag) GetTxHash() Hash {
	return t.txHash
}

// ObjectStorage interface

func (t *Tag) Update(_ objectstorage.StorableObject) {
	panic("Tag should never be updated")
}

func (t *Tag) ObjectStorageKey() []byte {
	return append(t.tag, t.txHash...)
}

func (t *Tag) ObjectStorageValue() (_ []byte) {
	return nil
}

func (t *Tag) UnmarshalObjectStorageValue(_ []byte) (consumedBytes int, err error) {
	return 0, nil
}
