package storage

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	BlockMetadataSolid         = 0
	BlockMetadataReferenced    = 1
	BlockMetadataNoTx          = 2
	BlockMetadataConflictingTx = 3
	BlockMetadataMilestone     = 4
)

// Conflict defines the reason why a block is marked as conflicting.
type Conflict uint8

const (
	// ConflictNone the block has no conflict.
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

	// ConflictTimelockNotExpired the configured timelock is not yet expired.
	ConflictTimelockNotExpired = 6

	// ConflictInvalidNativeTokens the given native tokens are invalid.
	ConflictInvalidNativeTokens = 7

	// ConflictReturnAmountNotFulfilled return amount in a transaction is not fulfilled by the output side.
	ConflictReturnAmountNotFulfilled = 8

	// ConflictInvalidInputUnlock input unlock is invalid.
	ConflictInvalidInputUnlock = 9

	// ConflictInvalidInputsCommitment the inputs commitment is invalid.
	ConflictInvalidInputsCommitment = 10

	// ConflictInvalidSender an output contains a Sender with an ident which is not unlocked.
	ConflictInvalidSender = 11

	// ConflictInvalidChainStateTransition the chain state transition is invalid.
	ConflictInvalidChainStateTransition = 12

	// ConflictSemanticValidationFailed the semantic validation failed.
	ConflictSemanticValidationFailed = 255
)

var errorToConflictMapping = map[error]Conflict{
	// Input validation
	iotago.ErrMissingUTXO:             ConflictInputUTXONotFound,
	iotago.ErrInputOutputSumMismatch:  ConflictInputOutputSumMismatch,
	iotago.ErrInvalidInputsCommitment: ConflictInvalidInputsCommitment,

	// Deposit
	iotago.ErrReturnAmountNotFulFilled: ConflictReturnAmountNotFulfilled,

	// Signature validation
	iotago.ErrEd25519SignatureInvalid:      ConflictInvalidSignature,
	iotago.ErrEd25519PubKeyAndAddrMismatch: ConflictInvalidSignature,

	// Timelocks
	iotago.ErrTimelockNotExpired: ConflictTimelockNotExpired,

	// Native tokens
	iotago.ErrNativeTokenAmountLessThanEqualZero: ConflictInvalidNativeTokens,
	iotago.ErrNativeTokenSumExceedsUint256:       ConflictInvalidNativeTokens,
	iotago.ErrMaxNativeTokensCountExceeded:       ConflictInvalidNativeTokens,
	iotago.ErrNativeTokenSumUnbalanced:           ConflictInvalidNativeTokens,

	// Sender validation
	iotago.ErrSenderFeatureNotUnlocked: ConflictInvalidSender,

	// Unlock validation
	iotago.ErrInvalidInputUnlock: ConflictInvalidInputUnlock,
}

func ConflictFromSemanticValidationError(err error) Conflict {
	var chainError *iotago.ChainTransitionError
	if errors.As(err, &chainError) {
		return ConflictInvalidChainStateTransition
	}

	for errKind, conflict := range errorToConflictMapping {
		if errors.Is(err, errKind) {
			return conflict
		}
	}

	return ConflictSemanticValidationFailed
}

type BlockMetadata struct {
	objectstorage.StorableObjectFlags
	syncutils.RWMutex

	blockID hornet.BlockID

	// Metadata
	metadata bitmask.BitMask

	// The index of the milestone which referenced this msg
	referencedIndex milestone.Index

	conflict Conflict

	// youngestConeRootIndex is the highest referenced index of the past cone of this block
	youngestConeRootIndex milestone.Index

	// oldestConeRootIndex is the lowest referenced index of the past cone of this block
	oldestConeRootIndex milestone.Index

	// coneRootCalculationIndex is the confirmed milestone index ycri and ocri were calculated at
	coneRootCalculationIndex milestone.Index

	// parents are the parents of the block
	parents hornet.BlockIDs
}

func NewBlockMetadata(blockID hornet.BlockID, parents hornet.BlockIDs) *BlockMetadata {
	return &BlockMetadata{
		blockID: blockID,
		parents: parents,
	}
}

func (m *BlockMetadata) BlockID() hornet.BlockID {
	return m.blockID
}

func (m *BlockMetadata) Parents() hornet.BlockIDs {
	return m.parents
}

func (m *BlockMetadata) IsSolid() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(BlockMetadataSolid)
}

func (m *BlockMetadata) SetSolid(solid bool) {
	m.Lock()
	defer m.Unlock()

	if solid != m.metadata.HasBit(BlockMetadataSolid) {
		m.metadata = m.metadata.ModifyBit(BlockMetadataSolid, solid)
		m.SetModified(true)
	}
}

func (m *BlockMetadata) IsIncludedTxInLedger() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(BlockMetadataReferenced) && !m.metadata.HasBit(BlockMetadataNoTx) && !m.metadata.HasBit(BlockMetadataConflictingTx)
}

