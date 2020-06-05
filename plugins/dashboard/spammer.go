package dashboard

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/pkg/shutdown"
	spammerPlugin "github.com/gohornet/hornet/plugins/spammer"
)

var (
	spammerMetricWorkerCount     = 1
	spammerMetricWorkerQueueSize = 100
	spammerMetricWorkerPool      *workerpool.WorkerPool
)

func configureSpammerMetric() {
	spammerMetricWorkerPool = workerpool.New(func(task workerpool.Task) {
		hub.BroadcastMsg(task.Param(0))
		task.Return(nil)
	}, workerpool.WorkerCount(spammerMetricWorkerCount), workerpool.QueueSize(spammerMetricWorkerQueueSize))
}

func runSpammerMetricWorker() {

	notifySpamPerformed := events.NewClosure(func(metrics *spammerPlugin.SpamStats) {
		spammerMetricWorkerPool.TrySubmit(&msg{Type: MsgTypeSpamMetrics, Data: metrics})
	})

	notifyAvgSpamMetricsUpdated := events.NewClosure(func(metrics *spammerPlugin.AvgSpamMetrics) {
		spammerMetricWorkerPool.TrySubmit(&msg{Type: MsgTypeAvgSpamMetrics, Data: metrics})
	})

	daemon.BackgroundWorker("Dashboard[SpammerMetricUpdater]", func(shutdownSignal <-chan struct{}) {
		spammerPlugin.Events.SpamPerformed.Attach(notifySpamPerformed)
		spammerPlugin.Events.AvgSpamMetricsUpdated.Attach(notifyAvgSpamMetricsUpdated)
		spammerMetricWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Dashboard[SpammerMetricUpdater] ...")
		spammerPlugin.Events.SpamPerformed.Detach(notifySpamPerformed)
		spammerPlugin.Events.AvgSpamMetricsUpdated.Detach(notifyAvgSpamMetricsUpdated)
		spammerMetricWorkerPool.StopAndWait()
		log.Info("Stopping Dashboard[SpammerMetricUpdater] ... done")
	}, shutdown.PriorityDashboard)
}
