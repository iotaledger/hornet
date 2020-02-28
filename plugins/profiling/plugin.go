package profiling

import (
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/iotaledger/hive.go/node"
)

var (
	PLUGIN = node.NewPlugin("Profiling", node.Enabled, run)
)

func run(plugin *node.Plugin) {
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	go http.ListenAndServe("localhost:6060", nil) // pprof Server for Debbuging Mutexes
}
