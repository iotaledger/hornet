package tangle

import (
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
)

type BPSMetrics struct {
	Incoming uint32 `json:"incoming"`
	New      uint32 `json:"new"`
	Outgoing uint32 `json:"outgoing"`
}

type Events struct {
	// block events
	// ReceivedNewBlock contains the block, the latestMilestoneIndex and the confirmedMilestoneIndex
	ReceivedNewBlock *event.Event3[*storage.CachedBlock, iotago.MilestoneIndex, iotago.MilestoneIndex]
	BlockSolid       *event.Event1[*storage.CachedMetadata]

	// milestone events
	LatestMilestoneChanged        *event.Event1[*storage.CachedMilestone]
	LatestMilestoneIndexChanged   *event.Event1[iotago.MilestoneIndex]
	MilestoneSolidificationFailed *event.Event1[iotago.MilestoneIndex]
	MilestoneTimeout              *event.Event

	// metrics
	BPSMetricsUpdated *event.Event1[*BPSMetrics]

	// Events related to milestone confirmation

	// Hint: Ledger is write locked
	ConfirmedMilestoneIndexChanged *event.Event1[iotago.MilestoneIndex]
	// Hint: Ledger is not locked
	ConfirmationMetricsUpdated *event.Event1[*whiteflag.ConfirmationMetrics] // used for prometheus metrics
	// Hint: Ledger is not locked
	ConfirmedMilestoneChanged *event.Event1[*storage.CachedMilestone]

	// BlockReferenced contains the metadata, the milestone index and the confirmation time.
	// Hint: Ledger is not locked
	BlockReferenced *event.Event3[*storage.CachedMetadata, iotago.MilestoneIndex, uint32]
	// Hint: Ledger is not locked
	ReferencedBlocksCountUpdated *event.Event2[iotago.MilestoneIndex, int]
	// Hint: Ledger is not locked
	LedgerUpdated *event.Event3[iotago.MilestoneIndex, utxo.Outputs, utxo.Spents]
	// Hint: Ledger is not locked
	TreasuryMutated *event.Event2[iotago.MilestoneIndex, *utxo.TreasuryMutationTuple]
	// Hint: Ledger is not locked
	NewReceipt *event.Event1[*iotago.ReceiptMilestoneOpt]
}

func newEvents() *Events {
	return &Events{
		BPSMetricsUpdated: event.New1[*BPSMetrics](),
		ReceivedNewBlock: event.New3[*storage.CachedBlock, iotago.MilestoneIndex, iotago.MilestoneIndex](event.WithPreTriggerFunc(func(block *storage.CachedBlock, _ iotago.MilestoneIndex, _ iotago.MilestoneIndex) {
			block.Retain() // block +1
		})),
		BlockSolid: event.New1[*storage.CachedMetadata](event.WithPreTriggerFunc(func(metadata *storage.CachedMetadata) {
			metadata.Retain() // meta pass +1
		})),
		BlockReferenced: event.New3[*storage.CachedMetadata, iotago.MilestoneIndex, uint32](event.WithPreTriggerFunc(func(metadata *storage.CachedMetadata, _ iotago.MilestoneIndex, _ uint32) {
			metadata.Retain() // meta pass +1
		})),
		LatestMilestoneChanged: event.New1[*storage.CachedMilestone](event.WithPreTriggerFunc(func(milestone *storage.CachedMilestone) {
			milestone.Retain() // milestone pass +1
		})),
		LatestMilestoneIndexChanged: event.New1[iotago.MilestoneIndex](),
		ConfirmedMilestoneChanged: event.New1[*storage.CachedMilestone](event.WithPreTriggerFunc(func(milestone *storage.CachedMilestone) {
			milestone.Retain() // milestone pass +1
		})),
		ConfirmedMilestoneIndexChanged: event.New1[iotago.MilestoneIndex](),
		ConfirmationMetricsUpdated:     event.New1[*whiteflag.ConfirmationMetrics](),
		ReferencedBlocksCountUpdated:   event.New2[iotago.MilestoneIndex, int](),
		MilestoneSolidificationFailed:  event.New1[iotago.MilestoneIndex](),
		MilestoneTimeout:               event.New(),
		LedgerUpdated:                  event.New3[iotago.MilestoneIndex, utxo.Outputs, utxo.Spents](),
		TreasuryMutated:                event.New2[iotago.MilestoneIndex, *utxo.TreasuryMutationTuple](),
		NewReceipt:                     event.New1[*iotago.ReceiptMilestoneOpt](),
	}
}
