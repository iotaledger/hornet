package storage

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

const (
	MessageMetadataSolid         = 0
	MessageMetadataReferenced    = 1
	MessageMetadataNoTx          = 2
	MessageMetadataConflictingTx = 3
	MessageMetadataMilestone     = 4
)

// Conflict defines the reason why a message is marked as conflicting.
type Conflict uint8

const (
	//ConflictNone the message has no conflict.
	ConflictNone Conflict = iota

	// ConflictInputUTXOAlreadySpent the referenced UTXO was already spent.
	ConflictInputUTXOAlreadySpent = 1

	// ConflictInputUTXOAlreadySpentInThisMilestone the referenced UTXO was already spent while confirming this milestone
	ConflictInputUTXOAlreadySpentInThisMilestone = 2

	// ConflictInputUTXONotFound the referenced UTXO cannot be found.
	ConflictInputUTXONotFound = 3

	// ConflictInputOutputSumMismatch the sum of the inputs and output values does not match.
	ConflictInputOutputSumMismatch = 4

	// ConflictInvalidSignature the unlock block signature is invalid.
	ConflictInvalidSignature = 5

	// ConflictInvalidDustAllowance the dust allowance for the address is invalid.
	ConflictInvalidDustAllowance = 6

	// ConflictSemanticValidationFailed the semantic validation failed.
	ConflictSemanticValidationFailed = 255
)

type MessageMetadata struct {
	objectstorage.StorableObjectFlags
	syncutils.RWMutex

	messageID *hornet.MessageID

	// Metadata
	metadata bitmask.BitMask

	// Unix time when the Tx became solid (needed for local modifiers for tipselection)
	solidificationTimestamp int32

	// The index of the milestone which referenced this msg
	referencedIndex milestone.Index

	conflict Conflict

	// youngestConeRootIndex is the highest referenced index of the past cone of this message
	youngestConeRootIndex milestone.Index

	// oldestConeRootIndex is the lowest referenced index of the past cone of this message
	oldestConeRootIndex milestone.Index

	// coneRootCalculationIndex is the solid index ycri and ocri were calculated at
	coneRootCalculationIndex milestone.Index

	// parent1MessageID is the parent1 (trunk) of the message
	parent1MessageID *hornet.MessageID

	// parent2MessageID is the parent2 (branch) of the message
	parent2MessageID *hornet.MessageID
}

func NewMessageMetadata(messageID *hornet.MessageID, parent1MessageID *hornet.MessageID, parent2MessageID *hornet.MessageID) *MessageMetadata {
	return &MessageMetadata{
		messageID:        messageID,
		parent1MessageID: parent1MessageID,
		parent2MessageID: parent2MessageID,
	}
}

func (m *MessageMetadata) GetMessageID() *hornet.MessageID {
	return m.messageID
}

func (m *MessageMetadata) GetParent1MessageID() *hornet.MessageID {
	return m.parent1MessageID
}

func (m *MessageMetadata) GetParent2MessageID() *hornet.MessageID {
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

func (m *MessageMetadata) IsIncludedTxInLedger() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(MessageMetadataReferenced) && !m.metadata.HasBit(MessageMetadataNoTx) && !m.metadata.HasBit(MessageMetadataConflictingTx)
}

func (m *MessageMetadata) IsReferenced() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(MessageMetadataReferenced)
}

func (m *MessageMetadata) GetReferenced() (bool, milestone.Index) {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(MessageMetadataReferenced), m.referencedIndex
}

func (m *MessageMetadata) SetReferenced(referenced bool, referencedIndex milestone.Index) {
	m.Lock()
	defer m.Unlock()

	if referenced != m.metadata.HasBit(MessageMetadataReferenced) {
		if referenced {
			m.referencedIndex = referencedIndex
		} else {
			m.referencedIndex = 0
		}
		m.metadata = m.metadata.ModifyBit(MessageMetadataReferenced, referenced)
		m.SetModified(true)
	}
}

func (m *MessageMetadata) IsNoTransaction() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(MessageMetadataNoTx)
}

func (m *MessageMetadata) SetIsNoTransaction(noTx bool) {
	m.Lock()
	defer m.Unlock()

	if noTx != m.metadata.HasBit(MessageMetadataNoTx) {
		m.metadata = m.metadata.ModifyBit(MessageMetadataNoTx, noTx)
		m.SetModified(true)
	}
}

