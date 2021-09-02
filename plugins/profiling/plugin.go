package profiling

import (
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/pkg/errors"

	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/configuration"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusEnabled,
		Pluggable: node.Pluggable{
			Name:     "Profiling",
			DepsFunc: func(cDeps dependencies) { deps = cDeps },
			Params:   params,
			Run:      run,
		},
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

	go func() {
		Plugin.LogInfof("You can now access the profiling server using: http://%s/debug/pprof/", bindAddr)

		// pprof Server for Debugging
		if err := http.ListenAndServe(bindAddr, nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
			Plugin.LogWarnf("Stopped profiling server due to an error (%s)", err)
		}
	}()
}
