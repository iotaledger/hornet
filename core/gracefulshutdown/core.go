package gracefulshutdown

import (
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/app"
)

func init() {
	CorePlugin = &app.CoreComponent{
		Component: &app.Component{
			Name:      "Graceful Shutdown",
			Provide:   provide,
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Configure: configure,
		},
	}
}

var (
	CorePlugin *app.CoreComponent
	deps       dependencies
)

type dependencies struct {
	dig.In
	ShutdownHandler *shutdown.ShutdownHandler
}

func provide(c *dig.Container) error {

	if err := c.Provide(func() *shutdown.ShutdownHandler {
		return shutdown.NewShutdownHandler(CorePlugin.Logger(), CorePlugin.Daemon())
	}); err != nil {
		CorePlugin.LogPanic(err)
	}

	return nil
}

func configure() error {
	deps.ShutdownHandler.Run()

	return nil
}
