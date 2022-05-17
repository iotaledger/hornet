package whiteflag

import (
	"context"
	"crypto"
	"encoding"
	"fmt"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go/v3"

	// import implementation
	_ "golang.org/x/crypto/blake2b"
)

var (
	// ErrIncludedMessagesSumDoesntMatch is returned when the sum of the included messages a milestone approves does not match the referenced messages minus the excluded messages.
	ErrIncludedMessagesSumDoesntMatch = errors.New("the sum of the included messages doesn't match the referenced messages minus the excluded messages")

	// traversal stops if no more messages pass the given condition
	// Caution: condition func is not in DFS order
	DefaultWhiteFlagTraversalCondition = func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
		defer cachedBlockMeta.Release(true) // meta -1

		// only traverse and process the message if it was not referenced yet
		return !cachedBlockMeta.Metadata().IsReferenced(), nil
	}

	emptyMilestoneID = iotago.MilestoneID{}
)

// Confirmation represents a confirmation done via a milestone under the "white-flag" approach.
type Confirmation struct {
	// The index of the milestone that got confirmed.
	MilestoneIndex milestone.Index
	// The milestone ID of the milestone that got confirmed.
	MilestoneID iotago.MilestoneID
	// The parents of the milestone that got confirmed.
	MilestoneParents hornet.BlockIDs
	// The ledger mutations and referenced messages of this milestone.
	Mutations *WhiteFlagMutations
}

type MessageWithConflict struct {
	MessageID hornet.BlockID
	Conflict  storage.Conflict
}

// WhiteFlagMutations contains the ledger mutations and referenced messages applied to a cone under the "white-flag" approach.
type WhiteFlagMutations struct {
	// The messages which mutate the ledger in the order in which they were applied.
	MessagesIncludedWithTransactions hornet.BlockIDs
	// The messages which were excluded as they were conflicting with the mutations.
	MessagesExcludedWithConflictingTransactions []MessageWithConflict
	// The messages which were excluded because they did not include a value transaction.
	MessagesExcludedWithoutTransactions hornet.BlockIDs
	// The messages which were referenced by the milestone (should be the sum of MessagesIncludedWithTransactions + MessagesExcludedWithConflictingTransactions + MessagesExcludedWithoutTransactions).
	BlocksReferenced hornet.BlockIDs
	// Contains the newly created Unspent Outputs by the given confirmation.
	NewOutputs map[string]*utxo.Output
	// Contains the Spent Outputs for the given confirmation.
	NewSpents map[string]*utxo.Spent
	// The merkle tree root hash of all referenced messages in the past cone.
	InclusionMerkleRoot [iotago.MilestoneMerkleProofLength]byte
	// The merkle tree root hash of all included transaction messages.
	AppliedMerkleRoot [iotago.MilestoneMerkleProofLength]byte
}

