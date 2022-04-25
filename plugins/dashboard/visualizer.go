package dashboard

import (
	"context"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/events"
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

// metainfo signals that metadata of a given message changed.
type metainfo struct {
	ID string `json:"id"`
}

// confirmationinfo signals confirmation of a milestone msg with a list of exluded msgs in the past cone.
type confirmationinfo struct {
	IDs         []string `json:"ids"`
	ExcludedIDs []string `json:"excluded_ids"`
}

// tipinfo holds information about whether a given message is a tip or not.
type tipinfo struct {
	ID    string `json:"id"`
	IsTip bool   `json:"is_tip"`
}

func runVisualizer() {

	onReceivedNewMessage := events.NewClosure(func(cachedMsg *storage.CachedMessage, _ milestone.Index, _ milestone.Index) {
		cachedMsg.ConsumeMessageAndMetadata(func(msg *storage.Message, metadata *storage.MessageMetadata) { // message -1
			if !deps.SyncManager.IsNodeAlmostSynced() {
				return
			}

			parentsHex := make([]string, len(msg.Parents()))
			for i, parent := range msg.Parents() {
				parentsHex[i] = parent.ToHex()[:VisualizerIDLength]
			}

			hub.BroadcastMsg(
				&Msg{
					Type: MsgTypeVertex,
					Data: &vertex{
						ID:           msg.MessageID().ToHex(),
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

	onMessageSolid := events.NewClosure(func(cachedMsgMeta *storage.CachedMetadata) {
		cachedMsgMeta.ConsumeMetadata(func(metadata *storage.MessageMetadata) { // meta -1

			if !deps.SyncManager.IsNodeAlmostSynced() {
				return
			}

			hub.BroadcastMsg(
				&Msg{
					Type: MsgTypeSolidInfo,
					Data: &metainfo{
						ID: metadata.MessageID().ToHex()[:VisualizerIDLength],
					},
				},
			)
		})
	})

	onReceivedNewMilestoneMessage := events.NewClosure(func(messageID hornet.MessageID) {
		if !deps.SyncManager.IsNodeAlmostSynced() {
			return
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeMilestoneInfo,
				Data: &metainfo{
					ID: messageID.ToHex()[:VisualizerIDLength],
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

		excludedIDs := make([]string, len(confirmation.Mutations.MessagesExcludedWithConflictingTransactions))
		for i, messageID := range confirmation.Mutations.MessagesExcludedWithConflictingTransactions {
			excludedIDs[i] = messageID.MessageID.ToHex()[:VisualizerIDLength]
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
					ID:    tip.MessageID.ToHex()[:VisualizerIDLength],
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
					ID:    tip.MessageID.ToHex()[:VisualizerIDLength],
					IsTip: false,
				},
			},
		)
	})

	if err := Plugin.Daemon().BackgroundWorker("Dashboard[Visualizer]", func(ctx context.Context) {
		deps.Tangle.Events.ReceivedNewMessage.Attach(onReceivedNewMessage)
		defer deps.Tangle.Events.ReceivedNewMessage.Detach(onReceivedNewMessage)
		deps.Tangle.Events.MessageSolid.Attach(onMessageSolid)
		defer deps.Tangle.Events.MessageSolid.Detach(onMessageSolid)
		deps.Tangle.Events.ReceivedNewMilestoneMessage.Attach(onReceivedNewMilestoneMessage)
		defer deps.Tangle.Events.ReceivedNewMilestoneMessage.Detach(onReceivedNewMilestoneMessage)
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
	}, shutdown.PriorityDashboard); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
}
