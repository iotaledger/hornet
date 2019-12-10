package metrics

import (
	"time"

	daemon "github.com/iotaledger/hive.go/daemon/ordered"
	"github.com/gohornet/hornet/packages/node"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/packages/timeutil"
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
