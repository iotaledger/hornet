package whiteflag

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v2"
)

type ConfirmedMilestoneStats struct {
	Index                                       milestone.Index
	ConfirmationTime                            int64
	MessagesReferenced                          int
	MessagesExcludedWithConflictingTransactions int
	MessagesIncludedWithTransactions            int
	MessagesExcludedWithoutTransactions         int
	Collecting                                  time.Duration
	Total                                       time.Duration
}

// ConfirmMilestone traverses a milestone and collects all unreferenced msg,
// then the ledger diffs are calculated, the ledger state is checked and all msg are marked as referenced.
// Additionally, this function also examines the milestone for a receipt and generates new migrated outputs
// if one is present. The treasury is mutated accordingly.
// all cachedMsgMetas have to be released outside.
func ConfirmMilestone(
	s *storage.Storage, serverMetrics *metrics.ServerMetrics,
	cachedMessageMetas map[string]*storage.CachedMetadata,
	milestoneMessageID hornet.MessageID,
	forEachReferencedMessage func(messageMetadata *storage.CachedMetadata, index milestone.Index, confTime uint64),
	onMilestoneConfirmed func(confirmation *Confirmation),
	forEachNewOutput func(output *utxo.Output),
	forEachNewSpent func(spent *utxo.Spent),
	validateAndStoreReceipt func(r *iotago.Receipt) error) (*ConfirmedMilestoneStats, error) {

	cachedMessages := make(map[string]*storage.CachedMessage)

	defer func() {
		// All releases are forced since the cone is referenced and not needed anymore

		// release all messages at the end
		for _, cachedMsg := range cachedMessages {
			cachedMsg.Release(true) // message -1
		}
	}()

	cachedMilestoneMessage := s.GetCachedMessageOrNil(milestoneMessageID)
	if cachedMilestoneMessage == nil {
		return nil, fmt.Errorf("milestone message not found: %v", milestoneMessageID.ToHex())
	}
	defer cachedMilestoneMessage.Release(true)

	cachedMilestoneMessageMapKey := cachedMilestoneMessage.GetMessage().GetMessageID().ToMapKey()
	if _, exists := cachedMessages[cachedMilestoneMessageMapKey]; !exists {
		// release the messages at the end to speed up calculation
		cachedMessages[cachedMilestoneMessageMapKey] = cachedMilestoneMessage.Retain()
	}

	s.UTXO().WriteLockLedger()
	defer s.UTXO().WriteUnlockLedger()
	message := cachedMilestoneMessage.GetMessage()

	ms := message.GetMilestone()
	if ms == nil {
		return nil, fmt.Errorf("confirmMilestone: message does not contain a milestone payload: %v", message.GetMessageID().ToHex())
	}

	msId, err := ms.ID()
	if err != nil {
		return nil, fmt.Errorf("unable to compute milestone Id: %w", err)
	}

	milestoneIndex := milestone.Index(ms.Index)

	ts := time.Now()

	mutations, err := ComputeWhiteFlagMutations(s, milestoneIndex, cachedMessageMetas, cachedMessages, message.GetParents())
	if err != nil {
		// According to the RFC we should panic if we encounter any invalid messages during confirmation
		return nil, fmt.Errorf("confirmMilestone: whiteflag.ComputeConfirmation failed with Error: %v", err)
	}

	confirmation := &Confirmation{
		MilestoneIndex:     milestoneIndex,
		MilestoneMessageID: message.GetMessageID(),
		Mutations:          mutations,
	}

	// Verify the calculated MerkleTreeHash with the one inside the milestone
	merkleTreeHash := ms.InclusionMerkleProof
	if mutations.MerkleTreeHash != merkleTreeHash {
		mutationsMerkleTreeHashSlice := mutations.MerkleTreeHash[:]
		milestoneMerkleTreeHashSlice := merkleTreeHash[:]
		return nil, fmt.Errorf("confirmMilestone: computed MerkleTreeHash %s does not match the value in the milestone %s", hex.EncodeToString(mutationsMerkleTreeHashSlice), hex.EncodeToString(milestoneMerkleTreeHashSlice))
	}

	tc := time.Now()

	newOutputs := make(utxo.Outputs, 0, len(mutations.NewOutputs))
	for _, output := range mutations.NewOutputs {
		newOutputs = append(newOutputs, output)
	}

	var receipt *iotago.Receipt
	var tm *utxo.TreasuryMutationTuple

	// validate receipt and extract migrated funds
	if ms.Receipt != nil {
		var err error

		receipt = ms.Receipt.(*iotago.Receipt)

		// receipt validation is optional
		if validateAndStoreReceipt != nil {
			if err := validateAndStoreReceipt(receipt); err != nil {
				return nil, err
			}
		}

		unspentTreasuryOutput, err := s.UTXO().UnspentTreasuryOutput()
		if err != nil {
			return nil, fmt.Errorf("unable to fetch previous unspent treasury output: %w", err)
		}
		if err := iotago.ValidateReceipt(receipt, &iotago.TreasuryOutput{Amount: unspentTreasuryOutput.Amount}); err != nil {
			return nil, fmt.Errorf("invalid receipt contained within milestone: %w", err)
		}

		migratedOutputs, err := utxo.ReceiptToOutputs(receipt, msId)
		if err != nil {
			return nil, fmt.Errorf("unable to extract migrated outputs from receipt: %w", err)
		}

		tm, err = utxo.ReceiptToTreasuryMutation(receipt, unspentTreasuryOutput, msId)
		if err != nil {
			return nil, fmt.Errorf("unable to convert receipt to treasury mutation tuple: %w", err)
		}

		newOutputs = append(newOutputs, migratedOutputs...)
	}

	newSpents := make(utxo.Spents, 0, len(mutations.NewSpents))
	for _, spent := range mutations.NewSpents {
		newSpents = append(newSpents, spent)
	}

	if err = s.UTXO().ApplyConfirmationWithoutLocking(milestoneIndex, newOutputs, newSpents, tm)
		err != nil {
		return nil, fmt.Errorf("confirmMilestone: utxo.ApplyConfirmation failed with Error: %v", err)
	}

	loadMessageMetadata := func(messageID hornet.MessageID) (*storage.CachedMetadata, error) {
		messageIDMapKey := messageID.ToMapKey()
		cachedMsgMeta, exists := cachedMessageMetas[messageIDMapKey]
		if !exists {
			cachedMsgMeta = s.GetCachedMessageMetadataOrNil(messageID) // meta +1
			if cachedMsgMeta == nil {
				return nil, fmt.Errorf("confirmMilestone: Message not found: %v", messageID.ToHex())
			}
			cachedMessageMetas[messageIDMapKey] = cachedMsgMeta
		}
		return cachedMsgMeta, nil
	}

	// load the message for the given id
	forMessageMetadataWithMessageID := func(messageID hornet.MessageID, do func(meta *storage.CachedMetadata)) error {
		meta, err := loadMessageMetadata(messageID)
		if err != nil {
			return err
		}
		do(meta)
		return nil
	}

	conf := &ConfirmedMilestoneStats{
		Index: milestoneIndex,
	}

	confirmationTime := ms.Timestamp

	// confirm all included messages
	for _, messageID := range mutations.MessagesIncludedWithTransactions {
		if err := forMessageMetadataWithMessageID(messageID, func(meta *storage.CachedMetadata) {
			if !meta.GetMetadata().IsReferenced() {
				meta.GetMetadata().SetReferenced(true, milestoneIndex)
				meta.GetMetadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				conf.MessagesReferenced++
				conf.MessagesIncludedWithTransactions++
				serverMetrics.IncludedTransactionMessages.Inc()
				serverMetrics.ReferencedMessages.Inc()
				forEachReferencedMessage(meta, milestoneIndex, confirmationTime)
			}
		}); err != nil {
			return nil, err
		}
	}

	// confirm all excluded messages not containing ledger transactions
	for _, messageID := range mutations.MessagesExcludedWithoutTransactions {
		if err := forMessageMetadataWithMessageID(messageID, func(meta *storage.CachedMetadata) {
			meta.GetMetadata().SetIsNoTransaction(true)
			if !meta.GetMetadata().IsReferenced() {
				meta.GetMetadata().SetReferenced(true, milestoneIndex)
				meta.GetMetadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				conf.MessagesReferenced++
				conf.MessagesExcludedWithoutTransactions++
				serverMetrics.NoTransactionMessages.Inc()
				serverMetrics.ReferencedMessages.Inc()
				forEachReferencedMessage(meta, milestoneIndex, confirmationTime)
			}
		}); err != nil {
			return nil, err
		}
	}

	// confirm the milestone itself
	if err := forMessageMetadataWithMessageID(milestoneMessageID, func(meta *storage.CachedMetadata) {
		meta.GetMetadata().SetIsNoTransaction(true)
		if !meta.GetMetadata().IsReferenced() {
			meta.GetMetadata().SetReferenced(true, milestoneIndex)
			meta.GetMetadata().SetMilestone(true)
			meta.GetMetadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
			conf.MessagesReferenced++
			conf.MessagesExcludedWithoutTransactions++
			serverMetrics.NoTransactionMessages.Inc()
			serverMetrics.ReferencedMessages.Inc()
			forEachReferencedMessage(meta, milestoneIndex, confirmationTime)
		}
	}); err != nil {
		return nil, err
	}

	// confirm all conflicting messages
	for _, conflictedMessage := range mutations.MessagesExcludedWithConflictingTransactions {
		if err := forMessageMetadataWithMessageID(conflictedMessage.MessageID, func(meta *storage.CachedMetadata) {
			meta.GetMetadata().SetConflictingTx(conflictedMessage.Conflict)
			if !meta.GetMetadata().IsReferenced() {
				meta.GetMetadata().SetReferenced(true, milestoneIndex)
				meta.GetMetadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				conf.MessagesReferenced++
				conf.MessagesExcludedWithConflictingTransactions++
				serverMetrics.ConflictingTransactionMessages.Inc()
				serverMetrics.ReferencedMessages.Inc()
				forEachReferencedMessage(meta, milestoneIndex, confirmationTime)
			}
		}); err != nil {
			return nil, err
		}
	}

	onMilestoneConfirmed(confirmation)

	for _, output := range newOutputs {
		forEachNewOutput(output)
	}

	for _, spent := range newSpents {
		forEachNewSpent(spent)
	}

	conf.Collecting = tc.Sub(ts)
	conf.Total = time.Since(ts)

	return conf, nil
}
