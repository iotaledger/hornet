package hornet

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

const (
	MessageMetadataSolid       = 0
	MessageMetadataConfirmed   = 1
	MessageMetadataConflicting = 2
)

type MessageMetadata struct {
	objectstorage.StorableObjectFlags
	syncutils.RWMutex

	messageID Hash

	// Metadata
	metadata bitmask.BitMask

	// Unix time when the Tx became solid (needed for local modifiers for tipselection)
	solidificationTimestamp int32

	// The index of the milestone which confirmed this tx
	confirmationIndex milestone.Index

	// youngestRootSnapshotIndex is the highest confirmed index of the past cone of this transaction
	youngestRootSnapshotIndex milestone.Index

	// oldestRootSnapshotIndex is the lowest confirmed index of the past cone of this transaction
	oldestRootSnapshotIndex milestone.Index

	// rootSnapshotCalculationIndex is the solid index ymrsi and omrsi were calculated at
	rootSnapshotCalculationIndex milestone.Index

	// parent1MessageID is the parent1 (trunk) of the message
	parent1MessageID Hash

	// parent2MessageID is the parent2 (branch) of the message
	parent2MessageID Hash
}

func NewMessageMetadata(messageID Hash, parent1MessageID Hash, parent2MessageID Hash) *MessageMetadata {
	return &MessageMetadata{
		messageID:        messageID,
		parent1MessageID: parent1MessageID,
		parent2MessageID: parent2MessageID,
	}
}

func (m *MessageMetadata) GetMessageID() Hash {
	return m.messageID
}

func (m *MessageMetadata) GetParent1MessageID() Hash {
	return m.parent1MessageID
}

func (m *MessageMetadata) GetParent2MessageID() Hash {
	return m.parent2MessageID
}

func (m *MessageMetadata) GetSolidificationTimestamp() int32 {
	m.RLock()
	defer m.RUnlock()

	return m.solidificationTimestamp
}

func (m *MessageMetadata) IsSolid() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(MessageMetadataSolid)
}

func (m *MessageMetadata) SetSolid(solid bool) {
	m.Lock()
	defer m.Unlock()

	if solid != m.metadata.HasBit(MessageMetadataSolid) {
		if solid {
			m.solidificationTimestamp = int32(time.Now().Unix())
		} else {
			m.solidificationTimestamp = 0
		}
		m.metadata = m.metadata.ModifyBit(MessageMetadataSolid, solid)
		m.SetModified(true)
	}
}

func (m *MessageMetadata) IsConfirmed() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(MessageMetadataConfirmed)
}

func (m *MessageMetadata) GetConfirmed() (bool, milestone.Index) {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(MessageMetadataConfirmed), m.confirmationIndex
}

func (m *MessageMetadata) SetConfirmed(confirmed bool, confirmationIndex milestone.Index) {
	m.Lock()
	defer m.Unlock()

	if confirmed != m.metadata.HasBit(MessageMetadataConfirmed) {
		if confirmed {
			m.confirmationIndex = confirmationIndex
		} else {
			m.confirmationIndex = 0
		}
		m.metadata = m.metadata.ModifyBit(MessageMetadataConfirmed, confirmed)
		m.SetModified(true)
	}
}

func (m *MessageMetadata) IsConflicting() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(MessageMetadataConflicting)
}

func (m *MessageMetadata) SetConflicting(conflicting bool) {
	m.Lock()
	defer m.Unlock()

	if conflicting != m.metadata.HasBit(MessageMetadataConflicting) {
		m.metadata = m.metadata.ModifyBit(MessageMetadataConflicting, conflicting)
		m.SetModified(true)
	}
}

func (m *MessageMetadata) SetRootSnapshotIndexes(ymrsi milestone.Index, omrsi milestone.Index, rtsci milestone.Index) {
	m.Lock()
	defer m.Unlock()

	m.youngestRootSnapshotIndex = ymrsi
	m.oldestRootSnapshotIndex = omrsi
	m.rootSnapshotCalculationIndex = rtsci
	m.SetModified(true)
}

func (m *MessageMetadata) GetRootSnapshotIndexes() (ymrsi milestone.Index, omrsi milestone.Index, rtsci milestone.Index) {
	m.RLock()
	defer m.RUnlock()

	return m.youngestRootSnapshotIndex, m.oldestRootSnapshotIndex, m.rootSnapshotCalculationIndex
}

func (m *MessageMetadata) GetMetadata() byte {
	m.RLock()
	defer m.RUnlock()

	return byte(m.metadata)
}

// ObjectStorage interface

func (m *MessageMetadata) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("MessageMetadata should never be updated: %v", m.messageID.Hex()))
}

func (m *MessageMetadata) ObjectStorageKey() []byte {
	return m.messageID
}

func (m *MessageMetadata) ObjectStorageValue() (data []byte) {
	m.Lock()
	defer m.Unlock()

	/*
		1 byte  metadata bitmask
		4 bytes uint32 solidificationTimestamp
		4 bytes uint32 confirmationIndex
		4 bytes uint32 youngestRootSnapshotIndex
		4 bytes uint32 oldestRootSnapshotIndex
		4 bytes uint32 rootSnapshotCalculationIndex
		32 bytes parent1 id
		32 bytes parent2 id
	*/

	value := make([]byte, 21)
	value[0] = byte(m.metadata)
	binary.LittleEndian.PutUint32(value[1:], uint32(m.solidificationTimestamp))
	binary.LittleEndian.PutUint32(value[5:], uint32(m.confirmationIndex))
	binary.LittleEndian.PutUint32(value[9:], uint32(m.youngestRootSnapshotIndex))
	binary.LittleEndian.PutUint32(value[13:], uint32(m.oldestRootSnapshotIndex))
	binary.LittleEndian.PutUint32(value[17:], uint32(m.rootSnapshotCalculationIndex))
	value = append(value, m.parent1MessageID...)
	value = append(value, m.parent2MessageID...)

	return value
}

func MetadataFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {

	/*
		1 byte  metadata bitmask
		4 bytes uint32 solidificationTimestamp
		4 bytes uint32 confirmationIndex
		4 bytes uint32 youngestRootSnapshotIndex
		4 bytes uint32 oldestRootSnapshotIndex
		4 bytes uint32 rootSnapshotCalculationIndex
		32 bytes parent1 id
		32 bytes parent2 id
	*/

	m := NewMessageMetadata(key[:32], data[21:21+32], data[21+32:21+32+32])

	m.metadata = bitmask.BitMask(data[0])
	m.solidificationTimestamp = int32(binary.LittleEndian.Uint32(data[1:5]))
	m.confirmationIndex = milestone.Index(binary.LittleEndian.Uint32(data[5:9]))
	m.youngestRootSnapshotIndex = milestone.Index(binary.LittleEndian.Uint32(data[9:13]))
	m.oldestRootSnapshotIndex = milestone.Index(binary.LittleEndian.Uint32(data[13:17]))
	m.rootSnapshotCalculationIndex = milestone.Index(binary.LittleEndian.Uint32(data[17:21]))

	return m, nil
}