func (m *MessageMetadata) IsConflictingTx() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(MessageMetadataConflictingTx)
}

func (m *MessageMetadata) SetConflictingTx(conflict Conflict) {
	m.Lock()
	defer m.Unlock()

	conflictingTx := conflict != ConflictNone

	if conflictingTx != m.metadata.HasBit(MessageMetadataConflictingTx) ||
		m.conflict != conflict {
		m.metadata = m.metadata.ModifyBit(MessageMetadataConflictingTx, conflictingTx)
		m.conflict = conflict
		m.SetModified(true)
	}
}

func (m *MessageMetadata) GetConflict() Conflict {
	m.RLock()
	defer m.RUnlock()

	return m.conflict
}

func (m *MessageMetadata) IsMilestone() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(MessageMetadataMilestone)
}

func (m *MessageMetadata) SetMilestone(milestone bool) {
	m.Lock()
	defer m.Unlock()

	if milestone != m.metadata.HasBit(MessageMetadataMilestone) {
		m.metadata = m.metadata.ModifyBit(MessageMetadataMilestone, milestone)
		m.SetModified(true)
	}
}

func (m *MessageMetadata) SetConeRootIndexes(ycri milestone.Index, ocri milestone.Index, ci milestone.Index) {
	m.Lock()
	defer m.Unlock()

	m.youngestConeRootIndex = ycri
	m.oldestConeRootIndex = ocri
	m.coneRootCalculationIndex = ci
	m.SetModified(true)
}

func (m *MessageMetadata) GetConeRootIndexes() (ycri milestone.Index, ocri milestone.Index, ci milestone.Index) {
	m.RLock()
	defer m.RUnlock()

	return m.youngestConeRootIndex, m.oldestConeRootIndex, m.coneRootCalculationIndex
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
	return m.messageID.Slice()
}

func (m *MessageMetadata) ObjectStorageValue() (data []byte) {
	m.Lock()
	defer m.Unlock()

	/*
		1 byte  metadata bitmask
		4 bytes uint32 solidificationTimestamp
		4 bytes uint32 referencedIndex
		1 byte  uint8 conflict
		4 bytes uint32 youngestConeRootIndex
		4 bytes uint32 oldestConeRootIndex
		4 bytes uint32 coneRootCalculationIndex
		32 bytes parent1 id
		32 bytes parent2 id
	*/

	value := make([]byte, 22)
	value[0] = byte(m.metadata)
	binary.LittleEndian.PutUint32(value[1:], uint32(m.solidificationTimestamp))
	binary.LittleEndian.PutUint32(value[5:], uint32(m.referencedIndex))
	value[9] = byte(m.conflict)
	binary.LittleEndian.PutUint32(value[10:], uint32(m.youngestConeRootIndex))
	binary.LittleEndian.PutUint32(value[14:], uint32(m.oldestConeRootIndex))
	binary.LittleEndian.PutUint32(value[18:], uint32(m.coneRootCalculationIndex))
	value = append(value, m.parent1MessageID.Slice()...)
	value = append(value, m.parent2MessageID.Slice()...)

	return value
}

func MetadataFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {

	/*
		1 byte  metadata bitmask
		4 bytes uint32 solidificationTimestamp
		4 bytes uint32 referencedIndex
		1 byte  uint8 conflict
		4 bytes uint32 youngestConeRootIndex
		4 bytes uint32 oldestConeRootIndex
		4 bytes uint32 coneRootCalculationIndex
		32 bytes parent1 id
		32 bytes parent2 id
	*/

	m := NewMessageMetadata(hornet.MessageIDFromBytes(key[:32]), hornet.MessageIDFromBytes(data[22:22+32]), hornet.MessageIDFromBytes(data[22+32:22+32+32]))

	m.metadata = bitmask.BitMask(data[0])
	m.solidificationTimestamp = int32(binary.LittleEndian.Uint32(data[1:5]))
	m.referencedIndex = milestone.Index(binary.LittleEndian.Uint32(data[5:9]))
	m.conflict = Conflict(data[9])
	m.youngestConeRootIndex = milestone.Index(binary.LittleEndian.Uint32(data[10:14]))
	m.oldestConeRootIndex = milestone.Index(binary.LittleEndian.Uint32(data[14:18]))
	m.coneRootCalculationIndex = milestone.Index(binary.LittleEndian.Uint32(data[18:22]))

	return m, nil
}
