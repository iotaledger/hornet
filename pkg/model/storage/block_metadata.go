package storage

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/bitmask"
	"github.com/iotaledger/hive.go/core/marshalutil"
	"github.com/iotaledger/hive.go/core/objectstorage"
	"github.com/iotaledger/hive.go/core/syncutils"
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

	// ConflictInputUTXOAlreadySpentInThisMilestone the referenced UTXO was already spent while confirming this milestone.
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

	// Chain transitions
	iotago.ErrTransDepIdentOutputNextInvalid: ConflictInvalidChainStateTransition,
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

	blockID iotago.BlockID

	// Metadata.
	metadata bitmask.BitMask

	// The index of the milestone which referenced this block.
	referencedIndex iotago.MilestoneIndex

	// The index of this block inside the milestone given the whiteflag ordering.
	whiteFlagIndex uint32

	conflict Conflict

	// youngestConeRootIndex is the highest referenced index of the past cone of this block.
	youngestConeRootIndex iotago.MilestoneIndex

	// oldestConeRootIndex is the lowest referenced index of the past cone of this block.
	oldestConeRootIndex iotago.MilestoneIndex

	// coneRootCalculationIndex is the confirmed milestone index ycri and ocri were calculated at.
	coneRootCalculationIndex iotago.MilestoneIndex

	// parents are the parents of the block.
	parents iotago.BlockIDs
}

func NewBlockMetadata(blockID iotago.BlockID, parents iotago.BlockIDs) *BlockMetadata {
	return &BlockMetadata{
		blockID: blockID,
		parents: parents,
	}
}

func (m *BlockMetadata) BlockID() iotago.BlockID {
	return m.blockID
}

func (m *BlockMetadata) Parents() iotago.BlockIDs {
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

func (m *BlockMetadata) ReferencedWithIndex() (bool, iotago.MilestoneIndex) {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(BlockMetadataReferenced), m.referencedIndex
}

func (m *BlockMetadata) ReferencedWithIndexAndWhiteFlagIndex() (bool, iotago.MilestoneIndex, uint32) {
	m.RLock()
	defer m.RUnlock()

	return m.metadata.HasBit(BlockMetadataReferenced), m.referencedIndex, m.whiteFlagIndex
}

func (m *BlockMetadata) SetReferenced(referenced bool, referencedIndex iotago.MilestoneIndex, whiteFlagIndex uint32) {
	m.Lock()
	defer m.Unlock()

	if referenced != m.metadata.HasBit(BlockMetadataReferenced) {
		if referenced {
			m.referencedIndex = referencedIndex
			m.whiteFlagIndex = whiteFlagIndex
		} else {
			m.referencedIndex = 0
			m.whiteFlagIndex = 0
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

func (m *BlockMetadata) SetConeRootIndexes(ycri iotago.MilestoneIndex, ocri iotago.MilestoneIndex, ci iotago.MilestoneIndex) {
	m.Lock()
	defer m.Unlock()

	m.youngestConeRootIndex = ycri
	m.oldestConeRootIndex = ocri
	m.coneRootCalculationIndex = ci
	m.SetModified(true)
}

func (m *BlockMetadata) ConeRootIndexes() (ycri iotago.MilestoneIndex, ocri iotago.MilestoneIndex, ci iotago.MilestoneIndex) {
	m.RLock()
	defer m.RUnlock()

	return m.youngestConeRootIndex, m.oldestConeRootIndex, m.coneRootCalculationIndex
}

func (m *BlockMetadata) Metadata() byte {
	m.RLock()
	defer m.RUnlock()

	return byte(m.metadata)
}

// ObjectStorage interface.

func (m *BlockMetadata) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("BlockMetadata should never be updated: %v", m.blockID.ToHex()))
}

func (m *BlockMetadata) ObjectStorageKey() []byte {
	return m.blockID[:]
}

func (m *BlockMetadata) ObjectStorageValue() (data []byte) {
	m.Lock()
	defer m.Unlock()

	/*
		1 byte  metadata bitmask
		4 bytes iotago.MilestoneIndex referencedIndex
		4 bytes uint32 whiteFlagIndex
		1 byte  uint8 conflict
		4 bytes iotago.MilestoneIndex youngestConeRootIndex
		4 bytes iotago.MilestoneIndex oldestConeRootIndex
		4 bytes iotago.MilestoneIndex coneRootCalculationIndex
		1 byte  parents count
		parents count * 32 bytes parent id
	*/

	marshalUtil := marshalutil.New(23 + len(m.parents)*iotago.BlockIDLength)

	marshalUtil.WriteByte(byte(m.metadata))
	marshalUtil.WriteUint32(m.referencedIndex)
	marshalUtil.WriteUint32(m.whiteFlagIndex)
	marshalUtil.WriteByte(byte(m.conflict))
	marshalUtil.WriteUint32(m.youngestConeRootIndex)
	marshalUtil.WriteUint32(m.oldestConeRootIndex)
	marshalUtil.WriteUint32(m.coneRootCalculationIndex)
	marshalUtil.WriteByte(byte(len(m.parents)))
	for _, parent := range m.parents {
		marshalUtil.WriteBytes(parent[:])
	}

	return marshalUtil.Bytes()
}

func MetadataFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {

	/*
		1 byte  metadata bitmask
		4 bytes iotago.MilestoneIndex referencedIndex
		4 bytes uint32 whiteFlagIndex
		1 byte  uint8 conflict
		4 bytes iotago.MilestoneIndex youngestConeRootIndex
		4 bytes iotago.MilestoneIndex oldestConeRootIndex
		4 bytes iotago.MilestoneIndex coneRootCalculationIndex
		1 byte  parents count
		parents count * 32 bytes parent id
	*/

	m := &BlockMetadata{}
	copy(m.blockID[:], key[:iotago.BlockIDLength])

	marshalUtil := marshalutil.New(data)

	metadataByte, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, err
	}

	m.referencedIndex, err = marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	m.whiteFlagIndex, err = marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	conflict, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, err
	}

	m.youngestConeRootIndex, err = marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	m.oldestConeRootIndex, err = marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	m.coneRootCalculationIndex, err = marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	m.metadata = bitmask.BitMask(metadataByte)
	m.conflict = Conflict(conflict)

	parentsCount, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, err
	}

	m.parents = make(iotago.BlockIDs, parentsCount)
	for i := 0; i < int(parentsCount); i++ {
		parentBytes, err := marshalUtil.ReadBytes(iotago.BlockIDLength)
		if err != nil {
			return nil, err
		}
		copy(m.parents[i][:], parentBytes)
	}

	return m, nil
}
