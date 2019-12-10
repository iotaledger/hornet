package spa

import (
	daemon "github.com/iotaledger/hive.go/daemon/ordered"
	"github.com/iotaledger/hive.go/events"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/packages/workerpool"
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
			if tailTx := getMilestone(x); tailTx != nil {
				sendToAllWSClient(&msg{MsgTypeMs, &ms{tailTx.GetHash(), x}})
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
		tipselection.Events.TipSelPerformed.Detach(notifyTipSelPerformed)
		tipSelMetricWorkerPool.StopAndWait()
	}, shutdown.ShutdownPrioritySPA)
}
