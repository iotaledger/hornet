package whiteflag

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
)

type ConfirmedMilestoneStats struct {
	Index                                       milestone.Index
	ConfirmationTime                            int64
	MessagesReferenced                          int
	MessagesExcludedWithConflictingTransactions int
	MessagesIncludedWithTransactions            int
	MessagesExcludedWithoutTransactions         int
}

// ConfirmationMetrics holds metrics about a confirmation run.
type ConfirmationMetrics struct {
	DurationWhiteflag                                time.Duration
	DurationReceipts                                 time.Duration
	DurationConfirmation                             time.Duration
	DurationLedgerUpdated                            time.Duration
	DurationApplyIncludedWithTransactions            time.Duration
	DurationApplyExcludedWithoutTransactions         time.Duration
	DurationApplyMilestone                           time.Duration
	DurationApplyExcludedWithConflictingTransactions time.Duration
	DurationForEachNewOutput                         time.Duration
	DurationForEachNewSpent                          time.Duration
	DurationOnMilestoneConfirmed                     time.Duration
	DurationSetConfirmedMilestoneIndex               time.Duration
	DurationUpdateConeRootIndexes                    time.Duration
	DurationConfirmedMilestoneChanged                time.Duration
	DurationConfirmedMilestoneIndexChanged           time.Duration
	DurationMilestoneConfirmedSyncEvent              time.Duration
	DurationMilestoneConfirmed                       time.Duration
	DurationTotal                                    time.Duration
}

type CheckMessageReferencedFunc func(meta *storage.MessageMetadata) bool
type SetMessageReferencedFunc func(meta *storage.MessageMetadata, referenced bool, msIndex milestone.Index)

var (
	DefaultCheckMessageReferencedFunc = func(meta *storage.MessageMetadata) bool {
		return meta.IsReferenced()
	}
	DefaultSetMessageReferencedFunc = func(meta *storage.MessageMetadata, referenced bool, msIndex milestone.Index) {
		meta.SetReferenced(referenced, msIndex)
	}
)

