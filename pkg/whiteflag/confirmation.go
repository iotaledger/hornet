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
	Index                                     milestone.Index
	ConfirmationTime                          int64
	BlocksReferenced                          int
	BlocksExcludedWithConflictingTransactions int
	BlocksIncludedWithTransactions            int
	BlocksExcludedWithoutTransactions         int
}

// ConfirmationMetrics holds metrics about a confirmation run.
type ConfirmationMetrics struct {
	DurationWhiteflag                                time.Duration
	DurationReceipts                                 time.Duration
	DurationConfirmation                             time.Duration
	DurationLedgerUpdated                            time.Duration
	DurationTreasuryMutated                          time.Duration
	DurationApplyIncludedWithTransactions            time.Duration
	DurationApplyExcludedWithoutTransactions         time.Duration
	DurationApplyExcludedWithConflictingTransactions time.Duration
	DurationOnMilestoneConfirmed                     time.Duration
	DurationSetConfirmedMilestoneIndex               time.Duration
	DurationUpdateConeRootIndexes                    time.Duration
	DurationConfirmedMilestoneChanged                time.Duration
	DurationConfirmedMilestoneIndexChanged           time.Duration
	DurationMilestoneConfirmedSyncEvent              time.Duration
	DurationMilestoneConfirmed                       time.Duration
	DurationTotal                                    time.Duration
}

type CheckBlockReferencedFunc func(meta *storage.BlockMetadata) bool
type SetBlockReferencedFunc func(meta *storage.BlockMetadata, referenced bool, msIndex milestone.Index)

