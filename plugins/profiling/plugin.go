package profiling

import (
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/configuration"
	"go.uber.org/dig"
)

var (
	Plugin *node.Plugin
	deps   dependencies
)

type dependencies struct {
	dig.In
	NodeConfig *configuration.Configuration `name:"nodeConfig"`
}

func init() {
	Plugin = node.NewPlugin("Profiling", node.Enabled, configure, run)
}

func configure(c *dig.Container) {
	if err := c.Invoke(func(cDeps dependencies) {
		deps = cDeps
	}); err != nil {
		panic(err)
	}
}

func run(_ *dig.Container) {
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	bindAddr := deps.NodeConfig.String(config.CfgProfilingBindAddress)
	go http.ListenAndServe(bindAddr, nil) // pprof Server for Debbuging Mutexes
}
