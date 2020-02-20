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
	TxHash []byte

	// Unix time when the Tx became solid (needed for local modifiers for tipselection)
	solidificationTimestamp int32

	// The index of the milestone which confirmed this tx
	confirmationIndex milestone_index.MilestoneIndex

	// Metadata
	metadataMutex syncutils.RWMutex
	metadata      bitmask.BitMask
}

func (m *TransactionMetadata) GetSolidificationTimestamp() int32 {
	return m.solidificationTimestamp
}

func (m *TransactionMetadata) IsSolid() bool {
	m.metadataMutex.RLock()
	defer m.metadataMutex.RUnlock()
	s := m.metadata.HasFlag(HORNET_TX_METADATA_SOLID)
	return s
}

func (m *TransactionMetadata) SetSolid(solid bool) {
	m.metadataMutex.Lock()
	defer m.metadataMutex.Unlock()

	if solid != m.metadata.HasFlag(HORNET_TX_METADATA_SOLID) {
		m.solidificationTimestamp = int32(time.Now().Unix())
		m.metadata = m.metadata.ModifyFlag(HORNET_TX_METADATA_SOLID, solid)
		m.SetModified(true)
	}
}

func (m *TransactionMetadata) GetConfirmed() (bool, milestone_index.MilestoneIndex) {
	m.metadataMutex.RLock()
	defer m.metadataMutex.RUnlock()

	return m.metadata.HasFlag(HORNET_TX_METADATA_CONFIRMED), m.confirmationIndex
}

func (m *TransactionMetadata) SetConfirmed(confirmed bool, confirmationIndex milestone_index.MilestoneIndex) {
	m.metadataMutex.Lock()
	defer m.metadataMutex.Unlock()

	if (confirmed != m.metadata.HasFlag(HORNET_TX_METADATA_CONFIRMED)) || (m.confirmationIndex != confirmationIndex) {
		m.metadata = m.metadata.ModifyFlag(HORNET_TX_METADATA_CONFIRMED, confirmed)
		m.confirmationIndex = confirmationIndex
		m.SetModified(true)
	}
}

func (m *TransactionMetadata) GetMetadata() byte {
	m.metadataMutex.RLock()
	defer m.metadataMutex.RUnlock()

	return byte(m.metadata)
}

// ObjectStorage interface

func (m *TransactionMetadata) Update(other objectstorage.StorableObject) {
	panic("No Update() should be called")
}

func (m *TransactionMetadata) GetStorageKey() []byte {
	return m.TxHash
}

func (m *TransactionMetadata) MarshalBinary() (data []byte, err error) {

	/*
		1 byte  metadata bitmask
		4 bytes uint32 confirmationIndex
		4 bytes uint32 solidificationTimestamp
	*/

	confirmed, confirmationIndex := m.GetConfirmed()
	if !confirmed {
		confirmationIndex = 0
	}

	value := make([]byte, 9)
	value[0] = m.GetMetadata()
	binary.LittleEndian.PutUint32(value[1:], uint32(confirmationIndex))
	binary.LittleEndian.PutUint32(value[5:], uint32(m.GetSolidificationTimestamp()))

	return value, nil
}

func (m *TransactionMetadata) UnmarshalBinary(data []byte) error {

	/*
		1 byte  metadata bitmask
		4 bytes uint32 confirmationIndex
		4 bytes uint32 solidificationTimestamp
	*/

	m.metadata = bitmask.BitMask(data[0])
	m.confirmationIndex = milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(data[1:5]))
	m.solidificationTimestamp = int32(binary.LittleEndian.Uint32(data[5:9]))

	return nil
}
