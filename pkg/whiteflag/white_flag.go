package whiteflag

import (
	"crypto"
	"errors"
	"fmt"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
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
	MilestoneMessageID hornet.Hash
	// The ledger mutations and referenced messages of this milestone.
	Mutations *WhiteFlagMutations
}

// WhiteFlagMutations contains the ledger mutations and referenced messages applied to a cone under the "white-flag" approach.
type WhiteFlagMutations struct {
	// The messages which mutate the ledger in the order in which they were applied.
	MessagesIncluded hornet.Hashes
	// The messages which were excluded as they were conflicting with the mutations.
	MessagesExcludedConflicting hornet.Hashes
	// The messages which were excluded because they were part of a zero or spam value transfer.
	MessagesExcludedZeroValue hornet.Hashes
	// The messages which were referenced by the milestone (should be the sum of MessagesIncluded + MessagesExcludedConflicting + MessagesExcludedZeroValue).
	MessagesReferenced hornet.Hashes
	// Contains the updated state of the addresses which were mutated by the given confirmation.
	NewAddressState map[string]int64
	// Contains the mutations to the state of the addresses for the given confirmation.
	AddressMutations map[string]int64
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
func ComputeWhiteFlagMutations(cachedMessageMetas map[string]*tangle.CachedMetadata, cachedMessages map[string]*tangle.CachedMessage, merkleTreeHashFunc crypto.Hash, parent1MessageID hornet.Hash, parent2MessageID ...hornet.Hash) (*WhiteFlagMutations, error) {
	wfConf := &WhiteFlagMutations{
		MessagesIncluded:            make(hornet.Hashes, 0),
		MessagesExcludedConflicting: make(hornet.Hashes, 0),
		MessagesExcludedZeroValue:   make(hornet.Hashes, 0),
		MessagesReferenced:          make(hornet.Hashes, 0),
		NewAddressState:             make(map[string]int64),
		AddressMutations:            make(map[string]int64),
	}

	// traversal stops if no more messages pass the given condition
	// Caution: condition func is not in DFS order
	condition := func(cachedMetadata *tangle.CachedMetadata) (bool, error) { // meta +1
		defer cachedMetadata.Release(true) // meta -1

		if _, exists := cachedMessageMetas[string(cachedMetadata.GetMetadata().GetMessageID())]; !exists {
			// release the msg metadata at the end to speed up calculation
			cachedMessageMetas[string(cachedMetadata.GetMetadata().GetMessageID())] = cachedMetadata.Retain()
		}

		// only traverse and process the message if it was not confirmed yet
		return !cachedMetadata.GetMetadata().IsConfirmed(), nil
	}

	// consumer
	consumer := func(cachedMetadata *tangle.CachedMetadata) error { // meta +1
		defer cachedMetadata.Release(true) // meta -1

		// load up message
		cachedMessage, exists := cachedMessages[string(cachedMetadata.GetMetadata().GetMessageID())]
		if !exists {
			cachedMessage = tangle.GetCachedMessageOrNil(cachedMetadata.GetMetadata().GetMessageID()) // message +1
			if cachedMessage == nil {
				return fmt.Errorf("%w: message %s of candidate msg %s doesn't exist", tangle.ErrMessageNotFound, cachedMetadata.GetMetadata().GetMessageID().Hex(), cachedMetadata.GetMetadata().GetMessageID().Hex())
			}

			// release the messages at the end to speed up calculation
			cachedMessages[string(cachedMetadata.GetMetadata().GetMessageID())] = cachedMessage
		}

		// exclude non value messages or spam value messages
		//message := cachedMessage.GetMessage()
		//mutations := message.GetLedgerChanges()
		//if message.IsValueSpam() || len(mutations) == 0 {
		//	wfConf.MessagesReferenced = append(wfConf.MessagesReferenced, cachedMetadata.GetMetadata().GetMessageID())
		//	wfConf.MessagesExcludedZeroValue = append(wfConf.MessagesExcludedZeroValue, cachedMetadata.GetMetadata().GetMessageID())
		//	return nil
		//}

		var conflicting bool

		// contains the updated mutations from this message against the
		// current mutations of the milestone's confirming cone (or previous ledger state).
		// we only apply it to the milestone's confirming cone mutations if
		// the message doesn't create any conflict.
		patchedState := make(map[string]int64)
		validMutations := make(map[string]int64)

		//for addr, change := range mutations {
		//
		//	// load state from milestone cone mutation or previous milestone
		//	balance, has := wfConf.NewAddressState[addr]
		//	if !has {
		//		balanceStateFromPreviousMilestone, _, err := tangle.GetBalanceForAddressWithoutLocking(hornet.Hash(addr))
		//		if err != nil {
		//			return fmt.Errorf("%w: unable to retrieve balance of address %s", err, addr)
		//		}
		//		balance = int64(balanceStateFromPreviousMilestone)
		//	}
		//
		//	// note that there's no overflow of int64 values here
		//	// as a valid message's message can not spend more than the total supply,
		//	// meaning that newBalance could be max 2*total_supply or min -total_supply.
		//	newBalance := balance + change
		//
		//	// on below zero or above total supply the mutation is invalid
		//	if newBalance < 0 || math.AbsInt64(newBalance) > consts.TotalSupply {
		//		conflicting = true
		//		break
		//	}
		//
		//	patchedState[addr] = newBalance
		//	validMutations[addr] = validMutations[addr] + change
		//}

		wfConf.MessagesReferenced = append(wfConf.MessagesReferenced, cachedMetadata.GetMetadata().GetMessageID())

		if conflicting {
			wfConf.MessagesExcludedConflicting = append(wfConf.MessagesExcludedConflicting, cachedMetadata.GetMetadata().GetMessageID())
			return nil
		}

		// mark the given message to be part of milestone ledger by changing message inclusion set
		wfConf.MessagesIncluded = append(wfConf.MessagesIncluded, cachedMetadata.GetMetadata().GetMessageID())

		// incorporate the mutations in accordance with the previous mutations
		// in the milestone's confirming cone/previous ledger state.
		for addr, balance := range patchedState {
			wfConf.NewAddressState[addr] = balance
		}

		// incorporate the mutations in accordance with the previous mutations
		for addr, mutation := range validMutations {
			wfConf.AddressMutations[addr] = wfConf.AddressMutations[addr] + mutation
		}

		return nil
	}

	// This function does the DFS and computes the mutations a white-flag confirmation would create.
	// If parent1 and parent2 of a message are both SEPs, are already processed or already confirmed,
	// then the mutations from the messages retrieved from the stack are accumulated to the given Confirmation struct's mutations.
	// If the popped message was used to mutate the Confirmation struct, it will also be appended to Confirmation.MessagesIncluded.
	if len(parent2MessageID) == 0 {
		// no parent2 message ID given, only walk parent 1
		if err := dag.TraverseParents(parent1MessageID,
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
	} else {
		// parent2 message ID given, first walk parent1 then parent2
		if err := dag.TraverseParent1AndParent2(parent1MessageID, parent2MessageID[0],
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
	}

	// compute merkle tree root hash
	merkleTreeHash := NewHasher(merkleTreeHashFunc).TreeHash(wfConf.MessagesIncluded)
	copy(wfConf.MerkleTreeHash[:], merkleTreeHash[:64])

	if len(wfConf.MessagesIncluded) != (len(wfConf.MessagesReferenced) - len(wfConf.MessagesExcludedConflicting) - len(wfConf.MessagesExcludedZeroValue)) {
		return nil, ErrIncludedMessagesSumDoesntMatch
	}

	return wfConf, nil
}
