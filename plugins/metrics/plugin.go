package metrics

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/packages/shutdown"
)

var PLUGIN = node.NewPlugin("Metrics", node.Enabled, configure, run)

func configure(plugin *node.Plugin) {
	// nothing
}

func run(plugin *node.Plugin) {
	// create a background worker that "measures" the TPS value every second
	daemon.BackgroundWorker("Metrics TPS Updater", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(measureTPS, 1*time.Second, shutdownSignal)
	}, shutdown.ShutdownPriorityMetricsUpdater)
}