func (m *BlockMetadata) IsReferenced() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(BlockMetadataReferenced)
}

func (m *BlockMetadata) ReferencedWithIndex() (bool, milestone.Index) {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(BlockMetadataReferenced), m.referencedIndex
}

func (m *BlockMetadata) SetReferenced(referenced bool, referencedIndex milestone.Index) {
	m.Lock()
	defer m.Unlock()

	if referenced != m.metadata.HasBit(BlockMetadataReferenced) {
		if referenced {
			m.referencedIndex = referencedIndex
		} else {
			m.referencedIndex = 0
		}
		m.metadata = m.metadata.ModifyBit(BlockMetadataReferenced, referenced)
		m.SetModified(true)
	}
}

func (m *BlockMetadata) IsNoTransaction() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(BlockMetadataNoTx)
}

func (m *BlockMetadata) SetIsNoTransaction(noTx bool) {
	m.Lock()
	defer m.Unlock()

	if noTx != m.metadata.HasBit(BlockMetadataNoTx) {
		m.metadata = m.metadata.ModifyBit(BlockMetadataNoTx, noTx)
		m.SetModified(true)
	}
}

func (m *BlockMetadata) IsConflictingTx() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(BlockMetadataConflictingTx)
}

func (m *BlockMetadata) SetConflictingTx(conflict Conflict) {
	m.Lock()
	defer m.Unlock()

	conflictingTx := conflict != ConflictNone

	if conflictingTx != m.metadata.HasBit(BlockMetadataConflictingTx) ||
		m.conflict != conflict {
		m.metadata = m.metadata.ModifyBit(BlockMetadataConflictingTx, conflictingTx)
		m.conflict = conflict
		m.SetModified(true)
	}
}

func (m *BlockMetadata) Conflict() Conflict {
	m.RLock()
	defer m.RUnlock()

	return m.conflict
}

func (m *BlockMetadata) IsMilestone() bool {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(BlockMetadataMilestone)
}

func (m *BlockMetadata) SetMilestone(milestone bool) {
	m.Lock()
	defer m.Unlock()

	if milestone != m.metadata.HasBit(BlockMetadataMilestone) {
		m.metadata = m.metadata.ModifyBit(BlockMetadataMilestone, milestone)
		m.SetModified(true)
	}
}

func (m *BlockMetadata) SetConeRootIndexes(ycri milestone.Index, ocri milestone.Index, ci milestone.Index) {
	m.Lock()
	defer m.Unlock()

	m.youngestConeRootIndex = ycri
	m.oldestConeRootIndex = ocri
	m.coneRootCalculationIndex = ci
	m.SetModified(true)
}

func (m *BlockMetadata) ConeRootIndexes() (ycri milestone.Index, ocri milestone.Index, ci milestone.Index) {
	m.RLock()
	defer m.RUnlock()

	return m.youngestConeRootIndex, m.oldestConeRootIndex, m.coneRootCalculationIndex
}

func (m *BlockMetadata) Metadata() byte {
	m.RLock()
	defer m.RUnlock()

	return byte(m.metadata)
}

// ObjectStorage interface

func (m *BlockMetadata) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("BlockMetadata should never be updated: %v", m.blockID.ToHex()))
}

func (m *BlockMetadata) ObjectStorageKey() []byte {
	return m.blockID
}

func (m *BlockMetadata) ObjectStorageValue() (data []byte) {
	m.Lock()
	defer m.Unlock()

	/*
		1 byte  metadata bitmask
		4 bytes uint32 referencedIndex
		1 byte  uint8 conflict
		4 bytes uint32 youngestConeRootIndex
		4 bytes uint32 oldestConeRootIndex
		4 bytes uint32 coneRootCalculationIndex
		1 byte  parents count
		parents count * 32 bytes parent id
	*/

	marshalUtil := marshalutil.New(19 + len(m.parents)*iotago.BlockIDLength)

	marshalUtil.WriteByte(byte(m.metadata))
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

	m := &BlockMetadata{
		blockID: hornet.BlockIDFromSlice(key[:32]),
	}

	m.metadata = bitmask.BitMask(metadataByte)
	m.referencedIndex = milestone.Index(referencedIndex)
	m.conflict = Conflict(conflict)
	m.youngestConeRootIndex = milestone.Index(youngestConeRootIndex)
	m.oldestConeRootIndex = milestone.Index(oldestConeRootIndex)
	m.coneRootCalculationIndex = milestone.Index(coneRootCalculationIndex)

	parentsCount, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, err
	}

	m.parents = make(hornet.BlockIDs, parentsCount)
	for i := 0; i < int(parentsCount); i++ {
		parentBytes, err := marshalUtil.ReadBytes(iotago.BlockIDLength)
		if err != nil {
			return nil, err
		}

		parent := hornet.BlockIDFromSlice(parentBytes)
		m.parents[i] = parent
	}

	return m, nil
}
