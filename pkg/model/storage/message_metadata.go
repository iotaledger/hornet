package storage

import (
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go/v3"
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

	// ConflictInvalidNetworkID the networkId in the essence does not match this nodes configuration.
	ConflictInvalidNetworkID = 6

	// ConflictSemanticValidationFailed the semantic validation failed.
	ConflictSemanticValidationFailed = 255
)

type MessageMetadata struct {
	objectstorage.StorableObjectFlags
	syncutils.RWMutex

	messageID hornet.MessageID

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

	// coneRootCalculationIndex is the confirmed milestone index ycri and ocri were calculated at
	coneRootCalculationIndex milestone.Index

	// parents are the parents of the message
	parents hornet.MessageIDs
}

func NewMessageMetadata(messageID hornet.MessageID, parents hornet.MessageIDs) *MessageMetadata {
	return &MessageMetadata{
		messageID: messageID,
		parents:   parents,
	}
}

func (m *MessageMetadata) MessageID() hornet.MessageID {
	return m.messageID
}

func (m *MessageMetadata) Parents() hornet.MessageIDs {
	return m.parents
}

func (m *MessageMetadata) SolidificationTimestamp() int32 {
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

func (m *MessageMetadata) ReferencedWithIndex() (bool, milestone.Index) {
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

func (m *MessageMetadata) Conflict() Conflict {
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

func (m *MessageMetadata) ConeRootIndexes() (ycri milestone.Index, ocri milestone.Index, ci milestone.Index) {
	m.RLock()
	defer m.RUnlock()

	return m.youngestConeRootIndex, m.oldestConeRootIndex, m.coneRootCalculationIndex
}

func (m *MessageMetadata) Metadata() byte {
	m.RLock()
	defer m.RUnlock()

	return byte(m.metadata)
}

// ObjectStorage interface

func (m *MessageMetadata) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("MessageMetadata should never be updated: %v", m.messageID.ToHex()))
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
		4 bytes uint32 referencedIndex
		1 byte  uint8 conflict
		4 bytes uint32 youngestConeRootIndex
		4 bytes uint32 oldestConeRootIndex
		4 bytes uint32 coneRootCalculationIndex
		1 byte  parents count
		parents count * 32 bytes parent id
	*/

	marshalUtil := marshalutil.New(23 + len(m.parents)*iotago.MessageIDLength)

	marshalUtil.WriteByte(byte(m.metadata))
	marshalUtil.WriteUint32(uint32(m.solidificationTimestamp))
	marshalUtil.WriteUint32(uint32(m.referencedIndex))
	marshalUtil.WriteByte(byte(m.conflict))
	marshalUtil.WriteUint32(uint32(m.youngestConeRootIndex))
	marshalUtil.WriteUint32(uint32(m.oldestConeRootIndex))
	marshalUtil.WriteUint32(uint32(m.coneRootCalculationIndex))
	marshalUtil.WriteByte(byte(len(m.parents)))
	for _, parent := range m.parents {
		marshalUtil.WriteBytes(parent[:])
	}

	return marshalUtil.Bytes()
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
		1 byte  parents count
		parents count * 32 bytes parent id
	*/

	marshalUtil := marshalutil.New(data)

	metadataByte, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, err
	}

	solidificationTimestamp, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	referencedIndex, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	conflict, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, err
	}

	youngestConeRootIndex, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	oldestConeRootIndex, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	coneRootCalculationIndex, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	m := &MessageMetadata{
		messageID: hornet.MessageIDFromSlice(key[:32]),
	}

	m.metadata = bitmask.BitMask(metadataByte)
	m.solidificationTimestamp = int32(solidificationTimestamp)
	m.referencedIndex = milestone.Index(referencedIndex)
	m.conflict = Conflict(conflict)
	m.youngestConeRootIndex = milestone.Index(youngestConeRootIndex)
	m.oldestConeRootIndex = milestone.Index(oldestConeRootIndex)
	m.coneRootCalculationIndex = milestone.Index(coneRootCalculationIndex)

	parentsCount, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, err
	}

	m.parents = make(hornet.MessageIDs, parentsCount)
	for i := 0; i < int(parentsCount); i++ {
		parentBytes, err := marshalUtil.ReadBytes(iotago.MessageIDLength)
		if err != nil {
			return nil, err
		}

		parent := hornet.MessageIDFromSlice(parentBytes)
		m.parents[i] = parent
	}

	return m, nil
}
