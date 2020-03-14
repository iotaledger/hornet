package profiling

import (
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/gohornet/hornet/packages/config"
	"github.com/iotaledger/hive.go/node"
)

var (
	PLUGIN = node.NewPlugin("Profiling", node.Enabled, run)
)

func run(plugin *node.Plugin) {
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	bindAddr := config.NodeConfig.GetString(config.CfgProfilingBindAddress)
	go http.ListenAndServe(bindAddr, nil) // pprof Server for Debbuging Mutexes
}
