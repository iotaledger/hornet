package dashboard

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	tanglePackage "github.com/gohornet/hornet/pkg/model/tangle"
	tanglemodel "github.com/gohornet/hornet/pkg/model/tangle"
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
	ID          string `json:"id"`
	Tag         string `json:"tag"`
	TrunkID     string `json:"trunk_id"`
	BranchID    string `json:"branch_id"`
	IsSolid     bool   `json:"is_solid"`
	IsConfirmed bool   `json:"is_confirmed"`
	IsMilestone bool   `json:"is_milestone"`
	IsTip       bool   `json:"is_tip"`
}

// metainfo signals that metadata of a given transaction changed.
type metainfo struct {
	ID string `json:"id"`
}

// confirmationinfo signals confirmation of a milestone tail tx with a list of exluded txs in the past cone.
type confirmationinfo struct {
	ID          string   `json:"id"`
	ExcludedIDs []string `json:"excluded_ids"`
}

// tipinfo holds information about whether a given transaction is a tip or not.
type tipinfo struct {
	ID    string `json:"id"`
	IsTip bool   `json:"is_tip"`
}

func runVisualizer() {

	onReceivedNewTransaction := events.NewClosure(func(cachedTx *tanglemodel.CachedTransaction, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
		cachedTx.ConsumeTransactionAndMetadata(func(tx *hornet.Transaction, metadata *hornet.TransactionMetadata) { // tx -1
			if !tanglemodel.IsNodeSyncedWithThreshold() {
				return
			}

			hub.BroadcastMsg(
				&Msg{
					Type: MsgTypeVertex,
					Data: &vertex{
						ID:          tx.Tx.Hash,
						Tag:         tx.Tx.Tag,
						TrunkID:     tx.Tx.TrunkTransaction[:VisualizerIdLength],
						BranchID:    tx.Tx.BranchTransaction[:VisualizerIdLength],
						IsSolid:     metadata.IsSolid(),
						IsConfirmed: metadata.IsConfirmed(),
						IsMilestone: false,
						IsTip:       false,
					},
				},
			)
		})
	})

	onTransactionSolid := events.NewClosure(func(txHash hornet.Hash) {
		if !tanglemodel.IsNodeSyncedWithThreshold() {
			return
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeSolidInfo,
				Data: &metainfo{
					ID: txHash.Trytes()[:VisualizerIdLength],
				},
			},
		)
	})

	onReceivedNewMilestone := events.NewClosure(func(cachedBndl *tanglePackage.CachedBundle) {
		cachedBndl.ConsumeBundle(func(bndl *tanglePackage.Bundle) { // bundle -1
			if !tanglemodel.IsNodeSyncedWithThreshold() {
				return
			}

			for _, txHash := range bndl.GetTxHashes() {
				hub.BroadcastMsg(
					&Msg{
						Type: MsgTypeMilestoneInfo,
						Data: &metainfo{
							ID: txHash.Trytes()[:VisualizerIdLength],
						},
					},
				)
			}
		})
	})

	// show checkpoints as milestones in the coordinator node
	onIssuedCheckpointTransaction := events.NewClosure(func(checkpointIndex int, tipIndex int, tipsTotal int, txHash hornet.Hash) {
		if !tanglemodel.IsNodeSyncedWithThreshold() {
			return
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeMilestoneInfo,
				Data: &metainfo{
					ID: txHash.Trytes()[:VisualizerIdLength],
				},
			},
		)
	})

	onMilestoneConfirmed := events.NewClosure(func(confirmation *whiteflag.Confirmation) {
		if !tanglemodel.IsNodeSyncedWithThreshold() {
			return
		}

		var excludedIDs []string
		for _, txHash := range confirmation.Mutations.TailsExcludedConflicting {
			excludedIDs = append(excludedIDs, txHash.Trytes()[:VisualizerIdLength])
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeConfirmedInfo,
				Data: &confirmationinfo{
					ID:          confirmation.MilestoneHash.Trytes()[:VisualizerIdLength],
					ExcludedIDs: excludedIDs,
				},
			},
		)
	})

	onTipAdded := events.NewClosure(func(tip *tipselect.Tip) {
		if !tanglemodel.IsNodeSyncedWithThreshold() {
			return
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeTipInfo,
				Data: &tipinfo{
					ID:    tip.Hash.Trytes()[:VisualizerIdLength],
					IsTip: true,
				},
			},
		)
	})

	onTipRemoved := events.NewClosure(func(tip *tipselect.Tip) {
		if !tanglemodel.IsNodeSyncedWithThreshold() {
			return
		}

		hub.BroadcastMsg(
			&Msg{
				Type: MsgTypeTipInfo,
				Data: &tipinfo{
					ID:    tip.Hash.Trytes()[:VisualizerIdLength],
					IsTip: false,
				},
			},
		)
	})

	daemon.BackgroundWorker("Dashboard[Visualizer]", func(shutdownSignal <-chan struct{}) {
		tangle.Events.ReceivedNewTransaction.Attach(onReceivedNewTransaction)
		defer tangle.Events.ReceivedNewTransaction.Detach(onReceivedNewTransaction)
		tangle.Events.TransactionSolid.Attach(onTransactionSolid)
		defer tangle.Events.TransactionSolid.Detach(onTransactionSolid)
		tangle.Events.ReceivedNewMilestone.Attach(onReceivedNewMilestone)
		defer tangle.Events.ReceivedNewMilestone.Detach(onReceivedNewMilestone)
		if cooEvents := coordinatorPlugin.GetEvents(); cooEvents != nil {
			cooEvents.IssuedCheckpointTransaction.Attach(onIssuedCheckpointTransaction)
			defer cooEvents.IssuedCheckpointTransaction.Detach(onIssuedCheckpointTransaction)
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
