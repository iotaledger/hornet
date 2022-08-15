package whiteflag

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
)

type ConfirmedMilestoneStats struct {
	Index                                     iotago.MilestoneIndex
	ConfirmationTime                          int64
	BlocksReferenced                          int
	BlocksExcludedWithConflictingTransactions int
	BlocksIncludedWithTransactions            int
	BlocksExcludedWithoutTransactions         int
}

// ConfirmationMetrics holds metrics about a confirmation run.
type ConfirmationMetrics struct {
	DurationWhiteflag                      time.Duration
	DurationReceipts                       time.Duration
	DurationConfirmation                   time.Duration
	DurationApplyConfirmation              time.Duration
	DurationOnMilestoneConfirmed           time.Duration
	DurationLedgerUpdated                  time.Duration
	DurationTreasuryMutated                time.Duration
	DurationSetConfirmedMilestoneIndex     time.Duration
	DurationUpdateConeRootIndexes          time.Duration
	DurationConfirmedMilestoneChanged      time.Duration
	DurationConfirmedMilestoneIndexChanged time.Duration
	DurationTotal                          time.Duration
}

type CheckBlockReferencedFunc func(meta *storage.BlockMetadata) bool
type SetBlockReferencedFunc func(meta *storage.BlockMetadata, referenced bool, msIndex iotago.MilestoneIndex, wfIndex uint32)

var (
	DefaultCheckBlockReferencedFunc = func(meta *storage.BlockMetadata) bool {
		return meta.IsReferenced()
	}
	DefaultSetBlockReferencedFunc = func(meta *storage.BlockMetadata, referenced bool, msIndex iotago.MilestoneIndex, wfIndex uint32) {
		meta.SetReferenced(referenced, msIndex, wfIndex)
	}
)

