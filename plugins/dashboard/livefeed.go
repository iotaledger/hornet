package dashboard

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/events"
)

func runLiveFeed() {

	onLatestMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		if milestoneMessageID := getMilestoneMessageID(msIndex); milestoneMessageID != nil {
			hub.BroadcastMsg(&Msg{Type: MsgTypeMs, Data: &LivefeedMilestone{MessageID: milestoneMessageID.ToHex(), Index: msIndex}})
		}
	})

	Plugin.Daemon().BackgroundWorker("Dashboard[TxUpdater]", func(shutdownSignal <-chan struct{}) {
		deps.Tangle.Events.LatestMilestoneIndexChanged.Attach(onLatestMilestoneIndexChanged)
		defer deps.Tangle.Events.LatestMilestoneIndexChanged.Detach(onLatestMilestoneIndexChanged)

		<-shutdownSignal
		log.Info("Stopping Dashboard[TxUpdater] ...")
		log.Info("Stopping Dashboard[TxUpdater] ... done")
	}, shutdown.PriorityDashboard)
}
