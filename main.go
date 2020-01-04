package main

import (
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/plugins/cli"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/gracefulshutdown"
	"github.com/gohornet/hornet/plugins/graph"
	"github.com/gohornet/hornet/plugins/metrics"
	"github.com/gohornet/hornet/plugins/monitor"
	"github.com/gohornet/hornet/plugins/mqtt"
	"github.com/gohornet/hornet/plugins/snapshot"
	"github.com/gohornet/hornet/plugins/spa"
	"github.com/gohornet/hornet/plugins/spammer"
	"github.com/gohornet/hornet/plugins/tangle"
	"github.com/gohornet/hornet/plugins/tipselection"
	"github.com/gohornet/hornet/plugins/webapi"
	"github.com/gohornet/hornet/plugins/zeromq"
)

func main() {

	cli.PrintVersion()

	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	go http.ListenAndServe("localhost:6060", nil) // pprof Server for Debbuging Mutexes

	node.Run(
		node.Plugins(
			cli.PLUGIN,
			gracefulshutdown.PLUGIN,
			gossip.PLUGIN,
			tangle.PLUGIN,
			tipselection.PLUGIN,
			metrics.PLUGIN,
			snapshot.PLUGIN,
			webapi.PLUGIN,
			spa.PLUGIN,
			zeromq.PLUGIN,
			mqtt.PLUGIN,
			graph.PLUGIN,
			monitor.PLUGIN,
			spammer.PLUGIN,
		),
	)
}
