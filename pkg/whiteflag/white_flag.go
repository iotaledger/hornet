package whiteflag

import (
	"context"
	"crypto"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/merklehasher"

	// import implementation.
	_ "golang.org/x/crypto/blake2b"
)

var (
	// ErrIncludedBlocksSumDoesntMatch is returned when the sum of the included blocks a milestone approves does not match the referenced blocks minus the excluded blocks.
	ErrIncludedBlocksSumDoesntMatch = errors.New("the sum of the included blocks doesn't match the referenced blocks minus the excluded blocks")

	// DefaultWhiteFlagTraversalCondition is the default traversal condition used in WhiteFlag.
	// The traversal stops if no more blocks pass the given condition
	// Caution: condition func is not in DFS order.
	DefaultWhiteFlagTraversalCondition = func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
		defer cachedBlockMeta.Release(true) // meta -1

		// only traverse and process the block if it was not referenced yet
		return !cachedBlockMeta.Metadata().IsReferenced(), nil
	}

	emptyMilestoneID = iotago.MilestoneID{}
)

// Confirmation represents a confirmation done via a milestone under the "white-flag" approach.
type Confirmation struct {
	// The index of the milestone that got confirmed.
	MilestoneIndex iotago.MilestoneIndex
	// The milestone ID of the milestone that got confirmed.
	MilestoneID iotago.MilestoneID
	// The parents of the milestone that got confirmed.
	MilestoneParents iotago.BlockIDs
	// The ledger mutations and referenced blocks of this milestone.
	Mutations *WhiteFlagMutations
}

type ReferencedBlock struct {
	BlockID       iotago.BlockID
	IsTransaction bool
	Conflict      storage.Conflict
}

type ReferencedBlocks []ReferencedBlock

func (b ReferencedBlocks) BlockIDs() iotago.BlockIDs {
	var blockIDs iotago.BlockIDs
	for _, rb := range b {
		blockIDs = append(blockIDs, rb.BlockID)
	}

	return blockIDs
}

func (b ReferencedBlocks) IncludedTransactionBlockIDs() iotago.BlockIDs {
	var blockIDs iotago.BlockIDs
	for _, rb := range b {
		if rb.IsTransaction && rb.Conflict == storage.ConflictNone {
			blockIDs = append(blockIDs, rb.BlockID)
		}
	}

	return blockIDs
}

func (b ReferencedBlocks) ConflictingTransactionBlockIDs() iotago.BlockIDs {
	var blockIDs iotago.BlockIDs
	for _, rb := range b {
		if rb.IsTransaction && rb.Conflict != storage.ConflictNone {
			blockIDs = append(blockIDs, rb.BlockID)
		}
	}

	return blockIDs
}

func (b ReferencedBlocks) NonTransactionBlockIDs() iotago.BlockIDs {
	var blockIDs iotago.BlockIDs
	for _, rb := range b {
		if !rb.IsTransaction {
			blockIDs = append(blockIDs, rb.BlockID)
		}
	}

	return blockIDs
}

// WhiteFlagMutations contains the ledger mutations and referenced blocks applied to a cone under the "white-flag" approach.
//
//nolint:revive // better be explicit here
type WhiteFlagMutations struct {
	// The blocks which were referenced by the milestone
	ReferencedBlocks ReferencedBlocks
	// Contains the newly created Unspent Outputs by the given confirmation.
	NewOutputs map[iotago.OutputID]*utxo.Output
	// Contains the Spent Outputs for the given confirmation.
	NewSpents map[iotago.OutputID]*utxo.Spent
	// The merkle tree root hash of all referenced blocks in the past cone.
	InclusionMerkleRoot [iotago.MilestoneMerkleProofLength]byte
	// The merkle tree root hash of all included transaction blocks.
	AppliedMerkleRoot [iotago.MilestoneMerkleProofLength]byte
}

