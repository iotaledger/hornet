package indexer

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	iotago "github.com/iotaledger/iota.go/v3"

	"github.com/gohornet/hornet/pkg/indexer"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/whiteflag"
	restapiv1 "github.com/gohornet/hornet/plugins/restapi/v1"
)

const (
	waitForNodeSyncedTimeout = 2000 * time.Millisecond
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusDisabled,
		Pluggable: node.Pluggable{
			Name:      "Indexer",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *node.Plugin
	deps   dependencies

	onMilestoneConfirmed *events.Closure
)

type dependencies struct {
	dig.In
	NodeConfig              *configuration.Configuration `name:"nodeConfig"`
	Indexer                 *indexer.Indexer
	SyncManager             *syncmanager.SyncManager
	UTXOManager             *utxo.Manager
	Tangle                  *tangle.Tangle
	Echo                    *echo.Echo
	Bech32HRP               iotago.NetworkPrefix `name:"bech32HRP"`
	RestAPILimitsMaxResults int                  `name:"restAPILimitsMaxResults"`
	ShutdownHandler         *shutdown.ShutdownHandler
}

func provide(c *dig.Container) {

	type indexerDeps struct {
		dig.In
		Storage                   *storage.Storage
		SyncManager               *syncmanager.SyncManager
		DatabasePath              string                       `name:"databasePath"`
		NodeConfig                *configuration.Configuration `name:"nodeConfig"`
		DeSerializationParameters *iotago.DeSerializationParameters
	}

	if err := c.Provide(func(deps indexerDeps) *indexer.Indexer {
		dbPath := filepath.Join(deps.DatabasePath, "indexer")
		idx, err := indexer.NewIndexer(dbPath)
		if err != nil {
			Plugin.LogPanic(err)
		}
		return idx
	}); err != nil {
		Plugin.LogPanic(err)
	}
}

func configure() {
	restapiv1.AddFeature(Plugin.Name)

	routeGroup := deps.Echo.Group("/api/plugins/indexer")
	configureRoutes(routeGroup)

	//TODO: compare ledgerIndex with UTXO and if it does not match, drop tables and iterate over unspent outputs

	if err := Plugin.Node.Daemon().BackgroundWorker("Close Participation database", func(ctx context.Context) {
		<-ctx.Done()

		Plugin.LogInfo("Syncing Indexer database to disk...")
		if err := deps.Indexer.CloseDatabase(); err != nil {
			Plugin.LogPanicf("Syncing Indexer database to disk... failed: %s", err)
		}
		Plugin.LogInfo("Syncing Indexer database to disk... done")
	}, shutdown.PriorityCloseDatabase); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}

	configureEvents()
}

func run() {
	// create a background worker that handles the Indexer
	if err := Plugin.Daemon().BackgroundWorker("Indexer", func(ctx context.Context) {
		Plugin.LogInfo("Starting Indexer ... done")
		attachEvents()
		<-ctx.Done()
		detachEvents()
		Plugin.LogInfo("Stopping Indexer ... done")
	}, shutdown.PriorityIndexer); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
}

func configureEvents() {

	onMilestoneConfirmed = events.NewClosure(func(confirmation *whiteflag.Confirmation) {
		if err := deps.Indexer.ApplyWhiteflagConfirmation(confirmation); err != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("indexer plugin hit a critical error while applying whiteflag confirmation: %s", err.Error()))
		}
	})
}

func attachEvents() {
	deps.Tangle.Events.MilestoneConfirmed.Attach(onMilestoneConfirmed)
}

func detachEvents() {
	deps.Tangle.Events.MilestoneConfirmed.Detach(onMilestoneConfirmed)
}
