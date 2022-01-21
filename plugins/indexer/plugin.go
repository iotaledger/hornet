package indexer

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	iotago "github.com/iotaledger/iota.go/v3"

	"github.com/gohornet/hornet/pkg/indexer"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	restapiv2 "github.com/gohornet/hornet/plugins/restapi/v2"
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

	onLedgerUpdated *events.Closure
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
	restapiv2.AddPlugin("indexer/v1")

	routeGroup := deps.Echo.Group("/api/plugins/indexer/v1")
	configureRoutes(routeGroup)

	initializeIndexer()

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

	onLedgerUpdated = events.NewClosure(func(index milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents) {
		if err := deps.Indexer.UpdatedLedger(index, newOutputs, newSpents); err != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("indexer plugin hit a critical error while updating ledger: %s", err.Error()))
		}
	})
}

func attachEvents() {
	deps.Tangle.Events.LedgerUpdated.Attach(onLedgerUpdated)
}

func detachEvents() {
	deps.Tangle.Events.LedgerUpdated.Detach(onLedgerUpdated)
}

func initializeIndexer() {
	//Compare Indexer ledgerIndex with UTXO ledgerIndex and if it does not match, drop tables and import unspent outputs
	needsInitialImport := false
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	utxoLedgerIndex, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		Plugin.LogPanicf("Reading UTXO ledger index failed: %s", err)
	}
	indexerLedgerIndex, err := deps.Indexer.LedgerIndex()
	if err != nil {
		if errors.Is(err, indexer.ErrNotFound) {
			needsInitialImport = true
		} else {
			Plugin.LogPanicf("Reading Indexer ledger index failed: %s", err)
		}
	} else {
		if utxoLedgerIndex != indexerLedgerIndex {
			Plugin.LogInfof("Re-indexing UTXO ledger with index: %d", utxoLedgerIndex)
			deps.Indexer.Clear()
			needsInitialImport = true
		}
	}

	if needsInitialImport {
		importer := deps.Indexer.ImportTransaction()
		if err := deps.UTXOManager.ForEachUnspentOutput(func(output *utxo.Output) bool {
			if err := importer.AddOutput(output); err != nil {
				Plugin.LogPanicf("Importing Indexer data failed: %s", err)
			}
			return true
		}); err != nil {
			Plugin.LogPanicf("Importing Indexer data failed: %s", err)
		}
		if err := importer.Finalize(utxoLedgerIndex); err != nil {
			Plugin.LogPanicf("Importing Indexer data failed: %s", err)
		}
	}
}
