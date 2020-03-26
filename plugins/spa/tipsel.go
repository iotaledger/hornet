package spa

import (
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"

	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/tipselection"
)

var (
	tipSelMetricWorkerPool = (&async.NonBlockingWorkerPool{}).Tune(1)
)

func runTipSelMetricWorker() {

	notifyTipSelPerformed := events.NewClosure(func(metrics *tipselection.TipSelStats) {
		tipSelMetricWorkerPool.Submit(func() {
			hub.BroadcastMsg(&msg{MsgTypeTipSelMetric, metrics})
		})
	})

	daemon.BackgroundWorker("SPA[TipSelMetricUpdater]", func(shutdownSignal <-chan struct{}) {
		tipselection.Events.TipSelPerformed.Attach(notifyTipSelPerformed)
		<-shutdownSignal
		log.Info("Stopping SPA[TipSelMetricUpdater] ...")
		tipselection.Events.TipSelPerformed.Detach(notifyTipSelPerformed)
		tipSelMetricWorkerPool.Shutdown()
		log.Info("Stopping SPA[TipSelMetricUpdater] ... done")
	}, shutdown.ShutdownPrioritySPA)
}
