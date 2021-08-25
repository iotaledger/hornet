package gracefulshutdown

import (
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
)

func init() {
	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
			Name:      "Graceful Shutdown",
			Provide:   provide,
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Configure: configure,
		},
	}
}

var (
	CorePlugin *node.CorePlugin
	deps       dependencies
)

type dependencies struct {
	dig.In
	ShutdownHandler *shutdown.ShutdownHandler
}

func provide(c *dig.Container) {

	if err := c.Provide(func() *shutdown.ShutdownHandler {
		return shutdown.NewShutdownHandler(CorePlugin.Logger(), CorePlugin.Daemon())
	}); err != nil {
		CorePlugin.Panic(err)
	}
}

func configure() {
	deps.ShutdownHandler.Run()
}
