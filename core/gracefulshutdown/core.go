package gracefulshutdown

import (
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/app"
)

func init() {
	CoreComponent = &app.CoreComponent{
		Component: &app.Component{
			Name:      "Graceful Shutdown",
			Provide:   provide,
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Configure: configure,
		},
	}
}

var (
	CoreComponent *app.CoreComponent
	deps          dependencies
)

type dependencies struct {
	dig.In
	ShutdownHandler *shutdown.ShutdownHandler
}

func provide(c *dig.Container) error {

	if err := c.Provide(func() *shutdown.ShutdownHandler {
		return shutdown.NewShutdownHandler(CoreComponent.Logger(), CoreComponent.Daemon())
	}); err != nil {
		CoreComponent.LogPanic(err)
	}

	return nil
}

func configure() error {
	deps.ShutdownHandler.Run()

	return nil
}