// ComputeWhiteFlagMutations computes the ledger changes in accordance to the white-flag rules for the cone referenced by the parents.
// Via a post-order depth-first search the approved messages of the given cone are traversed and
// in their corresponding order applied/mutated against the previous ledger state, respectively previous applied mutations.
// Blocks within the approving cone must be valid. Blocks causing conflicts are ignored but do not create an error.
// It also computes the merkle tree root hash consisting out of the IDs of the messages which are part of the set
// which mutated the ledger state when applying the white-flag approach.
// The ledger state must be write locked while this function is getting called in order to ensure consistency.
func ComputeWhiteFlagMutations(ctx context.Context,
	utxoManager *utxo.Manager,
	parentsTraverser *dag.ParentsTraverser,
	cachedMessageFunc storage.CachedBlockFunc,
	msIndex milestone.Index,
	msTimestamp uint32,
	parents hornet.BlockIDs,
	previousMilestoneID iotago.MilestoneID,
	traversalCondition dag.Predicate) (*WhiteFlagMutations, error) {

	wfConf := &WhiteFlagMutations{
		MessagesIncludedWithTransactions:            make(hornet.BlockIDs, 0),
		MessagesExcludedWithConflictingTransactions: make([]MessageWithConflict, 0),
		MessagesExcludedWithoutTransactions:         make(hornet.BlockIDs, 0),
		BlocksReferenced:                            make(hornet.BlockIDs, 0),
		NewOutputs:                                  make(map[string]*utxo.Output),
		NewSpents:                                   make(map[string]*utxo.Spent),
	}

	semValCtx := &iotago.SemanticValidationContext{
		ExtParas: &iotago.ExternalUnlockParameters{
			ConfMsIndex: uint32(msIndex),
			ConfUnix:    msTimestamp,
		},
	}

	isFirstMilestone := msIndex == 1
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
			msgMilestone, err := cachedMessageFunc(cachedBlockMeta.Metadata().BlockID()) // block +1
			if err != nil {
				return false, err
			}
			if msgMilestone == nil {
				return false, fmt.Errorf("ComputeWhiteFlagMutations: message not found for milestone message ID: %v", cachedBlockMeta.Metadata().BlockID().ToHex())
			}
			defer msgMilestone.Release(true) // block -1

			milestonePayload := msgMilestone.Block().Milestone()
			if milestonePayload == nil {
				return false, fmt.Errorf("ComputeWhiteFlagMutations: message for milestone message ID does not contain a milestone payload: %v", cachedBlockMeta.Metadata().BlockID().ToHex())
			}

			msIDPtr, err := milestonePayload.ID()
			if err != nil {
				return false, err
			}

			// Compare this milestones ID with the previousMilestoneID
			seenPreviousMilestoneID = *msIDPtr == previousMilestoneID
			if seenPreviousMilestoneID {
				// Check that the milestone timestamp has increased
				if milestonePayload.Timestamp >= msTimestamp {
					return false, fmt.Errorf("ComputeWhiteFlagMutations: milestone timestamp is smaller or equal to previous milestone timestamp (old: %d, new: %d): %v", milestonePayload.Timestamp, msTimestamp, cachedBlockMeta.Metadata().BlockID().ToHex())
				}
				if (milestonePayload.Index + 1) != uint32(msIndex) {
					return false, fmt.Errorf("ComputeWhiteFlagMutations: milestone index did not increase by one compared to previous milestone index (old: %d, new: %d): %v", milestonePayload.Index, msIndex, cachedBlockMeta.Metadata().BlockID().ToHex())
				}
			}
		}
		return traversalCondition(cachedBlockMeta) // meta pass +1
	}

	// consumer
	consumer := func(cachedBlockMeta *storage.CachedMetadata) error { // meta +1
		defer cachedBlockMeta.Release(true) // meta -1

		blockID := cachedBlockMeta.Metadata().BlockID()

		// load up message
		cachedBlock, err := cachedMessageFunc(blockID) // block +1
		if err != nil {
			return err
		}
		if cachedBlock == nil {
			return fmt.Errorf("%w: message %s of candidate msg %s doesn't exist", common.ErrBlockNotFound, blockID.ToHex(), blockID.ToHex())
		}
		defer cachedBlock.Release(true) // block -1

		message := cachedBlock.Block()

		// exclude message without transactions
		if !message.IsTransaction() {
			wfConf.BlocksReferenced = append(wfConf.BlocksReferenced, blockID)
			wfConf.MessagesExcludedWithoutTransactions = append(wfConf.MessagesExcludedWithoutTransactions, blockID)
			return nil
		}

		var conflict = storage.ConflictNone

		transaction := message.Transaction()
		transactionID, err := transaction.ID()
		if err != nil {
			return err
		}

		// go through all the inputs and validate that they are still unspent, in the ledger or were created during confirmation
		inputOutputs := utxo.Outputs{}
		if conflict == storage.ConflictNone {
			inputs := message.TransactionEssenceUTXOInputs()
			for _, input := range inputs {

				// check if this input was already spent during the confirmation
				_, hasSpent := wfConf.NewSpents[string(input[:])]
				if hasSpent {
					// UTXO already spent, so mark as conflict
					conflict = storage.ConflictInputUTXOAlreadySpentInThisMilestone
					break
				}

				// check if this input was newly created during the confirmation
				output, hasOutput := wfConf.NewOutputs[string(input[:])]
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

			transactionEssence := message.TransactionEssence()
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

		wfConf.BlocksReferenced = append(wfConf.BlocksReferenced, blockID)

		if conflict != storage.ConflictNone {
			wfConf.MessagesExcludedWithConflictingTransactions = append(wfConf.MessagesExcludedWithConflictingTransactions, MessageWithConflict{
				MessageID: blockID,
				Conflict:  conflict,
			})
			return nil
		}

		// mark the given message to be part of milestone ledger by changing message inclusion set
		wfConf.MessagesIncludedWithTransactions = append(wfConf.MessagesIncludedWithTransactions, blockID)

		newSpents := make(utxo.Spents, len(inputOutputs))

		// save the inputs as spent
		for i, input := range inputOutputs {
			spent := utxo.NewSpent(input, transactionID, msIndex, msTimestamp)
			wfConf.NewSpents[string(input.OutputID()[:])] = spent
			newSpents[i] = spent
		}

		// add new outputs
		for _, output := range generatedOutputs {
			wfConf.NewOutputs[string(output.OutputID()[:])] = output
		}

		return nil
	}

	// This function does the DFS and computes the mutations a white-flag confirmation would create.
	// If the parents are SEPs, are already processed or already referenced,
	// then the mutations from the messages retrieved from the stack are accumulated to the given Confirmation struct's mutations.
	// If the popped message was used to mutate the Confirmation struct, it will also be appended to Confirmation.BlocksIncludedWithTransactions.
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
	confirmedMarshalers := make([]encoding.BinaryMarshaler, len(wfConf.BlocksReferenced))
	for i := range wfConf.BlocksReferenced {
		confirmedMarshalers[i] = wfConf.BlocksReferenced[i]
	}
	confirmedMerkleHash, err := NewHasher(crypto.BLAKE2b_256).Hash(confirmedMarshalers)
	if err != nil {
		return nil, fmt.Errorf("failed to compute confirmed merkle tree root: %w", err)
	}
	copy(wfConf.InclusionMerkleRoot[:], confirmedMerkleHash)

	// compute inclusion merkle tree root hash
	appliedMarshalers := make([]encoding.BinaryMarshaler, len(wfConf.MessagesIncludedWithTransactions))
	for i := range wfConf.MessagesIncludedWithTransactions {
		appliedMarshalers[i] = wfConf.MessagesIncludedWithTransactions[i]
	}
	appliedMerkleHash, err := NewHasher(crypto.BLAKE2b_256).Hash(appliedMarshalers)
	if err != nil {
		return nil, fmt.Errorf("failed to compute applied merkle tree root: %w", err)
	}
	copy(wfConf.AppliedMerkleRoot[:], appliedMerkleHash)

	if len(wfConf.MessagesIncludedWithTransactions) != (len(wfConf.BlocksReferenced) - len(wfConf.MessagesExcludedWithConflictingTransactions) - len(wfConf.MessagesExcludedWithoutTransactions)) {
		return nil, ErrIncludedMessagesSumDoesntMatch
	}

	return wfConf, nil
}
