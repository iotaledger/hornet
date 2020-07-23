package dashboard

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"

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
	VisualizerIdLength = 5
)

var (
	visualizerWorkerCount     = 1
	visualizerWorkerQueueSize = 500
	visualizerWorkerPool      *workerpool.WorkerPool
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

func configureVisualizer() {
	visualizerWorkerPool = workerpool.New(func(task workerpool.Task) {
		hub.BroadcastMsg(task.Param(0), task.Param(1).(bool))
		task.Return(nil)
	}, workerpool.WorkerCount(visualizerWorkerCount), workerpool.QueueSize(visualizerWorkerQueueSize))
}

func runVisualizer() {

	notifyNewVertex := events.NewClosure(func(cachedTx *tanglemodel.CachedTransaction, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
		cachedTx.ConsumeTransaction(func(tx *hornet.Transaction, metadata *hornet.TransactionMetadata) { // tx -1
			if !tanglemodel.IsNodeSyncedWithThreshold() {
				return
			}

			visualizerWorkerPool.TrySubmit(
				&msg{
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
				}, false)
		})
	})

	notifySolidInfo := events.NewClosure(func(cachedTx *tanglePackage.CachedTransaction) {
		cachedTx.ConsumeTransaction(func(tx *hornet.Transaction, metadata *hornet.TransactionMetadata) { // tx -1
			if !tanglemodel.IsNodeSyncedWithThreshold() {
				return
			}

			visualizerWorkerPool.TrySubmit(
				&msg{
					Type: MsgTypeSolidInfo,
					Data: &metainfo{
						ID: tx.Tx.Hash[:VisualizerIdLength],
					},
				}, false)
		})
	})

	notifyMilestoneInfo := events.NewClosure(func(cachedBndl *tanglePackage.CachedBundle) {
		cachedBndl.ConsumeBundle(func(bndl *tanglePackage.Bundle) { // bundle -1
			if !tanglemodel.IsNodeSyncedWithThreshold() {
				return
			}

			for _, txHash := range bndl.GetTxHashes() {
				visualizerWorkerPool.TrySubmit(
					&msg{
						Type: MsgTypeMilestoneInfo,
						Data: &metainfo{
							ID: txHash.Trytes()[:VisualizerIdLength],
						},
					}, false)
			}
		})
	})

	// show checkpoints as milestones in the coordinator node
	notifyIssuedCheckpointTransaction := events.NewClosure(func(checkpointIndex int, tipIndex int, tipsTotal int, txHash hornet.Hash) {
		if !tanglemodel.IsNodeSyncedWithThreshold() {
			return
		}

		visualizerWorkerPool.TrySubmit(
			&msg{
				Type: MsgTypeMilestoneInfo,
				Data: &metainfo{
					ID: txHash.Trytes()[:VisualizerIdLength],
				},
			}, false)
	})

	notifyMilestoneConfirmedInfo := events.NewClosure(func(confirmation *whiteflag.Confirmation) {
		if !tanglemodel.IsNodeSyncedWithThreshold() {
			return
		}

		var excludedIDs []string
		for _, txHash := range confirmation.TailsExcludedConflicting {
			excludedIDs = append(excludedIDs, txHash.Trytes()[:VisualizerIdLength])
		}

		visualizerWorkerPool.TrySubmit(
			&msg{
				Type: MsgTypeConfirmedInfo,
				Data: &confirmationinfo{
					ID:          confirmation.MilestoneHash.Trytes()[:VisualizerIdLength],
					ExcludedIDs: excludedIDs,
				},
			}, false)
	})

	notifyTipAdded := events.NewClosure(func(tip *tipselect.Tip) {
		if !tanglemodel.IsNodeSyncedWithThreshold() {
			return
		}

		visualizerWorkerPool.TrySubmit(
			&msg{
				Type: MsgTypeTipInfo,
				Data: &tipinfo{
					ID:    tip.Hash.Trytes()[:VisualizerIdLength],
					IsTip: true,
				},
			}, true)
	})

	notifyTipRemoved := events.NewClosure(func(tip *tipselect.Tip) {
		if !tanglemodel.IsNodeSyncedWithThreshold() {
			return
		}

		visualizerWorkerPool.TrySubmit(
			&msg{
				Type: MsgTypeTipInfo,
				Data: &tipinfo{
					ID:    tip.Hash.Trytes()[:VisualizerIdLength],
					IsTip: false,
				},
			}, true)
	})

	daemon.BackgroundWorker("Dashboard[Visualizer]", func(shutdownSignal <-chan struct{}) {
		tangle.Events.ReceivedNewTransaction.Attach(notifyNewVertex)
		defer tangle.Events.ReceivedNewTransaction.Detach(notifyNewVertex)
		tangle.Events.TransactionSolid.Attach(notifySolidInfo)
		defer tangle.Events.TransactionSolid.Detach(notifySolidInfo)
		tangle.Events.ReceivedNewMilestone.Attach(notifyMilestoneInfo)
		defer tangle.Events.ReceivedNewMilestone.Detach(notifyMilestoneInfo)
		if cooEvents := coordinatorPlugin.GetEvents(); cooEvents != nil {
			cooEvents.IssuedCheckpointTransaction.Attach(notifyIssuedCheckpointTransaction)
			defer cooEvents.IssuedCheckpointTransaction.Detach(notifyIssuedCheckpointTransaction)
		}
		tangle.Events.MilestoneConfirmed.Attach(notifyMilestoneConfirmedInfo)
		defer tangle.Events.MilestoneConfirmed.Detach(notifyMilestoneConfirmedInfo)

		// check if URTS plugin is enabled
		if !node.IsSkipped(urts.PLUGIN) {
			urts.TipSelector.Events.TipAdded.Attach(notifyTipAdded)
			defer urts.TipSelector.Events.TipAdded.Detach(notifyTipAdded)
			urts.TipSelector.Events.TipRemoved.Attach(notifyTipRemoved)
			defer urts.TipSelector.Events.TipRemoved.Detach(notifyTipRemoved)
		}

		visualizerWorkerPool.Start()
		<-shutdownSignal

		log.Info("Stopping Dashboard[Visualizer] ...")
		visualizerWorkerPool.StopAndWait()
		log.Info("Stopping Dashboard[Visualizer] ... done")
	}, shutdown.PriorityDashboard)
}
