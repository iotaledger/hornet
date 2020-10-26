package whiteflag

import (
	"crypto"
	"errors"
	"fmt"

	iotago "github.com/iotaledger/iota.go"

	"github.com/iotaledger/hive.go/kvstore"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/model/utxo"
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
	MilestoneMessageID *hornet.MessageID
	// The ledger mutations and referenced messages of this milestone.
	Mutations *WhiteFlagMutations
}

// WhiteFlagMutations contains the ledger mutations and referenced messages applied to a cone under the "white-flag" approach.
type WhiteFlagMutations struct {
	// The messages which mutate the ledger in the order in which they were applied.
	MessagesIncludedWithTransactions hornet.MessageIDs
	// The messages which were excluded as they were conflicting with the mutations.
	MessagesExcludedWithConflictingTransactions hornet.MessageIDs
	// The messages which were excluded because they did not include a value transaction.
	MessagesExcludedWithoutTransactions hornet.MessageIDs
	// The messages which were referenced by the milestone (should be the sum of MessagesIncludedWithTransactions + MessagesExcludedWithConflictingTransactions + MessagesExcludedWithoutTransactions).
	MessagesReferenced hornet.MessageIDs
	// Contains the newly created Unspent Outputs by the given confirmation.
	NewOutputs map[string]*utxo.Output
	// Contains the Spent Outputs for the given confirmation.
	NewSpents map[string]*utxo.Spent
	// The merkle tree root hash of all messages.
	MerkleTreeHash [64]byte
}

