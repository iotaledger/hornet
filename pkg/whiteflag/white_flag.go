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
	DefaultWhiteFlagTraversalCondition = func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
		defer cachedMsgMeta.Release(true) // meta -1

		// only traverse and process the message if it was not referenced yet
		return !cachedMsgMeta.Metadata().IsReferenced(), nil
	}
)

// Confirmation represents a confirmation done via a milestone under the "white-flag" approach.
type Confirmation struct {
	// The index of the milestone that got confirmed.
	MilestoneIndex milestone.Index
	// The message ID of the milestone that got confirmed.
	MilestoneMessageID hornet.MessageID
	// The ledger mutations and referenced messages of this milestone.
	Mutations *WhiteFlagMutations
}

type MessageWithConflict struct {
	MessageID hornet.MessageID
	Conflict  storage.Conflict
}

// WhiteFlagMutations contains the ledger mutations and referenced messages applied to a cone under the "white-flag" approach.
type WhiteFlagMutations struct {
	// The messages which mutate the ledger in the order in which they were applied.
	MessagesIncludedWithTransactions hornet.MessageIDs
	// The messages which were excluded as they were conflicting with the mutations.
	MessagesExcludedWithConflictingTransactions []MessageWithConflict
	// The messages which were excluded because they did not include a value transaction.
	MessagesExcludedWithoutTransactions hornet.MessageIDs
	// The messages which were referenced by the milestone (should be the sum of MessagesIncludedWithTransactions + MessagesExcludedWithConflictingTransactions + MessagesExcludedWithoutTransactions).
	MessagesReferenced hornet.MessageIDs
	// Contains the newly created Unspent Outputs by the given confirmation.
	NewOutputs map[string]*utxo.Output
	// Contains the Spent Outputs for the given confirmation.
	NewSpents map[string]*utxo.Spent
	// The merkle tree root hash of all messages.
	MerkleTreeHash [iotago.MilestoneInclusionMerkleProofLength]byte
}

// ComputeWhiteFlagMutations computes the ledger changes in accordance to the white-flag rules for the cone referenced by the parents.
// Via a post-order depth-first search the approved messages of the given cone are traversed and
// in their corresponding order applied/mutated against the previous ledger state, respectively previous applied mutations.
// Messages within the approving cone must be valid. Messages causing conflicts are ignored but do not create an error.
// It also computes the merkle tree root hash consisting out of the IDs of the messages which are part of the set
// which mutated the ledger state when applying the white-flag approach.
// The ledger state must be write locked while this function is getting called in order to ensure consistency.
func ComputeWhiteFlagMutations(ctx context.Context,
	utxoManager *utxo.Manager,
	parentsTraverser *dag.ParentsTraverser,
	cachedMessageFunc storage.CachedMessageFunc,
	networkId uint64,
	msIndex milestone.Index,
	msTimestamp uint64,
	parents hornet.MessageIDs,
	traversalCondition dag.Predicate) (*WhiteFlagMutations, error) {

	wfConf := &WhiteFlagMutations{
		MessagesIncludedWithTransactions:            make(hornet.MessageIDs, 0),
		MessagesExcludedWithConflictingTransactions: make([]MessageWithConflict, 0),
		MessagesExcludedWithoutTransactions:         make(hornet.MessageIDs, 0),
		MessagesReferenced:                          make(hornet.MessageIDs, 0),
		NewOutputs:                                  make(map[string]*utxo.Output),
		NewSpents:                                   make(map[string]*utxo.Spent),
	}

	semValCtx := &iotago.SemanticValidationContext{
		ExtParas: &iotago.ExternalUnlockParameters{
			ConfMsIndex: uint32(msIndex),
			ConfUnix:    uint32(msTimestamp),
		},
	}

	// consumer
	consumer := func(cachedMsgMeta *storage.CachedMetadata) error { // meta +1
		defer cachedMsgMeta.Release(true) // meta -1

		messageID := cachedMsgMeta.Metadata().MessageID()

		// load up message
		cachedMsg, err := cachedMessageFunc(messageID) // message +1
		if err != nil {
			return err
		}
		if cachedMsg == nil {
			return fmt.Errorf("%w: message %s of candidate msg %s doesn't exist", common.ErrMessageNotFound, messageID.ToHex(), messageID.ToHex())
		}
		defer cachedMsg.Release(true) // message -1

		message := cachedMsg.Message()

		// exclude message without transactions
		if !message.IsTransaction() {
			wfConf.MessagesReferenced = append(wfConf.MessagesReferenced, messageID)
			wfConf.MessagesExcludedWithoutTransactions = append(wfConf.MessagesExcludedWithoutTransactions, messageID)
			return nil
		}

		var conflict = storage.ConflictNone

		transaction := message.Transaction()
		transactionID, err := transaction.ID()
		if err != nil {
			return err
		}

		if transaction.Essence.NetworkID != networkId {
			conflict = storage.ConflictInvalidNetworkID
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
					if errors.Is(err, iotago.ErrMissingUTXO) {
						conflict = storage.ConflictInputUTXONotFound
					} else if errors.Is(err, iotago.ErrInputOutputSumMismatch) {
						conflict = storage.ConflictInputOutputSumMismatch
					} else if errors.Is(err, iotago.ErrEd25519SignatureInvalid) || errors.Is(err, iotago.ErrEd25519PubKeyAndAddrMismatch) {
						conflict = storage.ConflictInvalidSignature
					} else {
						conflict = storage.ConflictSemanticValidationFailed
					}
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
				output, err := utxo.NewOutput(messageID, msIndex, msTimestamp, transaction, uint16(i))
				if err != nil {
					return err
				}
				generatedOutputs = append(generatedOutputs, output)
			}
		}

		wfConf.MessagesReferenced = append(wfConf.MessagesReferenced, messageID)

		if conflict != storage.ConflictNone {
			wfConf.MessagesExcludedWithConflictingTransactions = append(wfConf.MessagesExcludedWithConflictingTransactions, MessageWithConflict{
				MessageID: messageID,
				Conflict:  conflict,
			})
			return nil
		}

		// mark the given message to be part of milestone ledger by changing message inclusion set
		wfConf.MessagesIncludedWithTransactions = append(wfConf.MessagesIncludedWithTransactions, messageID)

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
	// If the popped message was used to mutate the Confirmation struct, it will also be appended to Confirmation.MessagesIncludedWithTransactions.
	if err := parentsTraverser.Traverse(
		ctx,
		parents,
		traversalCondition,
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

	// compute merkle tree root hash
	marshalers := make([]encoding.BinaryMarshaler, len(wfConf.MessagesIncludedWithTransactions))
	for i := range wfConf.MessagesIncludedWithTransactions {
		marshalers[i] = wfConf.MessagesIncludedWithTransactions[i]
	}
	merkleTreeHash, err := NewHasher(crypto.BLAKE2b_256).Hash(marshalers)
	if err != nil {
		return nil, fmt.Errorf("failed to compute Merkle tree hash: %w", err)
	}
	copy(wfConf.MerkleTreeHash[:], merkleTreeHash)

	if len(wfConf.MessagesIncludedWithTransactions) != (len(wfConf.MessagesReferenced) - len(wfConf.MessagesExcludedWithConflictingTransactions) - len(wfConf.MessagesExcludedWithoutTransactions)) {
		return nil, ErrIncludedMessagesSumDoesntMatch
	}

	return wfConf, nil
}
