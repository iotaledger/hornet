package spa

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/tipselection"
)

var tipSelMetricWorkerCount = 1
var tipSelMetricWorkerQueueSize = 100
var tipSelMetricWorkerPool *workerpool.WorkerPool

func configureTipSelMetric() {
	tipSelMetricWorkerPool = workerpool.New(func(task workerpool.Task) {
		switch x := task.Param(0).(type) {
		case *tipselection.TipSelStats:
			sendToAllWSClient(&msg{MsgTypeTipSelMetric, x})
		case milestone_index.MilestoneIndex:
			if cachedMsTailTx := getMilestoneTail(x); cachedMsTailTx != nil { // tx +1
				sendToAllWSClient(&msg{MsgTypeMs, &ms{cachedMsTailTx.GetTransaction().GetHash(), x}})
				cachedMsTailTx.Release() // tx -1
			}
		}
		task.Return(nil)
	}, workerpool.WorkerCount(tipSelMetricWorkerCount), workerpool.QueueSize(tipSelMetricWorkerQueueSize))
}

func runTipSelMetricWorker() {

	notifyTipSelPerformed := events.NewClosure(func(metrics *tipselection.TipSelStats) {
		tipSelMetricWorkerPool.TrySubmit(metrics)
	})

	daemon.BackgroundWorker("SPA[TipSelMetricUpdater]", func(shutdownSignal <-chan struct{}) {
		tipselection.Events.TipSelPerformed.Attach(notifyTipSelPerformed)
		tipSelMetricWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping SPA[TipSelMetricUpdater] ...")
		tipselection.Events.TipSelPerformed.Detach(notifyTipSelPerformed)
		tipSelMetricWorkerPool.StopAndWait()
		log.Info("Stopping SPA[TipSelMetricUpdater] ... done")
	}, shutdown.ShutdownPrioritySPA)
}
