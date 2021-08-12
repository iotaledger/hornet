package dashboard

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/whiteflag"
	coordinatorPlugin "github.com/gohornet/hornet/plugins/coordinator"
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
	ID          string   `json:"id"`
	ExcludedIDs []string `json:"excluded_ids"`
}

// tipinfo holds information about whether a given message is a tip or not.
type tipinfo struct {
	ID    string `json:"id"`
	IsTip bool   `json:"is_tip"`
}

func runVisualizer() {

	onReceivedNewMessage := events.NewClosure(func(cachedMsg *storage.CachedMessage, _ milestone.Index, _ milestone.Index) {
		cachedMsg.ConsumeMessageAndMetadata(func(msg *storage.Message, metadata *storage.MessageMetadata) { // msg -1
			if !deps.Storage.IsNodeAlmostSynced() {
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
		cachedMsgMeta.ConsumeMetadata(func(metadata *storage.MessageMetadata) { // metadata -1

			if !deps.Storage.IsNodeAlmostSynced() {
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

	onReceivedNewMilestone := events.NewClosure(func(cachedMilestone *storage.CachedMilestone) {
		defer cachedMilestone.Release(true) // milestone -1

		if !deps.Storage.IsNodeAlmostSynced() {
			return
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeMilestoneInfo,
				Data: &metainfo{
					ID: cachedMilestone.Milestone().MessageID.ToHex()[:VisualizerIDLength],
				},
			},
		)
	})

	// show checkpoints as milestones in the coordinator node
	onIssuedCheckpointMessage := events.NewClosure(func(_ int, _ int, _ int, messageID hornet.MessageID) {
		if !deps.Storage.IsNodeAlmostSynced() {
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
		if !deps.Storage.IsNodeAlmostSynced() {
			return
		}

		excludedIDs := make([]string, len(confirmation.Mutations.MessagesExcludedWithConflictingTransactions))
		for i, messageID := range confirmation.Mutations.MessagesExcludedWithConflictingTransactions {
			excludedIDs[i] = messageID.MessageID.ToHex()[:VisualizerIDLength]
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeConfirmedInfo,
				Data: &confirmationinfo{
					ID:          confirmation.MilestoneMessageID.ToHex()[:VisualizerIDLength],
					ExcludedIDs: excludedIDs,
				},
			},
		)
	})

	onTipAdded := events.NewClosure(func(tip *tipselect.Tip) {
		if !deps.Storage.IsNodeAlmostSynced() {
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
		if !deps.Storage.IsNodeAlmostSynced() {
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

	if err := Plugin.Daemon().BackgroundWorker("Dashboard[Visualizer]", func(shutdownSignal <-chan struct{}) {
		deps.Tangle.Events.ReceivedNewMessage.Attach(onReceivedNewMessage)
		defer deps.Tangle.Events.ReceivedNewMessage.Detach(onReceivedNewMessage)
		deps.Tangle.Events.MessageSolid.Attach(onMessageSolid)
		defer deps.Tangle.Events.MessageSolid.Detach(onMessageSolid)
		deps.Tangle.Events.ReceivedNewMilestone.Attach(onReceivedNewMilestone)
		defer deps.Tangle.Events.ReceivedNewMilestone.Detach(onReceivedNewMilestone)
		if cooEvents := coordinatorPlugin.Events(); cooEvents != nil {
			cooEvents.IssuedCheckpointMessage.Attach(onIssuedCheckpointMessage)
			defer cooEvents.IssuedCheckpointMessage.Detach(onIssuedCheckpointMessage)
		}
		deps.Tangle.Events.MilestoneConfirmed.Attach(onMilestoneConfirmed)
		defer deps.Tangle.Events.MilestoneConfirmed.Detach(onMilestoneConfirmed)

		if deps.TipSelector != nil {
			deps.TipSelector.Events.TipAdded.Attach(onTipAdded)
			defer deps.TipSelector.Events.TipAdded.Detach(onTipAdded)
			deps.TipSelector.Events.TipRemoved.Attach(onTipRemoved)
			defer deps.TipSelector.Events.TipRemoved.Detach(onTipRemoved)
		}

		<-shutdownSignal

		Plugin.LogInfo("Stopping Dashboard[Visualizer] ...")
		Plugin.LogInfo("Stopping Dashboard[Visualizer] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}
}
