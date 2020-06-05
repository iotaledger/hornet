package profiling

import (
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/iotaledger/hive.go/node"
)

var (
	PLUGIN = node.NewPlugin("Profiling", node.Enabled, configure, run)
)

func configure(_ *node.Plugin) {
	// nothing
}

func run(_ *node.Plugin) {
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	bindAddr := config.NodeConfig.GetString(config.CfgProfilingBindAddress)
	go http.ListenAndServe(bindAddr, nil) // pprof Server for Debbuging Mutexes
}
