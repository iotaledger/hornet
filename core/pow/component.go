package pow

import (
	"context"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/pow"
	"github.com/iotaledger/hornet/v2/pkg/protocol"
)

func init() {
	CoreComponent = &app.CoreComponent{
		Component: &app.Component{
			Name:    "PoW",
			Params:  params,
			Provide: provide,
			Run:     run,
		},
	}
}

var (
	CoreComponent *app.CoreComponent
)

func provide(c *dig.Container) error {

	type handlerDeps struct {
		dig.In
		ProtocolManager *protocol.Manager
	}

	if err := c.Provide(func(deps handlerDeps) *pow.Handler {
		// init the pow handler with all possible settings
		return pow.New(ParamsPoW.RefreshTipsInterval)
	}); err != nil {
		CoreComponent.LogPanic(err)
	}

	return nil
}

func run() error {

	// close the PoW handler on shutdown
	if err := CoreComponent.Daemon().BackgroundWorker("PoW Handler", func(ctx context.Context) {
		CoreComponent.LogInfo("Starting PoW Handler ... done")
		<-ctx.Done()
		CoreComponent.LogInfo("Stopping PoW Handler ...")
		CoreComponent.LogInfo("Stopping PoW Handler ... done")
	}, daemon.PriorityPoWHandler); err != nil {
		CoreComponent.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}
