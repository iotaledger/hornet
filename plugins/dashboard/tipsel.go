package dashboard

import (
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/urts"
	"github.com/iotaledger/hive.go/events"
)

func runTipSelMetricWorker() {

	// check if URTS plugin is enabled
	if Plugin.Node.IsSkipped(urts.Plugin) {
		return
	}

	onTipSelPerformed := events.NewClosure(func(metrics *tipselect.TipSelStats) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeTipSelMetric, Data: metrics})
	})

	if err := Plugin.Daemon().BackgroundWorker("Dashboard[TipSelMetricUpdater]", func(shutdownSignal <-chan struct{}) {
		deps.TipSelector.Events.TipSelPerformed.Attach(onTipSelPerformed)
		<-shutdownSignal
		Plugin.LogInfo("Stopping Dashboard[TipSelMetricUpdater] ...")
		deps.TipSelector.Events.TipSelPerformed.Detach(onTipSelPerformed)
		Plugin.LogInfo("Stopping Dashboard[TipSelMetricUpdater] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}
}
