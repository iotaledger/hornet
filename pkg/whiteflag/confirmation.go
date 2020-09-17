package whiteflag

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

type ConfirmedMilestoneStats struct {
	Index               milestone.Index
	ConfirmationTime    int64
	Txs                 tangle.CachedMessages
	MessagesConfirmed   int
	MessagesConflicting int
	MessagesValue       int
	MessagesZeroValue   int
	Collecting          time.Duration
	Total               time.Duration
}

// ConfirmMilestone traverses a milestone and collects all unconfirmed tx,
// then the ledger diffs are calculated, the ledger state is checked and all tx are marked as confirmed.
// all cachedMsgMetas have to be released outside.
func ConfirmMilestone(cachedMessageMetas map[string]*tangle.CachedMetadata, cachedMessage *tangle.CachedMessage, forEachConfirmedMessage func(messageMetadata *tangle.CachedMetadata, index milestone.Index, confTime uint64), onMilestoneConfirmed func(confirmation *Confirmation)) (*ConfirmedMilestoneStats, error) {
	defer cachedMessage.Release(true)
	message := cachedMessage.GetMessage()

	cachedMessages := make(map[string]*tangle.CachedMessage)

	defer func() {
		// All releases are forced since the cone is confirmed and not needed anymore

		// release all bundles at the end
		for _, cachedMsg := range cachedMessages {
			cachedMsg.Release(true) // message -1
		}
	}()

	if _, exists := cachedMessages[string(cachedMessage.GetMessage().GetMessageID())]; !exists {
		// release the bundles at the end to speed up calculation
		cachedMessages[string(cachedMessage.GetMessage().GetMessageID())] = cachedMessage.Retain()
	}

	//tangle.WriteLockLedger()
	//defer tangle.WriteUnlockLedger()
	ms, err := tangle.CheckIfMilestone(message)
	if err != nil {
		return nil, err
	}

	milestoneIndex := milestone.Index(ms.Index)

	ts := time.Now()

	mutations, err := ComputeWhiteFlagMutations(cachedMessageMetas, cachedMessages, tangle.GetMilestoneMerkleHashFunc(), message.GetMessageID())
	if err != nil {
		// According to the RFC we should panic if we encounter any invalid bundles during confirmation
		return nil, fmt.Errorf("confirmMilestone: whiteflag.ComputeConfirmation failed with Error: %v", err)
	}

	confirmation := &Confirmation{
		MilestoneIndex: milestoneIndex,
		MilestoneHash:  message.GetMessageID(),
		Mutations:      mutations,
	}

	// Verify the calculated MerkleTreeHash with the one inside the milestone
	merkleTreeHash := ms.InclusionMerkleProof[:]
	if !bytes.Equal(mutations.MerkleTreeHash, merkleTreeHash) {
		return nil, fmt.Errorf("confirmMilestone: computed MerkleTreeHash %s does not match the value in the milestone %s", hex.EncodeToString(mutations.MerkleTreeHash), hex.EncodeToString(merkleTreeHash))
	}

	tc := time.Now()

	//err = tangle.ApplyLedgerDiffWithoutLocking(mutations.AddressMutations, milestoneIndex)
	//if err != nil {
	//	return nil, fmt.Errorf("confirmMilestone: ApplyLedgerDiff failed with Error: %v", err)
	//}

	//cachedMsTailTx := message.GetTail()
	//defer cachedMsTailTx.Release(true)

	loadMessageMetadata := func(messageID hornet.Hash) (*tangle.CachedMetadata, error) {
		cachedMsgMeta, exists := cachedMessageMetas[string(messageID)]
		if !exists {
			cachedMsgMeta = tangle.GetCachedMessageMetadataOrNil(messageID) // meta +1
			if cachedMsgMeta == nil {
				return nil, fmt.Errorf("confirmMilestone: Message not found: %v", messageID.Hex())
			}
			cachedMessageMetas[string(messageID)] = cachedMsgMeta
		}
		return cachedMsgMeta, nil
	}

	// load the message for the given id
	forMessageMetadataWithMessageID := func(messageID hornet.Hash, do func(meta *tangle.CachedMetadata)) error {
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
	for _, messageID := range mutations.MessagesIncluded {
		if err := forMessageMetadataWithMessageID(messageID, func(meta *tangle.CachedMetadata) {
			if !meta.GetMetadata().IsConfirmed() {
				meta.GetMetadata().SetConfirmed(true, milestoneIndex)
				meta.GetMetadata().SetRootSnapshotIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				conf.MessagesConfirmed++
				conf.MessagesValue++
				metrics.SharedServerMetrics.ValueTransactions.Inc()
				metrics.SharedServerMetrics.ConfirmedMessages.Inc()
				forEachConfirmedMessage(meta, milestoneIndex, confirmationTime)
			}
		}); err != nil {
			return nil, err
		}
	}

	// confirm all excluded messages with zero value
	for _, messageID := range mutations.MessagesExcludedZeroValue {
		if err := forMessageMetadataWithMessageID(messageID, func(meta *tangle.CachedMetadata) {
			if !meta.GetMetadata().IsConfirmed() {
				meta.GetMetadata().SetConfirmed(true, milestoneIndex)
				meta.GetMetadata().SetRootSnapshotIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				conf.MessagesConfirmed++
				conf.MessagesZeroValue++
				metrics.SharedServerMetrics.ZeroValueTransactions.Inc()
				metrics.SharedServerMetrics.ConfirmedMessages.Inc()
				forEachConfirmedMessage(meta, milestoneIndex, confirmationTime)
			}
		}); err != nil {
			return nil, err
		}
	}

	// confirm all conflicting messages
	for _, messageID := range mutations.MessagesExcludedConflicting {
		if err := forMessageMetadataWithMessageID(messageID, func(meta *tangle.CachedMetadata) {
			meta.GetMetadata().SetConflicting(true)
			if !meta.GetMetadata().IsConfirmed() {
				meta.GetMetadata().SetConfirmed(true, milestoneIndex)
				meta.GetMetadata().SetRootSnapshotIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				conf.MessagesConfirmed++
				conf.MessagesConflicting++
				metrics.SharedServerMetrics.ConflictingMessages.Inc()
				metrics.SharedServerMetrics.ConfirmedMessages.Inc()
				forEachConfirmedMessage(meta, milestoneIndex, confirmationTime)
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
