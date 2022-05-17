package dashboard

import (
	"context"

	"github.com/gohornet/hornet/pkg/daemon"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/events"
)

func runMilestoneLiveFeed() {

	onLatestMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		if milestoneIDHex, err := getMilestoneIDHex(msIndex); err == nil {
			hub.BroadcastMsg(&Msg{Type: MsgTypeMilestone, Data: &LivefeedMilestone{MilestoneID: milestoneIDHex, Index: msIndex}})
		}
	})

	if err := Plugin.Daemon().BackgroundWorker("Dashboard[TxUpdater]", func(ctx context.Context) {
		deps.Tangle.Events.LatestMilestoneIndexChanged.Attach(onLatestMilestoneIndexChanged)
		defer deps.Tangle.Events.LatestMilestoneIndexChanged.Detach(onLatestMilestoneIndexChanged)

		<-ctx.Done()
		Plugin.LogInfo("Stopping Dashboard[TxUpdater] ...")
		Plugin.LogInfo("Stopping Dashboard[TxUpdater] ... done")
	}, daemon.PriorityDashboard); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
}
