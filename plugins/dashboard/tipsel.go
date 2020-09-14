package dashboard

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/urts"
)

func runTipSelMetricWorker() {

	// check if URTS plugin is enabled
	if node.IsSkipped(urts.PLUGIN) {
		return
	}

	onTipSelPerformed := events.NewClosure(func(metrics *tipselect.TipSelStats) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeTipSelMetric, Data: metrics})
	})

	daemon.BackgroundWorker("Dashboard[TipSelMetricUpdater]", func(shutdownSignal <-chan struct{}) {
		urts.TipSelector.Events.TipSelPerformed.Attach(onTipSelPerformed)
		<-shutdownSignal
		log.Info("Stopping Dashboard[TipSelMetricUpdater] ...")
		urts.TipSelector.Events.TipSelPerformed.Detach(onTipSelPerformed)
		log.Info("Stopping Dashboard[TipSelMetricUpdater] ... done")
	}, shutdown.PriorityDashboard)
}
