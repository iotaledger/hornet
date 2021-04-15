package dashboard

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/whiteflag"
	coordinatorPlugin "github.com/gohornet/hornet/plugins/coordinator"
	"github.com/gohornet/hornet/plugins/urts"
	"github.com/iotaledger/hive.go/events"
)

const (
	VisualizerIdLength = 7
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

	onReceivedNewMessage := events.NewClosure(func(cachedMsg *storage.CachedMessage, latestMilestoneIndex milestone.Index, confirmedMilestoneIndex milestone.Index) {
		cachedMsg.ConsumeMessageAndMetadata(func(msg *storage.Message, metadata *storage.MessageMetadata) { // msg -1
			if !deps.Storage.IsNodeAlmostSynced() {
				return
			}

			parentsHex := make([]string, len(msg.GetParents()))
			for i, parent := range msg.GetParents() {
				parentsHex[i] = parent.ToHex()[:VisualizerIdLength]
			}

			hub.BroadcastMsg(
				&Msg{
					Type: MsgTypeVertex,
					Data: &vertex{
						ID:           msg.GetMessageID().ToHex(),
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
						ID: cachedMsgMeta.GetMetadata().GetMessageID().ToHex()[:VisualizerIdLength],
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
					ID: cachedMilestone.GetMilestone().MessageID.ToHex()[:VisualizerIdLength],
				},
			},
		)
	})

	// show checkpoints as milestones in the coordinator node
	onIssuedCheckpointMessage := events.NewClosure(func(checkpointIndex int, tipIndex int, tipsTotal int, messageID hornet.MessageID) {
		if !deps.Storage.IsNodeAlmostSynced() {
			return
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeMilestoneInfo,
				Data: &metainfo{
					ID: messageID.ToHex()[:VisualizerIdLength],
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
			excludedIDs[i] = messageID.MessageID.ToHex()[:VisualizerIdLength]
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeConfirmedInfo,
				Data: &confirmationinfo{
					ID:          confirmation.MilestoneMessageID.ToHex()[:VisualizerIdLength],
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
					ID:    tip.MessageID.ToHex()[:VisualizerIdLength],
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
					ID:    tip.MessageID.ToHex()[:VisualizerIdLength],
					IsTip: false,
				},
			},
		)
	})

	Plugin.Daemon().BackgroundWorker("Dashboard[Visualizer]", func(shutdownSignal <-chan struct{}) {
		deps.Tangle.Events.ReceivedNewMessage.Attach(onReceivedNewMessage)
		defer deps.Tangle.Events.ReceivedNewMessage.Detach(onReceivedNewMessage)
		deps.Tangle.Events.MessageSolid.Attach(onMessageSolid)
		defer deps.Tangle.Events.MessageSolid.Detach(onMessageSolid)
		deps.Tangle.Events.ReceivedNewMilestone.Attach(onReceivedNewMilestone)
		defer deps.Tangle.Events.ReceivedNewMilestone.Detach(onReceivedNewMilestone)
		if cooEvents := coordinatorPlugin.GetEvents(); cooEvents != nil {
			cooEvents.IssuedCheckpointMessage.Attach(onIssuedCheckpointMessage)
			defer cooEvents.IssuedCheckpointMessage.Detach(onIssuedCheckpointMessage)
		}
		deps.Tangle.Events.MilestoneConfirmed.Attach(onMilestoneConfirmed)
		defer deps.Tangle.Events.MilestoneConfirmed.Detach(onMilestoneConfirmed)

		// check if URTS plugin is enabled
		if !Plugin.Node.IsSkipped(urts.Plugin) {
			deps.TipSelector.Events.TipAdded.Attach(onTipAdded)
			defer deps.TipSelector.Events.TipAdded.Detach(onTipAdded)
			deps.TipSelector.Events.TipRemoved.Attach(onTipRemoved)
			defer deps.TipSelector.Events.TipRemoved.Detach(onTipRemoved)
		}

		<-shutdownSignal

		log.Info("Stopping Dashboard[Visualizer] ...")
		log.Info("Stopping Dashboard[Visualizer] ... done")
	}, shutdown.PriorityDashboard)
}
