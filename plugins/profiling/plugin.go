package profiling

import (
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/pkg/errors"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/configuration"
)

func init() {
	Plugin = &app.Plugin{
		Status: app.StatusEnabled,
		Component: &app.Component{
			Name:     "Profiling",
			DepsFunc: func(cDeps dependencies) { deps = cDeps },
			Params:   params,
			Run:      run,
		},
	}
}

var (
	Plugin *app.Plugin
	deps   dependencies
)

type dependencies struct {
	dig.In
	AppConfig *configuration.Configuration `name:"appConfig"`
}

func run() error {
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	bindAddr := deps.AppConfig.String(CfgProfilingBindAddress)

	go func() {
		Plugin.LogInfof("You can now access the profiling server using: http://%s/debug/pprof/", bindAddr)

		// pprof Server for Debugging
		if err := http.ListenAndServe(bindAddr, nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
			Plugin.LogWarnf("Stopped profiling server due to an error (%s)", err)
		}
	}()

	return nil
}
