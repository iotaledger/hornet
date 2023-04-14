package pow

import (
	"context"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hornet/v2/pkg/components"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/pow"
	"github.com/iotaledger/hornet/v2/pkg/protocol"
)

func init() {
	Component = &app.Component{
		Name:      "PoW",
		Params:    params,
		IsEnabled: components.IsAutopeeringEntryNodeDisabled, // do not enable in "autopeering entry node" mode
		Provide:   provide,
		Run:       run,
	}
}

var (
	Component *app.Component
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
		Component.LogPanic(err)
	}

	return nil
}

func run() error {

	// close the PoW handler on shutdown
	if err := Component.Daemon().BackgroundWorker("PoW Handler", func(ctx context.Context) {
		Component.LogInfo("Starting PoW Handler ... done")
		<-ctx.Done()
		Component.LogInfo("Stopping PoW Handler ...")
		Component.LogInfo("Stopping PoW Handler ... done")
	}, daemon.PriorityPoWHandler); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}
