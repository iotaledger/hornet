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
	TransactionMetadataSolid       = 0
	TransactionMetadataConfirmed   = 1
	TransactionMetadataConflicting = 2
)

type TransactionMetadata struct {
	objectstorage.StorableObjectFlags
	syncutils.RWMutex

	txHash Hash

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

	// rootSnapshotCalculationIndex is the solid index yrtsi and ortsi were calculated at
	rootSnapshotCalculationIndex milestone.Index
}

func NewTransactionMetadata(txHash Hash) *TransactionMetadata {
	return &TransactionMetadata{
		txHash: txHash,
	}
}

func (m *TransactionMetadata) GetTxHash() Hash {
	return m.txHash
}

func (m *TransactionMetadata) GetSolidificationTimestamp() int32 {
	m.RLock()
	defer m.RUnlock()

	return m.solidificationTimestamp
}

func (m *TransactionMetadata) IsSolid() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasFlag(TransactionMetadataSolid)
}

func (m *TransactionMetadata) SetSolid(solid bool) {
	m.Lock()
	defer m.Unlock()

	if solid != m.metadata.HasFlag(TransactionMetadataSolid) {
		if solid {
			m.solidificationTimestamp = int32(time.Now().Unix())
		} else {
			m.solidificationTimestamp = 0
		}
		m.metadata = m.metadata.ModifyFlag(TransactionMetadataSolid, solid)
		m.SetModified(true)
	}
}

func (m *TransactionMetadata) IsConfirmed() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasFlag(TransactionMetadataConfirmed)
}

func (m *TransactionMetadata) GetConfirmed() (bool, milestone.Index) {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasFlag(TransactionMetadataConfirmed), m.confirmationIndex
}

func (m *TransactionMetadata) SetConfirmed(confirmed bool, confirmationIndex milestone.Index) {
	m.Lock()
	defer m.Unlock()

	if confirmed != m.metadata.HasFlag(TransactionMetadataConfirmed) {
		if confirmed {
			m.confirmationIndex = confirmationIndex
		} else {
			m.confirmationIndex = 0
		}
		m.metadata = m.metadata.ModifyFlag(TransactionMetadataConfirmed, confirmed)
		m.SetModified(true)
	}
}

func (m *TransactionMetadata) IsConflicting() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasFlag(TransactionMetadataConflicting)
}

func (m *TransactionMetadata) SetConflicting(conflicting bool) {
	m.Lock()
	defer m.Unlock()

	if conflicting != m.metadata.HasFlag(TransactionMetadataConflicting) {
		m.metadata = m.metadata.ModifyFlag(TransactionMetadataConflicting, conflicting)
		m.SetModified(true)
	}
}

func (m *TransactionMetadata) SetRootSnapshotIndexes(yrtsi milestone.Index, ortsi milestone.Index, rtsci milestone.Index) {
	m.Lock()
	defer m.Unlock()

	m.youngestRootSnapshotIndex = yrtsi
	m.oldestRootSnapshotIndex = ortsi
	m.rootSnapshotCalculationIndex = rtsci
	m.SetModified(true)
}

func (m *TransactionMetadata) GetRootSnapshotIndexes() (yrtsi milestone.Index, ortsi milestone.Index, rtsci milestone.Index) {
	m.RLock()
	defer m.RUnlock()

	return m.youngestRootSnapshotIndex, m.oldestRootSnapshotIndex, m.rootSnapshotCalculationIndex
}

func (m *TransactionMetadata) Reset() {
	m.Lock()
	defer m.Unlock()

	// Metadata
	m.metadata = bitmask.BitMask(0)
	m.solidificationTimestamp = 0
	m.confirmationIndex = 0
	m.youngestRootSnapshotIndex = 0
	m.oldestRootSnapshotIndex = 0
	m.rootSnapshotCalculationIndex = 0
	m.SetModified(true)
}

func (m *TransactionMetadata) GetMetadata() byte {
	m.RLock()
	defer m.RUnlock()

	return byte(m.metadata)
}

// ObjectStorage interface

func (m *TransactionMetadata) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("TransactionMetadata should never be updated: %v", m.txHash.Trytes()))
}

func (m *TransactionMetadata) ObjectStorageKey() []byte {
	return m.txHash
}

func (m *TransactionMetadata) ObjectStorageValue() (data []byte) {
	m.Lock()
	defer m.Unlock()

	/*
		1 byte  metadata bitmask
		4 bytes uint32 solidificationTimestamp
		4 bytes uint32 confirmationIndex
		4 bytes uint32 youngestRootSnapshotIndex
		4 bytes uint32 oldestRootSnapshotIndex
		4 bytes uint32 rootSnapshotCalculationIndex
	*/

	value := make([]byte, 21)
	value[0] = byte(m.metadata)
	binary.LittleEndian.PutUint32(value[1:], uint32(m.solidificationTimestamp))
	binary.LittleEndian.PutUint32(value[5:], uint32(m.confirmationIndex))
	binary.LittleEndian.PutUint32(value[9:], uint32(m.youngestRootSnapshotIndex))
	binary.LittleEndian.PutUint32(value[13:], uint32(m.oldestRootSnapshotIndex))
	binary.LittleEndian.PutUint32(value[17:], uint32(m.rootSnapshotCalculationIndex))

	return value
}

func (m *TransactionMetadata) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	m.Lock()
	defer m.Unlock()

	/*
		1 byte  metadata bitmask
		4 bytes uint32 solidificationTimestamp
		4 bytes uint32 confirmationIndex
		4 bytes uint32 youngestRootSnapshotIndex
		4 bytes uint32 oldestRootSnapshotIndex
		4 bytes uint32 rootSnapshotCalculationIndex
	*/

	m.metadata = bitmask.BitMask(data[0])
	m.solidificationTimestamp = int32(binary.LittleEndian.Uint32(data[1:5]))
	m.confirmationIndex = milestone.Index(binary.LittleEndian.Uint32(data[5:9]))
	m.youngestRootSnapshotIndex = milestone.Index(binary.LittleEndian.Uint32(data[9:13]))
	m.oldestRootSnapshotIndex = milestone.Index(binary.LittleEndian.Uint32(data[13:17]))
	m.rootSnapshotCalculationIndex = 0

	if len(data) == 21 {
		// ToDo: Remove at next DbVersion update
		m.rootSnapshotCalculationIndex = milestone.Index(binary.LittleEndian.Uint32(data[17:21]))
		return 21, nil
	}

	return 17, nil
}
