package hornet

import (
	"encoding/binary"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/packages/model/milestone_index"
)

// Storable Object
type TransactionMetaData struct {
	objectstorage.StorableObjectFlags
	syncutils.RWMutex

	TxHash []byte

	// Metadata
	Metadata bitmask.BitMask

	// The index of the milestone which requested (referenced) this tx
	ReqMilestoneIndex milestone_index.MilestoneIndex

	// Unix time when the Tx became solid (needed for local modifiers for tipselection)
	SolidificationTimestamp int32

	// The index of the milestone which confirmed this tx
	ConfirmationIndex milestone_index.MilestoneIndex
}

// ObjectStorage interface
func (txMeta *TransactionMetaData) Update(other objectstorage.StorableObject) {
	panic("TransactionMetaData should never be updated")
}

func (txMeta *TransactionMetaData) GetStorageKey() []byte {
	return txMeta.TxHash
}

func (txMeta *TransactionMetaData) MarshalBinary() (data []byte, err error) {

	/*
		1 byte  Metadata bitmask
		4 bytes uint32 ReqMilestoneIndex
		4 bytes uint32 SolidificationTimestamp
		4 bytes uint32 ConfirmationIndex
	*/

	value := make([]byte, 13)
	value[0] = byte(txMeta.Metadata)
	binary.LittleEndian.PutUint32(value[1:], uint32(txMeta.ReqMilestoneIndex))
	binary.LittleEndian.PutUint32(value[5:], uint32(txMeta.SolidificationTimestamp))
	binary.LittleEndian.PutUint32(value[9:], uint32(txMeta.ConfirmationIndex))

	return value, nil
}

func (txMeta *TransactionMetaData) UnmarshalBinary(data []byte) error {

	/*
		1 byte  Metadata bitmask
		4 bytes uint32 ReqMilestoneIndex
		4 bytes uint32 SolidificationTimestamp
		4 bytes uint32 ConfirmationIndex
	*/

	txMeta.Metadata = bitmask.BitMask(data[0])
	txMeta.ReqMilestoneIndex = milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(data[1:5]))
	txMeta.SolidificationTimestamp = int32(binary.LittleEndian.Uint32(data[5:9]))
	txMeta.ConfirmationIndex = milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(data[9:13]))

	return nil
}

// Cached Object
type CachedTransactionMetaData struct {
	objectstorage.CachedObject
}

func (c *CachedTransactionMetaData) GetTransactionMetaData() *TransactionMetaData {
	return c.Get().(*TransactionMetaData)
}
