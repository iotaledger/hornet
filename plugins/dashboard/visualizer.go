package dashboard

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	tanglepackage "github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/whiteflag"
	coordinatorPlugin "github.com/gohornet/hornet/plugins/coordinator"
	"github.com/gohornet/hornet/plugins/tangle"
	"github.com/gohornet/hornet/plugins/urts"
)

const (
	VisualizerIdLength = 7
)

// vertex defines a vertex in a DAG.
type vertex struct {
	ID               string `json:"id"`
	Parent1MessageID string `json:"parent1_id"`
	Parent2MessageID string `json:"parent2_2"`
	IsSolid          bool   `json:"is_solid"`
	IsReferenced     bool   `json:"is_referenced"`
	IsMilestone      bool   `json:"is_milestone"`
	IsTip            bool   `json:"is_tip"`
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

	onReceivedNewMessage := events.NewClosure(func(cachedMsg *tanglepackage.CachedMessage, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
		cachedMsg.ConsumeMessageAndMetadata(func(msg *tanglepackage.Message, metadata *tanglepackage.MessageMetadata) { // msg -1
			if !tanglepackage.IsNodeSyncedWithThreshold() {
				return
			}

			hub.BroadcastMsg(
				&Msg{
					Type: MsgTypeVertex,
					Data: &vertex{
						ID:               msg.GetMessageID().Hex(),
						Parent1MessageID: msg.GetParent1MessageID().Hex()[:VisualizerIdLength],
						Parent2MessageID: msg.GetParent2MessageID().Hex()[:VisualizerIdLength],
						IsSolid:          metadata.IsSolid(),
						IsReferenced:     metadata.IsReferenced(),
						IsMilestone:      false,
						IsTip:            false,
					},
				},
			)
		})
	})

	onMessageSolid := events.NewClosure(func(cachedMsgMeta *tanglepackage.CachedMetadata) {
		cachedMsgMeta.ConsumeMetadata(func(metadata *tanglepackage.MessageMetadata) { // metadata -1

			if !tanglepackage.IsNodeSyncedWithThreshold() {
				return
			}

			hub.BroadcastMsg(
				&Msg{
					Type: MsgTypeSolidInfo,
					Data: &metainfo{
						ID: cachedMsgMeta.GetMetadata().GetMessageID().Hex()[:VisualizerIdLength],
					},
				},
			)
		})
	})

	onReceivedNewMilestone := events.NewClosure(func(cachedMilestone *tanglepackage.CachedMilestone) {
		defer cachedMilestone.Release(true) // milestone -1

		if !tanglepackage.IsNodeSyncedWithThreshold() {
			return
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeMilestoneInfo,
				Data: &metainfo{
					ID: cachedMilestone.GetMilestone().MessageID.Hex()[:VisualizerIdLength],
				},
			},
		)
	})

	// show checkpoints as milestones in the coordinator node
	onIssuedCheckpointMessage := events.NewClosure(func(checkpointIndex int, tipIndex int, tipsTotal int, messageID *hornet.MessageID) {
		if !tanglepackage.IsNodeSyncedWithThreshold() {
			return
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeMilestoneInfo,
				Data: &metainfo{
					ID: messageID.Hex()[:VisualizerIdLength],
				},
			},
		)
	})

	onMilestoneConfirmed := events.NewClosure(func(confirmation *whiteflag.Confirmation) {
		if !tanglepackage.IsNodeSyncedWithThreshold() {
			return
		}

		var excludedIDs []string
		for _, messageID := range confirmation.Mutations.MessagesExcludedWithConflictingTransactions {
			excludedIDs = append(excludedIDs, messageID.Hex()[:VisualizerIdLength])
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeConfirmedInfo,
				Data: &confirmationinfo{
					ID:          confirmation.MilestoneMessageID.Hex()[:VisualizerIdLength],
					ExcludedIDs: excludedIDs,
				},
			},
		)
	})

	onTipAdded := events.NewClosure(func(tip *tipselect.Tip) {
		if !tanglepackage.IsNodeSyncedWithThreshold() {
			return
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeTipInfo,
				Data: &tipinfo{
					ID:    tip.MessageID.Hex()[:VisualizerIdLength],
					IsTip: true,
				},
			},
		)
	})

	onTipRemoved := events.NewClosure(func(tip *tipselect.Tip) {
		if !tanglepackage.IsNodeSyncedWithThreshold() {
			return
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeTipInfo,
				Data: &tipinfo{
					ID:    tip.MessageID.Hex()[:VisualizerIdLength],
					IsTip: false,
				},
			},
		)
	})

	daemon.BackgroundWorker("Dashboard[Visualizer]", func(shutdownSignal <-chan struct{}) {
		tangle.Events.ReceivedNewMessage.Attach(onReceivedNewMessage)
		defer tangle.Events.ReceivedNewMessage.Detach(onReceivedNewMessage)
		tangle.Events.MessageSolid.Attach(onMessageSolid)
		defer tangle.Events.MessageSolid.Detach(onMessageSolid)
		tangle.Events.ReceivedNewMilestone.Attach(onReceivedNewMilestone)
		defer tangle.Events.ReceivedNewMilestone.Detach(onReceivedNewMilestone)
		if cooEvents := coordinatorPlugin.GetEvents(); cooEvents != nil {
			cooEvents.IssuedCheckpointMessage.Attach(onIssuedCheckpointMessage)
			defer cooEvents.IssuedCheckpointMessage.Detach(onIssuedCheckpointMessage)
		}
		tangle.Events.MilestoneConfirmed.Attach(onMilestoneConfirmed)
		defer tangle.Events.MilestoneConfirmed.Detach(onMilestoneConfirmed)

		// check if URTS plugin is enabled
		if !node.IsSkipped(urts.PLUGIN) {
			urts.TipSelector.Events.TipAdded.Attach(onTipAdded)
			defer urts.TipSelector.Events.TipAdded.Detach(onTipAdded)
			urts.TipSelector.Events.TipRemoved.Attach(onTipRemoved)
			defer urts.TipSelector.Events.TipRemoved.Detach(onTipRemoved)
		}

		<-shutdownSignal

		log.Info("Stopping Dashboard[Visualizer] ...")
		log.Info("Stopping Dashboard[Visualizer] ... done")
	}, shutdown.PriorityDashboard)
}
