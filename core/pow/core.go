package pow

import (
	"context"

	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/configuration"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	CoreComponent = &app.CoreComponent{
		Component: &app.Component{
			Name:     "PoW",
			DepsFunc: func(cDeps dependencies) { deps = cDeps },
			Params:   params,
			Provide:  provide,
			Run:      run,
		},
	}
}

var (
	CoreComponent *app.CoreComponent
	deps          dependencies
)

type dependencies struct {
	dig.In
	Handler *pow.Handler
}

func provide(c *dig.Container) error {

	type handlerDeps struct {
		dig.In
		AppConfig          *configuration.Configuration `name:"appConfig"`
		ProtocolParameters *iotago.ProtocolParameters
	}

	if err := c.Provide(func(deps handlerDeps) *pow.Handler {
		// init the pow handler with all possible settings
		return pow.New(deps.ProtocolParameters.MinPoWScore, deps.AppConfig.Duration(CfgPoWRefreshTipsInterval))
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
	}, shutdown.PriorityPoWHandler); err != nil {
		CoreComponent.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}
