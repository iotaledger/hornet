package dashboard

import (
	"context"

	"github.com/gohornet/hornet/pkg/daemon"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/events"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	VisualizerIDLength = 7
)

// vertex defines a vertex in a DAG.
type vertex struct {
	ID           string   `json:"id"`
	Parents      []string `json:"parents"`
	IsSolid      bool     `json:"is_solid"`
	IsReferenced bool     `json:"is_referenced"`
	IsMilestone  bool     `json:"is_milestone"`
	IsTip        bool     `json:"is_tip"`
}

// metainfo signals that metadata of a given block changed.
type metainfo struct {
	ID string `json:"id"`
}

// confirmationinfo signals confirmation of a milestone block with a list of exluded blocks in the past cone.
type confirmationinfo struct {
	IDs         []string `json:"ids"`
	ExcludedIDs []string `json:"excluded_ids"`
}

// tipinfo holds information about whether a given block is a tip or not.
type tipinfo struct {
	ID    string `json:"id"`
	IsTip bool   `json:"is_tip"`
}

func runVisualizerFeed() {

	onReceivedNewBlock := events.NewClosure(func(cachedBlock *storage.CachedBlock, _ milestone.Index, _ milestone.Index) {
		cachedBlock.ConsumeBlockAndMetadata(func(block *storage.Block, metadata *storage.BlockMetadata) { // block -1
			if !deps.SyncManager.IsNodeAlmostSynced() {
				return
			}

			parentsHex := make([]string, len(block.Parents()))
			for i, parent := range block.Parents() {
				parentsHex[i] = parent.ToHex()[:VisualizerIDLength]
			}
			hub.BroadcastMsg(
				&Msg{
					Type: MsgTypeVertex,
					Data: &vertex{
						ID:           block.BlockID().ToHex(),
						Parents:      parentsHex,
						IsSolid:      metadata.IsSolid(),
						IsReferenced: metadata.IsReferenced(),
						IsMilestone:  false,
						IsTip:        false,
					},
				},
			)
		})
	})

	onBlockSolid := events.NewClosure(func(cachedBlockMeta *storage.CachedMetadata) {
		cachedBlockMeta.ConsumeMetadata(func(metadata *storage.BlockMetadata) { // meta -1

			if !deps.SyncManager.IsNodeAlmostSynced() {
				return
			}
			hub.BroadcastMsg(
				&Msg{
					Type: MsgTypeSolidInfo,
					Data: &metainfo{
						ID: metadata.BlockID().ToHex()[:VisualizerIDLength],
					},
				},
			)
		})
	})

	onReceivedNewMilestoneBlock := events.NewClosure(func(blockID iotago.BlockID) {
		if !deps.SyncManager.IsNodeAlmostSynced() {
			return
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeMilestoneInfo,
				Data: &metainfo{
					ID: blockID.ToHex()[:VisualizerIDLength],
				},
			},
		)
	})

	onMilestoneConfirmed := events.NewClosure(func(confirmation *whiteflag.Confirmation) {
		if !deps.SyncManager.IsNodeAlmostSynced() {
			return
		}

		milestoneParents := make([]string, len(confirmation.MilestoneParents))
		for i, parent := range confirmation.MilestoneParents {
			milestoneParents[i] = parent.ToHex()[:VisualizerIDLength]
		}

		excludedIDs := make([]string, len(confirmation.Mutations.BlocksExcludedWithConflictingTransactions))
		for i, blockID := range confirmation.Mutations.BlocksExcludedWithConflictingTransactions {
			excludedIDs[i] = blockID.BlockID.ToHex()[:VisualizerIDLength]
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeConfirmedInfo,
				Data: &confirmationinfo{
					IDs:         milestoneParents,
					ExcludedIDs: excludedIDs,
				},
			},
		)
	})

	onTipAdded := events.NewClosure(func(tip *tipselect.Tip) {
		if !deps.SyncManager.IsNodeAlmostSynced() {
			return
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeTipInfo,
				Data: &tipinfo{
					ID:    tip.BlockID.ToHex()[:VisualizerIDLength],
					IsTip: true,
				},
			},
		)
	})

	onTipRemoved := events.NewClosure(func(tip *tipselect.Tip) {
		if !deps.SyncManager.IsNodeAlmostSynced() {
			return
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeTipInfo,
				Data: &tipinfo{
					ID:    tip.BlockID.ToHex()[:VisualizerIDLength],
					IsTip: false,
				},
			},
		)
	})

	if err := Plugin.Daemon().BackgroundWorker("Dashboard[Visualizer]", func(ctx context.Context) {
		deps.Tangle.Events.ReceivedNewBlock.Attach(onReceivedNewBlock)
		defer deps.Tangle.Events.ReceivedNewBlock.Detach(onReceivedNewBlock)
		deps.Tangle.Events.BlockSolid.Attach(onBlockSolid)
		defer deps.Tangle.Events.BlockSolid.Detach(onBlockSolid)
		deps.Tangle.Events.ReceivedNewMilestoneBlock.Attach(onReceivedNewMilestoneBlock)
		defer deps.Tangle.Events.ReceivedNewMilestoneBlock.Detach(onReceivedNewMilestoneBlock)
		deps.Tangle.Events.MilestoneConfirmed.Attach(onMilestoneConfirmed)
		defer deps.Tangle.Events.MilestoneConfirmed.Detach(onMilestoneConfirmed)

		if deps.TipSelector != nil {
			deps.TipSelector.Events.TipAdded.Attach(onTipAdded)
			defer deps.TipSelector.Events.TipAdded.Detach(onTipAdded)
			deps.TipSelector.Events.TipRemoved.Attach(onTipRemoved)
			defer deps.TipSelector.Events.TipRemoved.Detach(onTipRemoved)
		}

		<-ctx.Done()

		Plugin.LogInfo("Stopping Dashboard[Visualizer] ...")
		Plugin.LogInfo("Stopping Dashboard[Visualizer] ... done")
	}, daemon.PriorityDashboard); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
}
