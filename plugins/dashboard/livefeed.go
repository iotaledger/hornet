package dashboard

import (
	"context"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/shutdown"
)

func runLiveFeed() {

	onLatestMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		if milestoneMessageID := getMilestoneMessageID(msIndex); milestoneMessageID != nil {
			hub.BroadcastMsg(&Msg{Type: MsgTypeMs, Data: &LivefeedMilestone{MessageID: milestoneMessageID.ToHex(), Index: msIndex}})
		}
	})

	if err := Plugin.Daemon().BackgroundWorker("Dashboard[TxUpdater]", func(ctx context.Context) {
		deps.Tangle.Events.LatestMilestoneIndexChanged.Attach(onLatestMilestoneIndexChanged)
		defer deps.Tangle.Events.LatestMilestoneIndexChanged.Detach(onLatestMilestoneIndexChanged)

		<-ctx.Done()
		Plugin.LogInfo("Stopping Dashboard[TxUpdater] ...")
		Plugin.LogInfo("Stopping Dashboard[TxUpdater] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
}
