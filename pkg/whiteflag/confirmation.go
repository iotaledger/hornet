package whiteflag

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/model/utxo"
)

type ConfirmedMilestoneStats struct {
	Index                                       milestone.Index
	ConfirmationTime                            int64
	CachedMessages                              tangle.CachedMessages
	MessagesReferenced                          int
	MessagesExcludedWithConflictingTransactions int
	MessagesIncludedWithTransactions            int
	MessagesExcludedWithoutTransactions         int
	Collecting                                  time.Duration
	Total                                       time.Duration
}

// ConfirmMilestone traverses a milestone and collects all unreferenced msg,
// then the ledger diffs are calculated, the ledger state is checked and all msg are marked as referenced.
// all cachedMsgMetas have to be released outside.
func ConfirmMilestone(cachedMessageMetas map[string]*tangle.CachedMetadata, milestoneMessageID *hornet.MessageID, forEachReferencedMessage func(messageMetadata *tangle.CachedMetadata, index milestone.Index, confTime uint64), onMilestoneConfirmed func(confirmation *Confirmation)) (*ConfirmedMilestoneStats, error) {

	cachedMessages := make(map[string]*tangle.CachedMessage)

	defer func() {
		// All releases are forced since the cone is referenced and not needed anymore

		// release all messages at the end
		for _, cachedMsg := range cachedMessages {
			cachedMsg.Release(true) // message -1
		}
	}()

	cachedMilestoneMessage := tangle.GetCachedMessageOrNil(milestoneMessageID)
	if cachedMilestoneMessage == nil {
		return nil, fmt.Errorf("milestone message not found: %v", milestoneMessageID.Hex())
	}
	defer cachedMilestoneMessage.Release(true)

	cachedMilestoneMessageMapKey := cachedMilestoneMessage.GetMessage().GetMessageID().MapKey()
	if _, exists := cachedMessages[cachedMilestoneMessageMapKey]; !exists {
		// release the messages at the end to speed up calculation
		cachedMessages[cachedMilestoneMessageMapKey] = cachedMilestoneMessage.Retain()
	}

	utxo.WriteLockLedger()
	defer utxo.WriteUnlockLedger()
	message := cachedMilestoneMessage.GetMessage()

	ms, err := tangle.CheckIfMilestone(message)
	if err != nil {
		return nil, err
	}

	if ms == nil {
		return nil, fmt.Errorf("confirmMilestone: message does not contain a milestone payload: %v", message.GetMessageID().Hex())
	}

	milestoneIndex := milestone.Index(ms.Index)

	ts := time.Now()

	mutations, err := ComputeWhiteFlagMutations(milestoneIndex, cachedMessageMetas, cachedMessages, tangle.GetMilestoneMerkleHashFunc(), message.GetParent1MessageID(), message.GetParent2MessageID())
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

	var newOutputs utxo.Outputs
	for _, output := range mutations.NewOutputs {
		newOutputs = append(newOutputs, output)
	}

	var newSpents utxo.Spents
	for _, spent := range mutations.NewSpents {
		newSpents = append(newSpents, spent)
	}

	err = utxo.ApplyConfirmationWithoutLocking(milestoneIndex, newOutputs, newSpents)
	if err != nil {
		return nil, fmt.Errorf("confirmMilestone: utxo.ApplyConfirmation failed with Error: %v", err)
	}

	loadMessageMetadata := func(messageID *hornet.MessageID) (*tangle.CachedMetadata, error) {
		messageIDMapKey := messageID.MapKey()
		cachedMsgMeta, exists := cachedMessageMetas[messageIDMapKey]
		if !exists {
			cachedMsgMeta = tangle.GetCachedMessageMetadataOrNil(messageID) // meta +1
			if cachedMsgMeta == nil {
				return nil, fmt.Errorf("confirmMilestone: Message not found: %v", messageID.Hex())
			}
			cachedMessageMetas[messageIDMapKey] = cachedMsgMeta
		}
		return cachedMsgMeta, nil
	}

	// load the message for the given id
	forMessageMetadataWithMessageID := func(messageID *hornet.MessageID, do func(meta *tangle.CachedMetadata)) error {
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
		if err := forMessageMetadataWithMessageID(messageID, func(meta *tangle.CachedMetadata) {
			if !meta.GetMetadata().IsReferenced() {
				meta.GetMetadata().SetReferenced(true, milestoneIndex)
				meta.GetMetadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				conf.MessagesReferenced++
				conf.MessagesIncludedWithTransactions++
				metrics.SharedServerMetrics.IncludedTransactionMessages.Inc()
				metrics.SharedServerMetrics.ReferencedMessages.Inc()
				forEachReferencedMessage(meta, milestoneIndex, confirmationTime)
			}
		}); err != nil {
			return nil, err
		}
	}

	// confirm all excluded messages with zero value
	for _, messageID := range mutations.MessagesExcludedWithoutTransactions {
		if err := forMessageMetadataWithMessageID(messageID, func(meta *tangle.CachedMetadata) {
			meta.GetMetadata().SetIsNoTransaction(true)
			if !meta.GetMetadata().IsReferenced() {
				meta.GetMetadata().SetReferenced(true, milestoneIndex)
				meta.GetMetadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				conf.MessagesReferenced++
				conf.MessagesExcludedWithoutTransactions++
				metrics.SharedServerMetrics.NoTransactionMessages.Inc()
				metrics.SharedServerMetrics.ReferencedMessages.Inc()
				forEachReferencedMessage(meta, milestoneIndex, confirmationTime)
			}
		}); err != nil {
			return nil, err
		}
	}

	// confirm all conflicting messages
	for _, messageID := range mutations.MessagesExcludedWithConflictingTransactions {
		if err := forMessageMetadataWithMessageID(messageID, func(meta *tangle.CachedMetadata) {
			meta.GetMetadata().SetConflictingTx(true)
			if !meta.GetMetadata().IsReferenced() {
				meta.GetMetadata().SetReferenced(true, milestoneIndex)
				meta.GetMetadata().SetConeRootIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				conf.MessagesReferenced++
				conf.MessagesExcludedWithConflictingTransactions++
				metrics.SharedServerMetrics.ConflictingTransactionMessages.Inc()
				metrics.SharedServerMetrics.ReferencedMessages.Inc()
				forEachReferencedMessage(meta, milestoneIndex, confirmationTime)
			}
		}); err != nil {
			return nil, err
		}
	}

	onMilestoneConfirmed(confirmation)

	conf.Collecting = tc.Sub(ts)
	conf.Total = time.Since(ts)

	return conf, nil
}