// ComputeWhiteFlagMutations computes the ledger changes in accordance to the white-flag rules for the cone referenced by the parents.
// Via a post-order depth-first search the approved blocks of the given cone are traversed and
// in their corresponding order applied/mutated against the previous ledger state, respectively previous applied mutations.
// Blocks within the approving cone must be valid. Blocks causing conflicts are ignored but do not create an error.
// It also computes the merkle tree root hash consisting out of the IDs of the blocks which are part of the set
// which mutated the ledger state when applying the white-flag approach.
// The ledger state must be write locked while this function is getting called in order to ensure consistency.
func ComputeWhiteFlagMutations(ctx context.Context,
	utxoManager *utxo.Manager,
	parentsTraverser *dag.ParentsTraverser,
	cachedBlockFunc storage.CachedBlockFunc,
	msIndex iotago.MilestoneIndex,
	msTimestamp uint32,
	parents iotago.BlockIDs,
	previousMilestoneID iotago.MilestoneID,
	genesisMilestoneIndex iotago.MilestoneIndex,
	traversalCondition dag.Predicate) (*WhiteFlagMutations, error) {

	wfConf := &WhiteFlagMutations{
		ReferencedBlocks: make(ReferencedBlocks, 0),
		NewOutputs:       make(map[iotago.OutputID]*utxo.Output),
		NewSpents:        make(map[iotago.OutputID]*utxo.Spent),
	}

	semValCtx := &iotago.SemanticValidationContext{
		ExtParas: &iotago.ExternalUnlockParameters{
			ConfUnix: msTimestamp,
		},
	}

	isFirstMilestone := msIndex == genesisMilestoneIndex+1
	if isFirstMilestone && previousMilestoneID != emptyMilestoneID {
		return nil, fmt.Errorf("invalid previousMilestoneID for initial milestone: %s", iotago.EncodeHex(previousMilestoneID[:]))
	}
	if !isFirstMilestone && previousMilestoneID == emptyMilestoneID {
		return nil, fmt.Errorf("missing previousMilestoneID for milestone: %d", msIndex)
	}

	// Use a custom traversal condition that tracks if the previousMilestoneID was seen in the past cone
	// Skip this check for the first milestone
	seenPreviousMilestoneID := isFirstMilestone
	internalTraversalCondition := func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
		if !seenPreviousMilestoneID && cachedBlockMeta.Metadata().IsMilestone() {
			blockID := cachedBlockMeta.Metadata().BlockID()
			blockMilestone, err := cachedBlockFunc(blockID) // block +1
			if err != nil {
				return false, err
			}
			if blockMilestone == nil {
				return false, fmt.Errorf("ComputeWhiteFlagMutations: block not found for milestone block ID: %v", blockID.ToHex())
			}
			defer blockMilestone.Release(true) // block -1

			milestonePayload := blockMilestone.Block().Milestone()
			if milestonePayload == nil {
				return false, fmt.Errorf("ComputeWhiteFlagMutations: block for milestone block ID does not contain a milestone payload: %v", blockID.ToHex())
			}

			msID, err := milestonePayload.ID()
			if err != nil {
				return false, err
			}

			// Compare this milestones ID with the previousMilestoneID
			seenPreviousMilestoneID = msID == previousMilestoneID
			if seenPreviousMilestoneID {
				// Check that the milestone timestamp has increased
				if milestonePayload.Timestamp >= msTimestamp {
					return false, fmt.Errorf("ComputeWhiteFlagMutations: milestone timestamp is smaller or equal to previous milestone timestamp (old: %d, new: %d): %v", milestonePayload.Timestamp, msTimestamp, blockID.ToHex())
				}
				if (milestonePayload.Index + 1) != msIndex {
					return false, fmt.Errorf("ComputeWhiteFlagMutations: milestone index did not increase by one compared to previous milestone index (old: %d, new: %d): %v", milestonePayload.Index, msIndex, blockID.ToHex())
				}
			}
		}

		return traversalCondition(cachedBlockMeta) // meta pass +1
	}

	// consumer
	consumer := func(cachedBlockMeta *storage.CachedMetadata) error { // meta +1
		defer cachedBlockMeta.Release(true) // meta -1

		blockID := cachedBlockMeta.Metadata().BlockID()

		// load up block
		cachedBlock, err := cachedBlockFunc(blockID) // block +1
		if err != nil {
			return err
		}
		if cachedBlock == nil {
			return fmt.Errorf("%w: block of candidate block %s not found", common.ErrBlockNotFound, blockID.ToHex())
		}
		defer cachedBlock.Release(true) // block -1

		block := cachedBlock.Block()

		// exclude block without transactions
		if !block.IsTransaction() {
			wfConf.ReferencedBlocks = append(wfConf.ReferencedBlocks, ReferencedBlock{
				BlockID:       blockID,
				IsTransaction: false,
				Conflict:      storage.ConflictNone,
			})

			return nil
		}

		var conflict = storage.ConflictNone

		transaction := block.Transaction()
		transactionID, err := transaction.ID()
		if err != nil {
			return err
		}

		// go through all the inputs and validate that they are still unspent, in the ledger or were created during confirmation
		inputOutputs := utxo.Outputs{}
		if conflict == storage.ConflictNone {
			inputs := block.TransactionEssenceUTXOInputs()
			for _, input := range inputs {

				// check if this input was already spent during the confirmation
				_, hasSpent := wfConf.NewSpents[input]
				if hasSpent {
					// UTXO already spent, so mark as conflict
					conflict = storage.ConflictInputUTXOAlreadySpentInThisMilestone

					break
				}

				// check if this input was newly created during the confirmation
				output, hasOutput := wfConf.NewOutputs[input]
				if hasOutput {
					// UTXO is in the current ledger mutation, so use it
					inputOutputs = append(inputOutputs, output)

					continue
				}

				// check current ledger for this input
				output, err = utxoManager.ReadOutputByOutputIDWithoutLocking(input)
				if err != nil {
					if errors.Is(err, kvstore.ErrKeyNotFound) {
						// input not found, so mark as invalid tx
						conflict = storage.ConflictInputUTXONotFound

						break
					}

					return err
				}

				// check if this output is unspent
				unspent, err := utxoManager.IsOutputUnspentWithoutLocking(output)
				if err != nil {
					return err
				}

				if !unspent {
					// output is already spent, so mark as conflict
					conflict = storage.ConflictInputUTXOAlreadySpent

					break
				}

				inputOutputs = append(inputOutputs, output)
			}

			if conflict == storage.ConflictNone {
				// Verify that all outputs consume all inputs and have valid signatures. Also verify that the amounts match.
				if err := transaction.SemanticallyValidate(semValCtx, inputOutputs.ToOutputSet()); err != nil {
					conflict = storage.ConflictFromSemanticValidationError(err)
				}
			}
		}

		// go through all deposits and generate unspent outputs
		generatedOutputs := utxo.Outputs{}
		if conflict == storage.ConflictNone {

			transactionEssence := block.TransactionEssence()
			if transactionEssence == nil {
				return fmt.Errorf("no transaction transactionEssence found")
			}

			for i := 0; i < len(transactionEssence.Outputs); i++ {
				output, err := utxo.NewOutput(blockID, msIndex, msTimestamp, transaction, uint16(i))
				if err != nil {
					return err
				}
				generatedOutputs = append(generatedOutputs, output)
			}
		}

		wfConf.ReferencedBlocks = append(wfConf.ReferencedBlocks, ReferencedBlock{
			BlockID:       blockID,
			IsTransaction: true,
			Conflict:      conflict,
		})

		if conflict != storage.ConflictNone {
			return nil
		}

		newSpents := make(utxo.Spents, len(inputOutputs))

		// save the inputs as spent
		for i, input := range inputOutputs {
			spent := utxo.NewSpent(input, transactionID, msIndex, msTimestamp)
			wfConf.NewSpents[input.OutputID()] = spent
			newSpents[i] = spent
		}

		// add new outputs
		for _, output := range generatedOutputs {
			wfConf.NewOutputs[output.OutputID()] = output
		}

		return nil
	}

	// This function does the DFS and computes the mutations a white-flag confirmation would create.
	// If the parents are SEPs, are already processed or already referenced,
	// then the mutations from the blocks retrieved from the stack are accumulated to the given Confirmation struct's mutations.
	// If the popped block was used to mutate the Confirmation struct, it will also be appended to Confirmation.BlocksIncludedWithTransactions.
	if err := parentsTraverser.Traverse(
		ctx,
		parents,
		internalTraversalCondition,
		consumer,
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false); err != nil {
		return nil, err
	}

	if !seenPreviousMilestoneID {
		return nil, fmt.Errorf("previousMilestoneID %s not referenced in past cone", iotago.EncodeHex(previousMilestoneID[:]))
	}

	// compute past cone merkle tree root hash
	confirmedMerkleHash := merklehasher.NewHasher(crypto.BLAKE2b_256).HashBlockIDs(wfConf.ReferencedBlocks.BlockIDs())
	copy(wfConf.InclusionMerkleRoot[:], confirmedMerkleHash)

	// compute inclusion merkle tree root hash
	appliedMerkleHash := merklehasher.NewHasher(crypto.BLAKE2b_256).HashBlockIDs(wfConf.ReferencedBlocks.IncludedTransactionBlockIDs())
	copy(wfConf.AppliedMerkleRoot[:], appliedMerkleHash)

	if len(wfConf.ReferencedBlocks.IncludedTransactionBlockIDs()) != (len(wfConf.ReferencedBlocks) - len(wfConf.ReferencedBlocks.ConflictingTransactionBlockIDs()) - len(wfConf.ReferencedBlocks.NonTransactionBlockIDs())) {
		return nil, ErrIncludedBlocksSumDoesntMatch
	}

	return wfConf, nil
}
