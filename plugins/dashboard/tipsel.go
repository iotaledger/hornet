package dashboard

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/urts"
)

var tipSelMetricWorkerCount = 1
var tipSelMetricWorkerQueueSize = 100
var tipSelMetricWorkerPool *workerpool.WorkerPool

func configureTipSelMetric() {
	tipSelMetricWorkerPool = workerpool.New(func(task workerpool.Task) {
		switch x := task.Param(0).(type) {
		case *tipselect.TipSelStats:
			hub.BroadcastMsg(&msg{MsgTypeTipSelMetric, x})
		case milestone.Index:
			if msTailTxHash := getMilestoneTailHash(x); msTailTxHash != nil {
				hub.BroadcastMsg(&msg{MsgTypeMs, &ms{msTailTxHash.Trytes(), x}})
			}
		}
		task.Return(nil)
	}, workerpool.WorkerCount(tipSelMetricWorkerCount), workerpool.QueueSize(tipSelMetricWorkerQueueSize))
}

func runTipSelMetricWorker() {

	// check if URTS plugin is enabled
	if node.IsSkipped(urts.PLUGIN) {
		return
	}

	onTipSelPerformed := events.NewClosure(func(metrics *tipselect.TipSelStats) {
		tipSelMetricWorkerPool.TrySubmit(metrics)
	})

	daemon.BackgroundWorker("Dashboard[TipSelMetricUpdater]", func(shutdownSignal <-chan struct{}) {
		urts.TipSelector.Events.TipSelPerformed.Attach(onTipSelPerformed)
		tipSelMetricWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Dashboard[TipSelMetricUpdater] ...")
		urts.TipSelector.Events.TipSelPerformed.Detach(onTipSelPerformed)
		tipSelMetricWorkerPool.StopAndWait()
		log.Info("Stopping Dashboard[TipSelMetricUpdater] ... done")
	}, shutdown.PriorityDashboard)
}