// ConfirmMilestone traverses a milestone and collects all unreferenced msg,
// then the ledger diffs are calculated, the ledger state is checked and all msg are marked as referenced.
// Additionally, this function also examines the milestone for a receipt and generates new migrated outputs
// if one is present. The treasury is mutated accordingly.
func ConfirmMilestone(
	utxoManager *utxo.Manager,
	parentsTraverserStorage dag.ParentsTraverserStorage,
	cachedMessageFunc storage.CachedMessageFunc,
	networkId uint64,
	milestoneMessageID hornet.MessageID,
	whiteFlagTraversalCondition dag.Predicate,
	checkMessageReferencedFunc CheckMessageReferencedFunc,
	setMessageReferencedFunc SetMessageReferencedFunc,
	serverMetrics *metrics.ServerMetrics,
	forEachReferencedMessage func(messageMetadata *storage.CachedMetadata, index milestone.Index, confTime uint64),
	onMilestoneConfirmed func(confirmation *Confirmation),
	onLedgerUpdated func(index milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents),
	forEachNewOutput func(index milestone.Index, output *utxo.Output),
	forEachNewSpent func(index milestone.Index, spent *utxo.Spent),
	onReceipt func(r *utxo.ReceiptTuple) error) (*ConfirmedMilestoneStats, *ConfirmationMetrics, error) {

	cachedMsgMilestone, err := cachedMessageFunc(milestoneMessageID) // message +1
	if err != nil {
		return nil, nil, fmt.Errorf("get milestone message failed: %v, error: %w", milestoneMessageID.ToHex(), err)
	}
	if cachedMsgMilestone == nil {
		return nil, nil, fmt.Errorf("milestone message not found: %v", milestoneMessageID.ToHex())
	}
	defer cachedMsgMilestone.Release(true) // message -1

	utxoManager.WriteLockLedger()
	defer utxoManager.WriteUnlockLedger()
	message := cachedMsgMilestone.Message()

	ms := message.Milestone()
	if ms == nil {
		return nil, nil, fmt.Errorf("confirmMilestone: message does not contain a milestone payload: %v", message.MessageID().ToHex())
	}

	msID, err := ms.ID()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to compute milestone Id: %w", err)
	}

	milestoneIndex := milestone.Index(ms.Index)

	timeStart := time.Now()

	parentsTraverser := dag.NewParentsTraverser(parentsTraverserStorage)

	// we pass a background context here to not cancel the whiteflag computation!
	// otherwise the node could panic at shutdown.
	mutations, err := ComputeWhiteFlagMutations(context.Background(), utxoManager, parentsTraverser, cachedMessageFunc, networkId, milestoneIndex, ms.Timestamp, message.Parents(), whiteFlagTraversalCondition)
	if err != nil {
		// According to the RFC we should panic if we encounter any invalid messages during confirmation
		return nil, nil, fmt.Errorf("confirmMilestone: whiteflag.ComputeConfirmation failed with Error: %w", err)
	}

	confirmation := &Confirmation{
		MilestoneIndex:     milestoneIndex,
		MilestoneMessageID: message.MessageID(),
		Mutations:          mutations,
	}

	// Verify the calculated MerkleTreeHash with the one inside the milestone
	merkleTreeHash := ms.InclusionMerkleProof
	if mutations.MerkleTreeHash != merkleTreeHash {
		mutationsMerkleTreeHashSlice := mutations.MerkleTreeHash[:]
		milestoneMerkleTreeHashSlice := merkleTreeHash[:]
		return nil, nil, fmt.Errorf("confirmMilestone: computed MerkleTreeHash %s does not match the value in the milestone %s", hex.EncodeToString(mutationsMerkleTreeHashSlice), hex.EncodeToString(milestoneMerkleTreeHashSlice))
	}
	timeWhiteflag := time.Now()

	newOutputs := make(utxo.Outputs, 0, len(mutations.NewOutputs))
	for _, output := range mutations.NewOutputs {
		newOutputs = append(newOutputs, output)
	}

	var receipt *iotago.Receipt
	var tm *utxo.TreasuryMutationTuple
	var rt *utxo.ReceiptTuple

	// validate receipt and extract migrated funds
	if ms.Receipt != nil {
		var err error

		receipt = ms.Receipt.(*iotago.Receipt)

		rt = &utxo.ReceiptTuple{
			Receipt:        receipt,
			MilestoneIndex: milestone.Index(ms.Index),
		}

		// receipt validation is optional
		if onReceipt != nil {
			if err := onReceipt(rt); err != nil {
				return nil, nil, err
			}
		}

		unspentTreasuryOutput, err := utxoManager.UnspentTreasuryOutputWithoutLocking()
		if err != nil {
			return nil, nil, fmt.Errorf("unable to fetch previous unspent treasury output: %w", err)
		}
		if err := iotago.ValidateReceipt(receipt, &iotago.TreasuryOutput{Amount: unspentTreasuryOutput.Amount}); err != nil {
			return nil, nil, fmt.Errorf("invalid receipt contained within milestone: %w", err)
		}

		migratedOutputs, err := utxo.ReceiptToOutputs(receipt, message.MessageID(), msID, milestoneIndex, ms.Timestamp)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to extract migrated outputs from receipt: %w", err)
		}

		tm, err = utxo.ReceiptToTreasuryMutation(receipt, unspentTreasuryOutput, msID)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to convert receipt to treasury mutation tuple: %w", err)
		}

		newOutputs = append(newOutputs, migratedOutputs...)
	}
	timeReceipts := time.Now()

	newSpents := make(utxo.Spents, 0, len(mutations.NewSpents))
	for _, spent := range mutations.NewSpents {
		newSpents = append(newSpents, spent)
	}

	if err = utxoManager.ApplyConfirmationWithoutLocking(milestoneIndex, newOutputs, newSpents, tm, rt); err != nil {
		return nil, nil, fmt.Errorf("confirmMilestone: utxo.ApplyConfirmation failed: %w", err)
	}
	timeConfirmation := time.Now()

	if onLedgerUpdated != nil {
		onLedgerUpdated(milestoneIndex, newOutputs, newSpents)
	}
	timeLedgerUpdated := time.Now()

	// load the message for the given id
	forMessageMetadataWithMessageID := func(messageID hornet.MessageID, do func(meta *storage.CachedMetadata)) error {
		cachedMsgMeta, err := parentsTraverserStorage.CachedMessageMetadata(messageID) // meta +1
		if err != nil {
			return fmt.Errorf("confirmMilestone: get message failed: %v, Error: %w", messageID.ToHex(), err)
		}
		if cachedMsgMeta == nil {
			return fmt.Errorf("confirmMilestone: message not found: %v", messageID.ToHex())
		}
		do(cachedMsgMeta)
		cachedMsgMeta.Release(true) // meta -1
		return nil
	}

	confirmedMilestoneStats := &ConfirmedMilestoneStats{
		Index: milestoneIndex,
	}
	confirmationTime := ms.Timestamp

	// confirm all included messages
	for _, messageID := range mutations.MessagesIncludedWithTransactions {
		if err := forMessageMetadataWithMessageID(messageID, func(meta *storage.CachedMetadata) {
			if !checkMessageReferencedFunc(meta.Metadata()) {
				setMessageReferencedFunc(meta.Metadata(), true, milestoneIndex)
				meta.Metadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				confirmedMilestoneStats.MessagesReferenced++
				confirmedMilestoneStats.MessagesIncludedWithTransactions++
				if serverMetrics != nil {
					serverMetrics.IncludedTransactionMessages.Inc()
					serverMetrics.ReferencedMessages.Inc()
				}
				if forEachReferencedMessage != nil {
					forEachReferencedMessage(meta, milestoneIndex, confirmationTime)
				}
			}
		}); err != nil {
			return nil, nil, err
		}
	}
	timeApplyIncludedWithTransactions := time.Now()

	// confirm all excluded messages not containing ledger transactions
	for _, messageID := range mutations.MessagesExcludedWithoutTransactions {
		if err := forMessageMetadataWithMessageID(messageID, func(meta *storage.CachedMetadata) {
			meta.Metadata().SetIsNoTransaction(true)
			if !checkMessageReferencedFunc(meta.Metadata()) {
				setMessageReferencedFunc(meta.Metadata(), true, milestoneIndex)
				meta.Metadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				confirmedMilestoneStats.MessagesReferenced++
				confirmedMilestoneStats.MessagesExcludedWithoutTransactions++
				if serverMetrics != nil {
					serverMetrics.NoTransactionMessages.Inc()
					serverMetrics.ReferencedMessages.Inc()
				}
				if forEachReferencedMessage != nil {
					forEachReferencedMessage(meta, milestoneIndex, confirmationTime)
				}
			}
		}); err != nil {
			return nil, nil, err
		}
	}
	timeApplyExcludedWithoutTransactions := time.Now()

	// confirm the milestone itself
	if err := forMessageMetadataWithMessageID(milestoneMessageID, func(meta *storage.CachedMetadata) {
		meta.Metadata().SetIsNoTransaction(true)
		if !checkMessageReferencedFunc(meta.Metadata()) {
			setMessageReferencedFunc(meta.Metadata(), true, milestoneIndex)
			meta.Metadata().SetMilestone(true)
			meta.Metadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
			confirmedMilestoneStats.MessagesReferenced++
			confirmedMilestoneStats.MessagesExcludedWithoutTransactions++
			if serverMetrics != nil {
				serverMetrics.NoTransactionMessages.Inc()
				serverMetrics.ReferencedMessages.Inc()
			}
			if forEachReferencedMessage != nil {
				forEachReferencedMessage(meta, milestoneIndex, confirmationTime)
			}
		}
	}); err != nil {
		return nil, nil, err
	}
	timeApplyMilestone := time.Now()

	// confirm all conflicting messages
	for _, conflictedMessage := range mutations.MessagesExcludedWithConflictingTransactions {
		if err := forMessageMetadataWithMessageID(conflictedMessage.MessageID, func(meta *storage.CachedMetadata) {
			meta.Metadata().SetConflictingTx(conflictedMessage.Conflict)
			if !checkMessageReferencedFunc(meta.Metadata()) {
				setMessageReferencedFunc(meta.Metadata(), true, milestoneIndex)
				meta.Metadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				confirmedMilestoneStats.MessagesReferenced++
				confirmedMilestoneStats.MessagesExcludedWithConflictingTransactions++
				if serverMetrics != nil {
					serverMetrics.ConflictingTransactionMessages.Inc()
					serverMetrics.ReferencedMessages.Inc()
				}
				if forEachReferencedMessage != nil {
					forEachReferencedMessage(meta, milestoneIndex, confirmationTime)
				}
			}
		}); err != nil {
			return nil, nil, err
		}
	}
	timeApplyExcludedWithConflictingTransactions := time.Now()

	if forEachNewOutput != nil {
		for _, output := range newOutputs {
			forEachNewOutput(milestoneIndex, output)
		}
	}
	timeForEachNewOutput := time.Now()

	if forEachNewSpent != nil {
		for _, spent := range newSpents {
			forEachNewSpent(milestoneIndex, spent)
		}
	}
	timeForEachNewSpent := time.Now()

	if onMilestoneConfirmed != nil {
		onMilestoneConfirmed(confirmation)
	}
	timeOnMilestoneConfirmed := time.Now()

	return confirmedMilestoneStats, &ConfirmationMetrics{
		DurationWhiteflag:                                timeWhiteflag.Sub(timeStart),
		DurationReceipts:                                 timeReceipts.Sub(timeWhiteflag),
		DurationConfirmation:                             timeConfirmation.Sub(timeReceipts),
		DurationLedgerUpdated:                            timeLedgerUpdated.Sub(timeConfirmation),
		DurationApplyIncludedWithTransactions:            timeApplyIncludedWithTransactions.Sub(timeLedgerUpdated),
		DurationApplyExcludedWithoutTransactions:         timeApplyExcludedWithoutTransactions.Sub(timeApplyIncludedWithTransactions),
		DurationApplyMilestone:                           timeApplyMilestone.Sub(timeApplyExcludedWithoutTransactions),
		DurationApplyExcludedWithConflictingTransactions: timeApplyExcludedWithConflictingTransactions.Sub(timeApplyMilestone),
		DurationForEachNewOutput:                         timeForEachNewOutput.Sub(timeApplyExcludedWithConflictingTransactions),
		DurationForEachNewSpent:                          timeForEachNewSpent.Sub(timeForEachNewOutput),
		DurationOnMilestoneConfirmed:                     timeOnMilestoneConfirmed.Sub(timeForEachNewSpent),
	}, nil
}
