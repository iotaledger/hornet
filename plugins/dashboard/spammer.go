package dashboard

import (
	"context"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hornet/pkg/shutdown"
	"github.com/iotaledger/hornet/pkg/spammer"
	spammerplugin "github.com/iotaledger/hornet/plugins/spammer"
)

func runSpammerMetricWorker() {

	onSpamPerformed := events.NewClosure(func(metrics *spammer.SpamStats) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeSpamMetrics, Data: metrics})
	})

	onAvgSpamMetricsUpdated := events.NewClosure(func(metrics *spammer.AvgSpamMetrics) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeAvgSpamMetrics, Data: metrics})
	})

	if err := Plugin.Daemon().BackgroundWorker("Dashboard[SpammerMetricUpdater]", func(ctx context.Context) {
		spammerplugin.Events.SpamPerformed.Hook(onSpamPerformed)
		spammerplugin.Events.AvgSpamMetricsUpdated.Hook(onAvgSpamMetricsUpdated)
		<-ctx.Done()
		Plugin.LogInfo("Stopping Dashboard[SpammerMetricUpdater] ...")
		spammerplugin.Events.SpamPerformed.Detach(onSpamPerformed)
		spammerplugin.Events.AvgSpamMetricsUpdated.Detach(onAvgSpamMetricsUpdated)
		Plugin.LogInfo("Stopping Dashboard[SpammerMetricUpdater] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
}