var (
	DefaultCheckBlockReferencedFunc = func(meta *storage.BlockMetadata) bool {
		return meta.IsReferenced()
	}
	DefaultSetBlockReferencedFunc = func(meta *storage.BlockMetadata, referenced bool, msIndex milestone.Index) {
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
	cachedBlockFunc storage.CachedBlockFunc,
	protoParas *iotago.ProtocolParameters,
	milestonePayload *iotago.Milestone,
	whiteFlagTraversalCondition dag.Predicate,
	checkBlockReferencedFunc CheckBlockReferencedFunc,
	setBlockReferencedFunc SetBlockReferencedFunc,
	serverMetrics *metrics.ServerMetrics,
	forEachReferencedBlock func(blockMetadata *storage.CachedMetadata, index milestone.Index, confTime uint32),
	onMilestoneConfirmed func(confirmation *Confirmation),
	onLedgerUpdated func(index milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents),
	onTreasuryMutated func(index milestone.Index, tuple *utxo.TreasuryMutationTuple),
	onReceipt func(r *utxo.ReceiptTuple) error) (*ConfirmedMilestoneStats, *ConfirmationMetrics, error) {

	utxoManager.WriteLockLedger()
	defer utxoManager.WriteUnlockLedger()

	msIDPtr, err := milestonePayload.ID()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to compute milestone Id: %w", err)
	}

	milestoneID := *msIDPtr
	previousMilestoneID := milestonePayload.PreviousMilestoneID
	milestoneIndex := milestone.Index(milestonePayload.Index)
	milestoneTimestamp := milestonePayload.Timestamp
	milestoneParents := hornet.BlockIDsFromSliceOfArrays(milestonePayload.Parents)

	timeStart := time.Now()

	parentsTraverser := dag.NewParentsTraverser(parentsTraverserStorage)

	// we pass a background context here to not cancel the whiteflag computation!
	// otherwise the node could panic at shutdown.
	mutations, err := ComputeWhiteFlagMutations(
		context.Background(),
		utxoManager,
		parentsTraverser,
		cachedBlockFunc,
		milestoneIndex,
		milestoneTimestamp,
		milestoneParents,
		previousMilestoneID,
		whiteFlagTraversalCondition)
	if err != nil {
		// According to the RFC we should panic if we encounter any invalid blocks during confirmation
		return nil, nil, fmt.Errorf("confirmMilestone: whiteflag.ComputeConfirmation failed with Error: %w", err)
	}

	confirmation := &Confirmation{
		MilestoneIndex:   milestoneIndex,
		MilestoneID:      milestoneID,
		MilestoneParents: milestoneParents,
		Mutations:        mutations,
	}

	// Verify the calculated InclusionMerkleRoot with the one inside the milestone
	inclusionMerkleTreeHash := milestonePayload.InclusionMerkleRoot
	if mutations.InclusionMerkleRoot != inclusionMerkleTreeHash {
		return nil, nil, fmt.Errorf("confirmMilestone: computed InclusionMerkleRoot %s does not match the value in the milestone %s", hex.EncodeToString(mutations.InclusionMerkleRoot[:]), hex.EncodeToString(inclusionMerkleTreeHash[:]))
	}

	// Verify the calculated AppliedMerkleRoot with the one inside the milestone
	appliedMerkleTreeHash := milestonePayload.AppliedMerkleRoot
	if mutations.AppliedMerkleRoot != appliedMerkleTreeHash {
		return nil, nil, fmt.Errorf("confirmMilestone: computed AppliedMerkleRoot %s does not match the value in the milestone %s", hex.EncodeToString(mutations.AppliedMerkleRoot[:]), hex.EncodeToString(appliedMerkleTreeHash[:]))
	}

	timeWhiteflag := time.Now()

	newOutputs := make(utxo.Outputs, 0, len(mutations.NewOutputs))
	for _, output := range mutations.NewOutputs {
		newOutputs = append(newOutputs, output)
	}

	var tm *utxo.TreasuryMutationTuple
	var rt *utxo.ReceiptTuple

	// validate receipt and extract migrated funds
	opts, err := milestonePayload.Opts.Set()
	if err != nil {
		return nil, nil, err
	}

	receipt := opts.Receipt()
	if receipt != nil {
		var err error

		rt = &utxo.ReceiptTuple{
			Receipt:        receipt,
			MilestoneIndex: milestoneIndex,
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
		if err := iotago.ValidateReceipt(receipt, &iotago.TreasuryOutput{Amount: unspentTreasuryOutput.Amount}, protoParas.TokenSupply); err != nil {
			return nil, nil, fmt.Errorf("invalid receipt contained within milestone: %w", err)
		}

		migratedOutputs, err := utxo.ReceiptToOutputs(receipt, milestoneID, milestoneIndex, milestoneTimestamp)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to extract migrated outputs from receipt: %w", err)
		}

		tm, err = utxo.ReceiptToTreasuryMutation(receipt, unspentTreasuryOutput, milestoneID)
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

	// load the block for the given id
	forBlockMetadataWithBlockID := func(blockID hornet.BlockID, do func(meta *storage.CachedMetadata)) error {
		cachedBlockMeta, err := parentsTraverserStorage.CachedBlockMetadata(blockID) // meta +1
		if err != nil {
			return fmt.Errorf("confirmMilestone: get block failed: %v, Error: %w", blockID.ToHex(), err)
		}
		if cachedBlockMeta == nil {
			return fmt.Errorf("confirmMilestone: block not found: %v", blockID.ToHex())
		}
		do(cachedBlockMeta)
		cachedBlockMeta.Release(true) // meta -1
		return nil
	}

	confirmedMilestoneStats := &ConfirmedMilestoneStats{
		Index: milestoneIndex,
	}

	confirmationTime := milestonePayload.Timestamp

	// confirm all included blocks
	for _, blockID := range mutations.BlocksIncludedWithTransactions {
		if err := forBlockMetadataWithBlockID(blockID, func(meta *storage.CachedMetadata) {
			if !checkBlockReferencedFunc(meta.Metadata()) {
				setBlockReferencedFunc(meta.Metadata(), true, milestoneIndex)
				meta.Metadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				confirmedMilestoneStats.BlocksReferenced++
				confirmedMilestoneStats.BlocksIncludedWithTransactions++
				if serverMetrics != nil {
					serverMetrics.IncludedTransactionBlocks.Inc()
					serverMetrics.ReferencedBlocks.Inc()
				}
				if forEachReferencedBlock != nil {
					forEachReferencedBlock(meta, milestoneIndex, confirmationTime)
				}
			}
		}); err != nil {
			return nil, nil, err
		}
	}
	timeApplyIncludedWithTransactions := time.Now()

	// confirm all excluded blocks not containing ledger transactions
	for _, blockID := range mutations.BlocksExcludedWithoutTransactions {
		if err := forBlockMetadataWithBlockID(blockID, func(meta *storage.CachedMetadata) {
			meta.Metadata().SetIsNoTransaction(true)
			if !checkBlockReferencedFunc(meta.Metadata()) {
				setBlockReferencedFunc(meta.Metadata(), true, milestoneIndex)
				meta.Metadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				confirmedMilestoneStats.BlocksReferenced++
				confirmedMilestoneStats.BlocksExcludedWithoutTransactions++
				if serverMetrics != nil {
					serverMetrics.NoTransactionBlocks.Inc()
					serverMetrics.ReferencedBlocks.Inc()
				}
				if forEachReferencedBlock != nil {
					forEachReferencedBlock(meta, milestoneIndex, confirmationTime)
				}
			}
		}); err != nil {
			return nil, nil, err
		}
	}
	timeApplyExcludedWithoutTransactions := time.Now()

	// confirm all conflicting blocks
	for _, conflictedBlock := range mutations.BlocksExcludedWithConflictingTransactions {
		if err := forBlockMetadataWithBlockID(conflictedBlock.BlockID, func(meta *storage.CachedMetadata) {
			meta.Metadata().SetConflictingTx(conflictedBlock.Conflict)
			if !checkBlockReferencedFunc(meta.Metadata()) {
				setBlockReferencedFunc(meta.Metadata(), true, milestoneIndex)
				meta.Metadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				confirmedMilestoneStats.BlocksReferenced++
				confirmedMilestoneStats.BlocksExcludedWithConflictingTransactions++
				if serverMetrics != nil {
					serverMetrics.ConflictingTransactionBlocks.Inc()
					serverMetrics.ReferencedBlocks.Inc()
				}
				if forEachReferencedBlock != nil {
					forEachReferencedBlock(meta, milestoneIndex, confirmationTime)
				}
			}
		}); err != nil {
			return nil, nil, err
		}
	}
	timeApplyExcludedWithConflictingTransactions := time.Now()

	if onMilestoneConfirmed != nil {
		onMilestoneConfirmed(confirmation)
	}
	timeOnMilestoneConfirmed := time.Now()

	if onLedgerUpdated != nil {
		onLedgerUpdated(milestoneIndex, newOutputs, newSpents)
	}
	timeLedgerUpdated := time.Now()

	if onTreasuryMutated != nil && tm != nil {
		onTreasuryMutated(milestoneIndex, tm)
	}
	timeTreasuryMutated := time.Now()

	return confirmedMilestoneStats, &ConfirmationMetrics{
		DurationWhiteflag:                                timeWhiteflag.Sub(timeStart),
		DurationReceipts:                                 timeReceipts.Sub(timeWhiteflag),
		DurationConfirmation:                             timeConfirmation.Sub(timeReceipts),
		DurationApplyIncludedWithTransactions:            timeApplyIncludedWithTransactions.Sub(timeConfirmation),
		DurationApplyExcludedWithoutTransactions:         timeApplyExcludedWithoutTransactions.Sub(timeApplyIncludedWithTransactions),
		DurationApplyExcludedWithConflictingTransactions: timeApplyExcludedWithConflictingTransactions.Sub(timeApplyExcludedWithoutTransactions),
		DurationOnMilestoneConfirmed:                     timeOnMilestoneConfirmed.Sub(timeApplyExcludedWithConflictingTransactions),
		DurationLedgerUpdated:                            timeLedgerUpdated.Sub(timeOnMilestoneConfirmed),
		DurationTreasuryMutated:                          timeTreasuryMutated.Sub(timeLedgerUpdated),
	}, nil
}