// ConfirmMilestone traverses a milestone and collects all unreferenced blocks,
// then the ledger diffs are calculated, the ledger state is checked and all blocks are marked as referenced.
// Additionally, this function also examines the milestone for a receipt and generates new migrated outputs
// if one is present. The treasury is mutated accordingly.
func ConfirmMilestone(
	utxoManager *utxo.Manager,
	parentsTraverserStorage dag.ParentsTraverserStorage,
	cachedBlockFunc storage.CachedBlockFunc,
	protoParams *iotago.ProtocolParameters,
	genesisMilestoneIndex iotago.MilestoneIndex,
	milestonePayload *iotago.Milestone,
	whiteFlagTraversalCondition dag.Predicate,
	checkBlockReferencedFunc CheckBlockReferencedFunc,
	setBlockReferencedFunc SetBlockReferencedFunc,
	serverMetrics *metrics.ServerMetrics,
	onValidateReceipt func(r *utxo.ReceiptTuple) error,
	onMilestoneConfirmed func(confirmation *Confirmation),
	forEachReferencedBlock func(blockMetadata *storage.CachedMetadata, index iotago.MilestoneIndex, confTime uint32),
	onLedgerUpdated func(index iotago.MilestoneIndex, newOutputs utxo.Outputs, newSpents utxo.Spents),
	onTreasuryMutated func(index iotago.MilestoneIndex, tuple *utxo.TreasuryMutationTuple),
) (*ConfirmedMilestoneStats, *ConfirmationMetrics, error) {

	msID, err := milestonePayload.ID()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to compute milestone Id: %w", err)
	}

	milestoneID := msID
	previousMilestoneID := milestonePayload.PreviousMilestoneID
	milestoneIndex := milestonePayload.Index
	milestoneTimestamp := milestonePayload.Timestamp
	milestoneParents := milestonePayload.Parents

	var (
		timeStart                time.Time
		timeWhiteflag            time.Time
		timeReceipts             time.Time
		timeConfirmation         time.Time
		timeApplyConfirmation    time.Time
		timeOnMilestoneConfirmed time.Time
		timeLedgerUpdatedStart   time.Time
		timeLedgerUpdatedEnd     time.Time
		timeTreasuryMutatedStart time.Time
		timeTreasuryMutatedEnd   time.Time
	)

	parentsTraverser := dag.NewParentsTraverser(parentsTraverserStorage)

	newOutputs := make(utxo.Outputs, 0)
	newSpents := make(utxo.Spents, 0)

	var newReceipt *utxo.ReceiptTuple
	var treasuryMutation *utxo.TreasuryMutationTuple

	confirmation := &Confirmation{
		MilestoneIndex:   milestoneIndex,
		MilestoneID:      milestoneID,
		MilestoneParents: milestoneParents,
	}

	confirmedMilestoneStats := &ConfirmedMilestoneStats{
		Index: milestoneIndex,
	}

	// load the block for the given id
	forBlockMetadataWithBlockID := func(blockID iotago.BlockID, do func(meta *storage.CachedMetadata)) error {
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

	// we use this inline function for easier unlocking of the ledger at the end
	calculateAndApplyLedgerChanges := func() error {
		utxoManager.WriteLockLedger()
		defer utxoManager.WriteUnlockLedger()

		timeStart = time.Now()

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
			genesisMilestoneIndex,
			whiteFlagTraversalCondition)
		if err != nil {
			// According to the RFC we should panic if we encounter any invalid blocks during confirmation
			return fmt.Errorf("confirmMilestone: whiteflag.ComputeConfirmation failed with Error: %w", err)
		}
		confirmation.Mutations = mutations

		// Verify the calculated InclusionMerkleRoot with the one inside the milestone
		inclusionMerkleTreeHash := milestonePayload.InclusionMerkleRoot
		if mutations.InclusionMerkleRoot != inclusionMerkleTreeHash {
			return fmt.Errorf("confirmMilestone: computed InclusionMerkleRoot %s does not match the value in the milestone %s", hex.EncodeToString(mutations.InclusionMerkleRoot[:]), hex.EncodeToString(inclusionMerkleTreeHash[:]))
		}

		// Verify the calculated AppliedMerkleRoot with the one inside the milestone
		appliedMerkleTreeHash := milestonePayload.AppliedMerkleRoot
		if mutations.AppliedMerkleRoot != appliedMerkleTreeHash {
			return fmt.Errorf("confirmMilestone: computed AppliedMerkleRoot %s does not match the value in the milestone %s", hex.EncodeToString(mutations.AppliedMerkleRoot[:]), hex.EncodeToString(appliedMerkleTreeHash[:]))
		}

		for _, output := range mutations.NewOutputs {
			newOutputs = append(newOutputs, output)
		}

		for _, spent := range mutations.NewSpents {
			newSpents = append(newSpents, spent)
		}

		timeWhiteflag = time.Now()

		// validate receipt and extract migrated funds
		opts, err := milestonePayload.Opts.Set()
		if err != nil {
			return err
		}

		receipt := opts.Receipt()
		if receipt != nil {
			var err error

			newReceipt = &utxo.ReceiptTuple{
				Receipt:        receipt,
				MilestoneIndex: milestoneIndex,
			}

			// receipt validation is optional
			if onValidateReceipt != nil {
				if err := onValidateReceipt(newReceipt); err != nil {
					return err
				}
			}

			unspentTreasuryOutput, err := utxoManager.UnspentTreasuryOutputWithoutLocking()
			if err != nil {
				return fmt.Errorf("unable to fetch previous unspent treasury output: %w", err)
			}

			if err := iotago.ValidateReceipt(receipt, &iotago.TreasuryOutput{Amount: unspentTreasuryOutput.Amount}, protoParams.TokenSupply); err != nil {
				return fmt.Errorf("invalid receipt contained within milestone: %w", err)
			}

			migratedOutputs, err := utxo.ReceiptToOutputs(receipt, milestoneID, milestoneIndex, milestoneTimestamp)
			if err != nil {
				return fmt.Errorf("unable to extract migrated outputs from receipt: %w", err)
			}

			treasuryMutation, err = utxo.ReceiptToTreasuryMutation(receipt, unspentTreasuryOutput, milestoneID)
			if err != nil {
				return fmt.Errorf("unable to convert receipt to treasury mutation tuple: %w", err)
			}

			newOutputs = append(newOutputs, migratedOutputs...)
		}
		timeReceipts = time.Now()

		if err = utxoManager.ApplyConfirmationWithoutLocking(milestoneIndex, newOutputs, newSpents, treasuryMutation, newReceipt); err != nil {
			return fmt.Errorf("confirmMilestone: utxo.ApplyConfirmation failed: %w", err)
		}
		timeConfirmation = time.Now()

		// mark all blocks as referenced
		for wfIndex, referencedBlock := range mutations.ReferencedBlocks {
			if err := forBlockMetadataWithBlockID(referencedBlock.BlockID, func(meta *storage.CachedMetadata) {

				if referencedBlock.IsTransaction {
					if referencedBlock.Conflict != storage.ConflictNone {
						meta.Metadata().SetConflictingTx(referencedBlock.Conflict)
					}
				} else {
					meta.Metadata().SetIsNoTransaction(true)
				}

				if !checkBlockReferencedFunc(meta.Metadata()) {
					setBlockReferencedFunc(meta.Metadata(), true, milestoneIndex, uint32(wfIndex))
					meta.Metadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)

					confirmedMilestoneStats.BlocksReferenced++
					if serverMetrics != nil {
						serverMetrics.ReferencedBlocks.Inc()
					}

					if referencedBlock.IsTransaction {
						if referencedBlock.Conflict != storage.ConflictNone {
							confirmedMilestoneStats.BlocksExcludedWithConflictingTransactions++
							if serverMetrics != nil {
								serverMetrics.ConflictingTransactionBlocks.Inc()
							}
						} else {
							confirmedMilestoneStats.BlocksIncludedWithTransactions++
							if serverMetrics != nil {
								serverMetrics.IncludedTransactionBlocks.Inc()
							}
						}
					} else {
						confirmedMilestoneStats.BlocksExcludedWithoutTransactions++
						if serverMetrics != nil {
							serverMetrics.NoTransactionBlocks.Inc()
						}
					}

				}
			}); err != nil {
				return err
			}
		}
		timeApplyConfirmation = time.Now()

		if onMilestoneConfirmed != nil {
			onMilestoneConfirmed(confirmation)
		}

		timeOnMilestoneConfirmed = time.Now()

		return nil
	}

	err = calculateAndApplyLedgerChanges()
	if err != nil {
		return nil, nil, err
	}

	// fire all events after the ledger got unlocked
	for _, blockWithStatus := range confirmation.Mutations.ReferencedBlocks {
		if err := forBlockMetadataWithBlockID(blockWithStatus.BlockID, func(meta *storage.CachedMetadata) {
			if forEachReferencedBlock != nil {
				forEachReferencedBlock(meta, milestoneIndex, milestonePayload.Timestamp)
			}
		}); err != nil {
			return nil, nil, err
		}
	}

	timeLedgerUpdatedStart = time.Now()
	if onLedgerUpdated != nil {
		onLedgerUpdated(milestoneIndex, newOutputs, newSpents)
	}
	timeLedgerUpdatedEnd = time.Now()

	if onTreasuryMutated != nil && treasuryMutation != nil {
		timeTreasuryMutatedStart = time.Now()
		onTreasuryMutated(milestoneIndex, treasuryMutation)
		timeTreasuryMutatedEnd = time.Now()
	}

	return confirmedMilestoneStats, &ConfirmationMetrics{
		DurationWhiteflag:            timeWhiteflag.Sub(timeStart),
		DurationReceipts:             timeReceipts.Sub(timeWhiteflag),
		DurationConfirmation:         timeConfirmation.Sub(timeReceipts),
		DurationApplyConfirmation:    timeApplyConfirmation.Sub(timeConfirmation),
		DurationOnMilestoneConfirmed: timeOnMilestoneConfirmed.Sub(timeApplyConfirmation),
		DurationLedgerUpdated:        timeLedgerUpdatedEnd.Sub(timeLedgerUpdatedStart),
		DurationTreasuryMutated:      timeTreasuryMutatedEnd.Sub(timeTreasuryMutatedStart),
	}, nil
}
