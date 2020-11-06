package profiling

import (
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/configuration"
	"go.uber.org/dig"
)

func init() {
	Plugin = &node.Plugin{
		Name:     "Profiling",
		DepsFunc: func(cDeps dependencies) { deps = cDeps },
		Run:      run,
		Status:   node.Enabled,
	}
}

var (
	Plugin *node.Plugin
	deps   dependencies
)

type dependencies struct {
	dig.In
	NodeConfig *configuration.Configuration `name:"nodeConfig"`
}

func run() {
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	bindAddr := deps.NodeConfig.String(CfgProfilingBindAddress)
	go http.ListenAndServe(bindAddr, nil) // pprof Server for Debbuging Mutexes
}
