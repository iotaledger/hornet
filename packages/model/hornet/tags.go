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
	if obj, ok := other.(*Tag); !ok {
		panic("invalid object passed to Tag.Update()")
	} else {
		t.Tag = obj.Tag
		t.TxHash = obj.TxHash
	}
}

func (t *Tag) GetStorageKey() []byte {
	return append(t.Tag, t.TxHash...)
}

func (t *Tag) MarshalBinary() (data []byte, err error) {
	return nil, nil
}

func (t *Tag) UnmarshalBinary(data []byte) error {
	return nil
}
