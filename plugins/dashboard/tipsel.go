package dashboard

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tipselection"
	"github.com/gohornet/hornet/pkg/shutdown"
	tipselectionPlugin "github.com/gohornet/hornet/plugins/tipselection"
)

var tipSelMetricWorkerCount = 1
var tipSelMetricWorkerQueueSize = 100
var tipSelMetricWorkerPool *workerpool.WorkerPool

func configureTipSelMetric() {
	tipSelMetricWorkerPool = workerpool.New(func(task workerpool.Task) {
		switch x := task.Param(0).(type) {
		case *tipselection.TipSelStats:
			hub.BroadcastMsg(&msg{MsgTypeTipSelMetric, x})
		case milestone.Index:
			if cachedMsTailTx := getMilestoneTail(x); cachedMsTailTx != nil { // tx +1
				hub.BroadcastMsg(&msg{MsgTypeMs, &ms{cachedMsTailTx.GetTransaction().Tx.Hash, x}})
				cachedMsTailTx.Release(true) // tx -1
			}
		}
		task.Return(nil)
	}, workerpool.WorkerCount(tipSelMetricWorkerCount), workerpool.QueueSize(tipSelMetricWorkerQueueSize))
}

func runTipSelMetricWorker() {

	notifyTipSelPerformed := events.NewClosure(func(metrics *tipselection.TipSelStats) {
		tipSelMetricWorkerPool.TrySubmit(metrics)
	})

	daemon.BackgroundWorker("Dashboard[TipSelMetricUpdater]", func(shutdownSignal <-chan struct{}) {
		tipselectionPlugin.Events.TipSelPerformed.Attach(notifyTipSelPerformed)
		tipSelMetricWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Dashboard[TipSelMetricUpdater] ...")
		tipselectionPlugin.Events.TipSelPerformed.Detach(notifyTipSelPerformed)
		tipSelMetricWorkerPool.StopAndWait()
		log.Info("Stopping Dashboard[TipSelMetricUpdater] ... done")
	}, shutdown.PriorityDashboard)
}
