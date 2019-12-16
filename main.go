package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/packages/node"
	"github.com/gohornet/hornet/plugins/cli"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/gracefulshutdown"
	"github.com/gohornet/hornet/plugins/metrics"
	"github.com/gohornet/hornet/plugins/monitor"
	"github.com/gohornet/hornet/plugins/snapshot"
	"github.com/gohornet/hornet/plugins/spa"
	"github.com/gohornet/hornet/plugins/spammer"
	"github.com/gohornet/hornet/plugins/tangle"
	"github.com/gohornet/hornet/plugins/tipselection"
	"github.com/gohornet/hornet/plugins/webapi"
	"github.com/gohornet/hornet/plugins/zeromq"
)

func main() {

	// Print out HORNET version
	version := flag.BoolP("version", "v", false, "Prints the HORNET version")
	flag.Parse()
	if *version {
		fmt.Println(cli.AppName + " " + cli.AppVersion)
		os.Exit(0)
	}

	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	go http.ListenAndServe("localhost:6060", nil) // pprof Server for Debbuging Mutexes

	node.Run(
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
		monitor.PLUGIN,
		spammer.PLUGIN,
	)
}
