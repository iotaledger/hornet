package whiteflag

import (
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
	iotago "github.com/iotaledger/iota.go/v2"

	// import implementation
	_ "golang.org/x/crypto/blake2b"
)

var (
	// ErrIncludedMessagesSumDoesntMatch is returned when the sum of the included messages a milestone approves does not match the referenced messages minus the excluded messages.
	ErrIncludedMessagesSumDoesntMatch = errors.New("the sum of the included messages doesn't match the referenced messages minus the excluded messages")
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
	// Contains the calculated Dust allowance diff.
	dustAllowanceDiff *utxo.BalanceDiff
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
// metadataMemcache has to be cleaned up outside.
func ComputeWhiteFlagMutations(s *storage.Storage, msIndex milestone.Index, metadataMemcache *storage.MetadataMemcache, messagesMemcache *storage.MessagesMemcache, parents hornet.MessageIDs) (*WhiteFlagMutations, error) {
	wfConf := &WhiteFlagMutations{
		MessagesIncludedWithTransactions:            make(hornet.MessageIDs, 0),
		MessagesExcludedWithConflictingTransactions: make([]MessageWithConflict, 0),
		MessagesExcludedWithoutTransactions:         make(hornet.MessageIDs, 0),
		MessagesReferenced:                          make(hornet.MessageIDs, 0),
		NewOutputs:                                  make(map[string]*utxo.Output),
		NewSpents:                                   make(map[string]*utxo.Spent),
		dustAllowanceDiff:                           utxo.NewBalanceDiff(),
	}

	// traversal stops if no more messages pass the given condition
	// Caution: condition func is not in DFS order
	condition := func(cachedMetadata *storage.CachedMetadata) (bool, error) { // meta +1
		defer cachedMetadata.Release(true) // meta -1

		// only traverse and process the message if it was not referenced yet
		return !cachedMetadata.Metadata().IsReferenced(), nil
	}

	// consumer
	consumer := func(cachedMetadata *storage.CachedMetadata) error { // meta +1
		defer cachedMetadata.Release(true) // meta -1

		// load up message
		cachedMessage := messagesMemcache.CachedMessageOrNil(cachedMetadata.Metadata().MessageID())
		if cachedMessage == nil {
			return fmt.Errorf("%w: message %s of candidate msg %s doesn't exist", common.ErrMessageNotFound, cachedMetadata.Metadata().MessageID().ToHex(), cachedMetadata.Metadata().MessageID().ToHex())
		}

		message := cachedMessage.Message()

		// exclude message without transactions
		if !message.IsTransaction() {
			wfConf.MessagesReferenced = append(wfConf.MessagesReferenced, message.MessageID())
			wfConf.MessagesExcludedWithoutTransactions = append(wfConf.MessagesExcludedWithoutTransactions, message.MessageID())
			return nil
		}

		var conflict = storage.ConflictNone

		transaction := message.Transaction()
		transactionID, err := transaction.ID()
		if err != nil {
			return err
		}

		// Verify transaction syntax
		if err := transaction.SyntacticallyValidate(); err != nil {
			// We do not mark as conflict here but error out, because the message should not be part of a sane tangle if the syntax is wrong
			return err
		}

		inputs := message.TransactionEssenceUTXOInputs()

		// go through all the inputs and validate that they are still unspent, in the ledger or were created during confirmation
		inputOutputs := utxo.Outputs{}
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
			output, err = s.UTXO().ReadOutputByOutputIDWithoutLocking(input)
			if err != nil {
				if errors.Is(err, kvstore.ErrKeyNotFound) {
					// input not found, so mark as invalid tx
					conflict = storage.ConflictInputUTXONotFound
					break
				}
				return err
			}

			// check if this output is unspent
			unspent, err := s.UTXO().IsOutputUnspentWithoutLocking(output)
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
			// Dust validation
			dustValidation := iotago.NewDustSemanticValidation(iotago.DustAllowanceDivisor, iotago.MaxDustOutputsOnAddress, func(addr iotago.Address) (dustAllowanceSum uint64, amountDustOutputs int64, err error) {
				return s.UTXO().ReadDustForAddress(addr, wfConf.dustAllowanceDiff)
			})

			// Verify that all outputs consume all inputs and have valid signatures. Also verify that the amounts match.
			mapping, err := inputOutputs.InputToOutputMapping()
			if err != nil {
				return err
			}
			if err := transaction.SemanticallyValidate(mapping, dustValidation); err != nil {

				if errors.Is(err, iotago.ErrMissingUTXO) {
					conflict = storage.ConflictInputUTXONotFound
				} else if errors.Is(err, iotago.ErrInputOutputSumMismatch) {
					conflict = storage.ConflictInputOutputSumMismatch
				} else if errors.Is(err, iotago.ErrEd25519SignatureInvalid) || errors.Is(err, iotago.ErrEd25519PubKeyAndAddrMismatch) {
					conflict = storage.ConflictInvalidSignature
				} else if errors.Is(err, iotago.ErrInvalidDustAllowance) {
					conflict = storage.ConflictInvalidDustAllowance
				} else {
					conflict = storage.ConflictSemanticValidationFailed
				}
			}
		}

		// go through all deposits and generate unspent outputs
		depositOutputs := utxo.Outputs{}
		if conflict == storage.ConflictNone {

			transactionEssence := message.TransactionEssence()
			if transactionEssence == nil {
				return fmt.Errorf("no transaction transactionEssence found")
			}

			for i := 0; i < len(transactionEssence.Outputs); i++ {
				output, err := utxo.NewOutput(message.MessageID(), transaction, uint16(i))
				if err != nil {
					return err
				}
				depositOutputs = append(depositOutputs, output)
			}
		}

		wfConf.MessagesReferenced = append(wfConf.MessagesReferenced, cachedMetadata.Metadata().MessageID())

		if conflict != storage.ConflictNone {
			wfConf.MessagesExcludedWithConflictingTransactions = append(wfConf.MessagesExcludedWithConflictingTransactions, MessageWithConflict{
				MessageID: cachedMetadata.Metadata().MessageID(),
				Conflict:  conflict,
			})
			return nil
		}

		// mark the given message to be part of milestone ledger by changing message inclusion set
		wfConf.MessagesIncludedWithTransactions = append(wfConf.MessagesIncludedWithTransactions, cachedMetadata.Metadata().MessageID())

		newSpents := make(utxo.Spents, len(inputOutputs))

		// save the inputs as spent
		for i, input := range inputOutputs {
			spent := utxo.NewSpent(input, transactionID, msIndex)
			wfConf.NewSpents[string(input.OutputID()[:])] = spent
			newSpents[i] = spent
		}

		// add new outputs
		for _, output := range depositOutputs {
			wfConf.NewOutputs[string(output.OutputID()[:])] = output
		}

		// Apply the new outputs and spents to the current dust allowance diff
		return wfConf.dustAllowanceDiff.Add(depositOutputs, newSpents)
	}

	// we don't need to call cleanup at the end, because we pass our own metadataMemcache.
	parentsTraverser := dag.NewParentTraverser(s, metadataMemcache)

	// This function does the DFS and computes the mutations a white-flag confirmation would create.
	// If the parents are SEPs, are already processed or already referenced,
	// then the mutations from the messages retrieved from the stack are accumulated to the given Confirmation struct's mutations.
	// If the popped message was used to mutate the Confirmation struct, it will also be appended to Confirmation.MessagesIncludedWithTransactions.
	if err := parentsTraverser.Traverse(parents,
		condition,
		consumer,
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false,
		nil); err != nil {
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
