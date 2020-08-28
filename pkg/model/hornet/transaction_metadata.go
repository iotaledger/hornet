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
	TransactionMetadataIsHead      = 3
	TransactionMetadataIsTail      = 4
	TransactionMetadataIsValue     = 5
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

	// trunkHash is the trunk of the transaction
	trunkHash Hash

	// branchHash is the branch of the transaction
	branchHash Hash

	// bundleHash is the bundle of the transaction
	bundleHash Hash
}

func NewTransactionMetadata(txHash Hash) *TransactionMetadata {
	return &TransactionMetadata{
		txHash: txHash,
	}
}

func (m *TransactionMetadata) GetTxHash() Hash {
	return m.txHash
}

func (m *TransactionMetadata) GetTrunkHash() Hash {
	return m.trunkHash
}

func (m *TransactionMetadata) GetBranchHash() Hash {
	return m.branchHash
}

func (m *TransactionMetadata) GetBundleHash() Hash {
	return m.bundleHash
}

func (m *TransactionMetadata) IsTail() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(TransactionMetadataIsTail)
}

func (m *TransactionMetadata) IsHead() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(TransactionMetadataIsHead)
}

func (m *TransactionMetadata) IsValue() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(TransactionMetadataIsValue)
}

func (m *TransactionMetadata) GetSolidificationTimestamp() int32 {
	m.RLock()
	defer m.RUnlock()

	return m.solidificationTimestamp
}

func (m *TransactionMetadata) IsSolid() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(TransactionMetadataSolid)
}

func (m *TransactionMetadata) SetSolid(solid bool) {
	m.Lock()
	defer m.Unlock()

	if solid != m.metadata.HasBit(TransactionMetadataSolid) {
		if solid {
			m.solidificationTimestamp = int32(time.Now().Unix())
		} else {
			m.solidificationTimestamp = 0
		}
		m.metadata = m.metadata.ModifyBit(TransactionMetadataSolid, solid)
		m.SetModified(true)
	}
}

func (m *TransactionMetadata) IsConfirmed() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(TransactionMetadataConfirmed)
}

func (m *TransactionMetadata) GetConfirmed() (bool, milestone.Index) {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(TransactionMetadataConfirmed), m.confirmationIndex
}

func (m *TransactionMetadata) SetConfirmed(confirmed bool, confirmationIndex milestone.Index) {
	m.Lock()
	defer m.Unlock()

	if confirmed != m.metadata.HasBit(TransactionMetadataConfirmed) {
		if confirmed {
			m.confirmationIndex = confirmationIndex
		} else {
			m.confirmationIndex = 0
		}
		m.metadata = m.metadata.ModifyBit(TransactionMetadataConfirmed, confirmed)
		m.SetModified(true)
	}
}

func (m *TransactionMetadata) IsConflicting() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(TransactionMetadataConflicting)
}

func (m *TransactionMetadata) SetConflicting(conflicting bool) {
	m.Lock()
	defer m.Unlock()

	if conflicting != m.metadata.HasBit(TransactionMetadataConflicting) {
		m.metadata = m.metadata.ModifyBit(TransactionMetadataConflicting, conflicting)
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

func (m *TransactionMetadata) SetAdditionalTxInfo(trunkHash Hash, branchHash Hash, bundleHash Hash, isHead bool, isTail bool, isValue bool) {
	m.Lock()
	defer m.Unlock()

	m.trunkHash = trunkHash
	m.branchHash = branchHash
	m.bundleHash = bundleHash
	m.metadata = m.metadata.ModifyBit(TransactionMetadataIsHead, isHead).ModifyBit(TransactionMetadataIsTail, isTail).ModifyBit(TransactionMetadataIsValue, isValue)
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
		49 bytes hash trunk
		49 bytes hash branch
		49 bytes hash bundle
	*/

	value := make([]byte, 21)
	value[0] = byte(m.metadata)
	binary.LittleEndian.PutUint32(value[1:], uint32(m.solidificationTimestamp))
	binary.LittleEndian.PutUint32(value[5:], uint32(m.confirmationIndex))
	binary.LittleEndian.PutUint32(value[9:], uint32(m.youngestRootSnapshotIndex))
	binary.LittleEndian.PutUint32(value[13:], uint32(m.oldestRootSnapshotIndex))
	binary.LittleEndian.PutUint32(value[17:], uint32(m.rootSnapshotCalculationIndex))
	value = append(value, m.trunkHash...)
	value = append(value, m.branchHash...)
	value = append(value, m.bundleHash...)

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
		49 bytes hash trunk
		49 bytes hash branch
		49 bytes hash bundle
	*/

	m.metadata = bitmask.BitMask(data[0])
	m.solidificationTimestamp = int32(binary.LittleEndian.Uint32(data[1:5]))
	m.confirmationIndex = milestone.Index(binary.LittleEndian.Uint32(data[5:9]))
	m.youngestRootSnapshotIndex = milestone.Index(binary.LittleEndian.Uint32(data[9:13]))
	m.oldestRootSnapshotIndex = milestone.Index(binary.LittleEndian.Uint32(data[13:17]))
	m.rootSnapshotCalculationIndex = 0

	if len(data) > 17 {
		// ToDo: Remove at next DbVersion update
		m.rootSnapshotCalculationIndex = milestone.Index(binary.LittleEndian.Uint32(data[17:21]))

		if len(data) == 21+49+49+49 {
			m.trunkHash = Hash(data[21 : 21+49])
			m.branchHash = Hash(data[21+49 : 21+49+49])
			m.bundleHash = Hash(data[21+49+49 : 21+49+49+49])
		}
	}

	return len(data), nil
}
