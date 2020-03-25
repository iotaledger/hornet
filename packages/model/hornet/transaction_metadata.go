package hornet

import (
	"encoding/binary"
	"time"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/packages/model/milestone_index"
)

const (
	HORNET_TX_METADATA_SOLID     = 0
	HORNET_TX_METADATA_CONFIRMED = 1
)

type TransactionMetadata struct {
	objectstorage.StorableObjectFlags
	syncutils.RWMutex

	TxHash []byte

	// Metadata
	metadata bitmask.BitMask

	// Unix time when the Tx became solid (needed for local modifiers for tipselection)
	solidificationTimestamp int32

	// The index of the milestone which confirmed this tx
	confirmationIndex milestone_index.MilestoneIndex
}

func (m *TransactionMetadata) GetSolidificationTimestamp() int32 {
	m.RLock()
	defer m.RUnlock()

	return m.solidificationTimestamp
}

func (m *TransactionMetadata) IsSolid() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasFlag(HORNET_TX_METADATA_SOLID)
}

func (m *TransactionMetadata) SetSolid(solid bool) {
	m.Lock()
	defer m.Unlock()

	if solid != m.metadata.HasFlag(HORNET_TX_METADATA_SOLID) {
		if solid {
			m.solidificationTimestamp = int32(time.Now().Unix())
		} else {
			m.solidificationTimestamp = 0
		}
		m.metadata = m.metadata.ModifyFlag(HORNET_TX_METADATA_SOLID, solid)
		m.SetModified(true)
	}
}

func (m *TransactionMetadata) GetConfirmed() (bool, milestone_index.MilestoneIndex) {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasFlag(HORNET_TX_METADATA_CONFIRMED), m.confirmationIndex
}

func (m *TransactionMetadata) SetConfirmed(confirmed bool, confirmationIndex milestone_index.MilestoneIndex) {
	m.Lock()
	defer m.Unlock()

	if (confirmed != m.metadata.HasFlag(HORNET_TX_METADATA_CONFIRMED)) || (m.confirmationIndex != confirmationIndex) {
		if confirmed {
			m.confirmationIndex = confirmationIndex
		} else {
			m.confirmationIndex = 0
		}
		m.metadata = m.metadata.ModifyFlag(HORNET_TX_METADATA_CONFIRMED, confirmed)
		m.SetModified(true)
	}
}

func (m *TransactionMetadata) Reset() {
	m.Lock()
	defer m.Unlock()

	// Metadata
	m.metadata = bitmask.BitMask(0)
	m.solidificationTimestamp = 0
	m.confirmationIndex = 0
	m.SetModified(true)
}

func (m *TransactionMetadata) GetMetadata() byte {
	m.RLock()
	defer m.RUnlock()

	return byte(m.metadata)
}

// ObjectStorage interface

func (m *TransactionMetadata) Update(other objectstorage.StorableObject) {
	panic("TransactionMetadata should never be updated")
}

func (m *TransactionMetadata) ObjectStorageKey() []byte {
	return m.TxHash
}

func (m *TransactionMetadata) ObjectStorageValue() (data []byte) {
	m.Lock()
	defer m.Unlock()

	/*
		1 byte  metadata bitmask
		4 bytes uint32 solidificationTimestamp
		4 bytes uint32 confirmationIndex
	*/

	value := make([]byte, 9)
	value[0] = byte(m.metadata)
	binary.LittleEndian.PutUint32(value[1:], uint32(m.solidificationTimestamp))
	binary.LittleEndian.PutUint32(value[5:], uint32(m.confirmationIndex))

	return value
}

func (m *TransactionMetadata) UnmarshalObjectStorageValue(data []byte) error {
	m.Lock()
	defer m.Unlock()

	/*
		1 byte  metadata bitmask
		4 bytes uint32 solidificationTimestamp
		4 bytes uint32 confirmationIndex
	*/

	m.metadata = bitmask.BitMask(data[0])
	m.solidificationTimestamp = int32(binary.LittleEndian.Uint32(data[1:5]))
	m.confirmationIndex = milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(data[5:9]))

	return nil
}
