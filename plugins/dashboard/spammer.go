package dashboard

import (
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/spammer"
	spammerplugin "github.com/gohornet/hornet/plugins/spammer"
	"github.com/iotaledger/hive.go/events"
)

func runSpammerMetricWorker() {

	onSpamPerformed := events.NewClosure(func(metrics *spammer.SpamStats) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeSpamMetrics, Data: metrics})
	})

	onAvgSpamMetricsUpdated := events.NewClosure(func(metrics *spammer.AvgSpamMetrics) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeAvgSpamMetrics, Data: metrics})
	})

	if err := Plugin.Daemon().BackgroundWorker("Dashboard[SpammerMetricUpdater]", func(shutdownSignal <-chan struct{}) {
		spammerplugin.Events.SpamPerformed.Attach(onSpamPerformed)
		spammerplugin.Events.AvgSpamMetricsUpdated.Attach(onAvgSpamMetricsUpdated)
		<-shutdownSignal
		Plugin.LogInfo("Stopping Dashboard[SpammerMetricUpdater] ...")
		spammerplugin.Events.SpamPerformed.Detach(onSpamPerformed)
		spammerplugin.Events.AvgSpamMetricsUpdated.Detach(onAvgSpamMetricsUpdated)
		Plugin.LogInfo("Stopping Dashboard[SpammerMetricUpdater] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}
}