// ComputeConfirmation computes the ledger changes in accordance to the white-flag rules for the cone referenced by parent1 and parent2.
// Via a post-order depth-first search the approved messages of the given cone are traversed and
// in their corresponding order applied/mutated against the previous ledger state, respectively previous applied mutations.
// Messages within the approving cone must be valid. Messages causing conflicts are ignored but do not create an error.
// It also computes the merkle tree root hash consisting out of the IDs of the messages which are part of the set
// which mutated the ledger state when applying the white-flag approach.
// The ledger state must be write locked while this function is getting called in order to ensure consistency.
// all cachedMsgMetas and cachedMessages have to be released outside.
func ComputeWhiteFlagMutations(msIndex milestone.Index, cachedMessageMetas map[string]*tangle.CachedMetadata, cachedMessages map[string]*tangle.CachedMessage, merkleTreeHashFunc crypto.Hash, parent1MessageID *hornet.MessageID, parent2MessageID *hornet.MessageID) (*WhiteFlagMutations, error) {
	wfConf := &WhiteFlagMutations{
		MessagesIncludedWithTransactions:            make(hornet.MessageIDs, 0),
		MessagesExcludedWithConflictingTransactions: make(hornet.MessageIDs, 0),
		MessagesExcludedWithoutTransactions:         make(hornet.MessageIDs, 0),
		MessagesReferenced:                          make(hornet.MessageIDs, 0),
		NewOutputs:                                  make(map[string]*utxo.Output),
		NewSpents:                                   make(map[string]*utxo.Spent),
	}

	// traversal stops if no more messages pass the given condition
	// Caution: condition func is not in DFS order
	condition := func(cachedMetadata *tangle.CachedMetadata) (bool, error) { // meta +1
		defer cachedMetadata.Release(true) // meta -1

		cachedMetadataMapKey := cachedMetadata.GetMetadata().GetMessageID().MapKey()
		if _, exists := cachedMessageMetas[cachedMetadataMapKey]; !exists {
			// release the msg metadata at the end to speed up calculation
			cachedMessageMetas[cachedMetadataMapKey] = cachedMetadata.Retain()
		}

		// only traverse and process the message if it was not referenced yet
		return !cachedMetadata.GetMetadata().IsReferenced(), nil
	}

	// consumer
	consumer := func(cachedMetadata *tangle.CachedMetadata) error { // meta +1
		defer cachedMetadata.Release(true) // meta -1

		cachedMetadataMapKey := cachedMetadata.GetMetadata().GetMessageID().MapKey()

		// load up message
		cachedMessage, exists := cachedMessages[cachedMetadataMapKey]
		if !exists {
			cachedMessage = tangle.GetCachedMessageOrNil(cachedMetadata.GetMetadata().GetMessageID()) // message +1
			if cachedMessage == nil {
				return fmt.Errorf("%w: message %s of candidate msg %s doesn't exist", tangle.ErrMessageNotFound, cachedMetadata.GetMetadata().GetMessageID().Hex(), cachedMetadata.GetMetadata().GetMessageID().Hex())
			}

			// release the messages at the end to speed up calculation
			cachedMessages[cachedMetadataMapKey] = cachedMessage
		}

		message := cachedMessage.GetMessage()

		// exclude message without transactions
		if !message.IsTransaction() {
			wfConf.MessagesReferenced = append(wfConf.MessagesReferenced, message.GetMessageID())
			wfConf.MessagesExcludedWithoutTransactions = append(wfConf.MessagesExcludedWithoutTransactions, message.GetMessageID())
			return nil
		}

		var conflicting bool

		transaction := message.GetTransaction()
		transactionID, err := transaction.ID()
		if err != nil {
			return err
		}

		transactionEssence := message.GetTransactionEssence()
		if transactionEssence == nil {
			return fmt.Errorf("no transaction transactionEssence found")
		}

		inputs := message.GetTransactionEssenceUTXOInputs()

		// go through all the inputs and validate that they are still unspent, in the ledger or were created during confirmation
		// also sum up the amount required
		var inputOutputs utxo.Outputs
		var inputAmount uint64
		for inputIndex, input := range inputs {

			// check if this input was already spent during the confirmation
			_, hasSpent := wfConf.NewSpents[string(input[:])]
			if hasSpent {
				// UTXO already spent, so mark as conflicting
				conflicting = true
				break
			}

			// check if this input was newly created during the confirmation
			output, hasOutput := wfConf.NewOutputs[string(input[:])]
			if hasOutput {
				// UTXO is in the current ledger mutation, so use it
				inputOutputs = append(inputOutputs, output)
				inputAmount += output.Amount()
				continue
			}

			// check current ledger for this input
			output, err := utxo.ReadOutputByOutputIDWithoutLocking(input)
			if err != nil {
				if err == kvstore.ErrKeyNotFound {
					// input not found, so mark as invalid tx
					conflicting = true
					break
				}
				return err
			}

			// check if this output is unspent
			unspent, err := output.IsUnspentWithoutLocking()
			if err != nil {
				return err
			}

			if !unspent {
				// output is already spent, so mark as conflicting
				conflicting = true
				break
			}

			// validate signature
			signature := message.GetSignatureForInputIndex(uint16(inputIndex))
			if signature == nil {
				// no valid signature found for index
				conflicting = true
				break
			}

			unsignedTransactionBytes, err := transactionEssence.Serialize(iotago.DeSeriModeNoValidation)
			if err != nil {
				return err
			}
			if err := signature.Valid(unsignedTransactionBytes, output.Address()); err != nil {
				// invalid signature
				conflicting = true
				break
			}

			inputOutputs = append(inputOutputs, output)
			inputAmount += output.Amount()
		}

		// go through all deposits and generate unspent outputs
		var outputAmount uint64
		var depositOutputs utxo.Outputs
		if !conflicting {
			for i := 0; i < len(transactionEssence.Outputs); i++ {
				output, err := utxo.NewOutput(message.GetMessageID(), transaction, uint16(i))
				if err != nil {
					return err
				}
				depositOutputs = append(depositOutputs, output)
				outputAmount += output.Amount()
			}

			// check that the transaction is consuming and sending the same amount
			if inputAmount != outputAmount {
				conflicting = true
			}
		}

		wfConf.MessagesReferenced = append(wfConf.MessagesReferenced, cachedMetadata.GetMetadata().GetMessageID())

		if conflicting {
			wfConf.MessagesExcludedWithConflictingTransactions = append(wfConf.MessagesExcludedWithConflictingTransactions, cachedMetadata.GetMetadata().GetMessageID())
			return nil
		}

		// mark the given message to be part of milestone ledger by changing message inclusion set
		wfConf.MessagesIncludedWithTransactions = append(wfConf.MessagesIncludedWithTransactions, cachedMetadata.GetMetadata().GetMessageID())

		// save the inputs as spent
		for _, input := range inputOutputs {
			delete(wfConf.NewOutputs, string(input.OutputID()[:]))
			wfConf.NewSpents[string(input.OutputID()[:])] = utxo.NewSpent(input, transactionID, msIndex)
		}

		// add new outputs
		for _, output := range depositOutputs {
			wfConf.NewOutputs[string(output.OutputID()[:])] = output
		}

		return nil
	}

	// This function does the DFS and computes the mutations a white-flag confirmation would create.
	// If parent1 and parent2 of a message are both SEPs, are already processed or already referenced,
	// then the mutations from the messages retrieved from the stack are accumulated to the given Confirmation struct's mutations.
	// If the popped message was used to mutate the Confirmation struct, it will also be appended to Confirmation.MessagesIncludedWithTransactions.
	if err := dag.TraverseParent1AndParent2(parent1MessageID, parent2MessageID,
		condition,
		consumer,
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false, nil); err != nil {
		return nil, err
	}

	// compute merkle tree root hash
	merkleTreeHash := NewHasher(merkleTreeHashFunc).TreeHash(wfConf.MessagesIncludedWithTransactions)
	copy(wfConf.MerkleTreeHash[:], merkleTreeHash[:64])

	if len(wfConf.MessagesIncludedWithTransactions) != (len(wfConf.MessagesReferenced) - len(wfConf.MessagesExcludedWithConflictingTransactions) - len(wfConf.MessagesExcludedWithoutTransactions)) {
		return nil, ErrIncludedMessagesSumDoesntMatch
	}

	return wfConf, nil
}
