package migrator

import (
	"fmt"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	iotago "github.com/iotaledger/iota.go/v2"

	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/configuration"
)

const (
	// CfgMigratorBootstrap configures whether the migration process is bootstrapped.
	CfgMigratorBootstrap = "migratorBootstrap"
	// CfgMigratorStartIndex configures the index of the first milestone to migrate.
	CfgMigratorStartIndex = "migratorStartIndex"
)

func init() {
	_ = flag.CommandLine.MarkHidden(CfgMigratorBootstrap)
	_ = flag.CommandLine.MarkHidden(CfgMigratorStartIndex)

	Plugin = &node.Plugin{
		Status: node.StatusDisabled,
		Pluggable: node.Pluggable{
			Name:      "Migrator",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *node.Plugin
	deps   dependencies

	bootstrap  = flag.Bool(CfgMigratorBootstrap, false, "bootstrap the migration process")
	startIndex = flag.Uint32(CfgMigratorStartIndex, 1, "index of the first milestone to migrate")
)

type dependencies struct {
	dig.In
	UTXOManager     *utxo.Manager
	NodeConfig      *configuration.Configuration `name:"nodeConfig"`
	MigratorService *migrator.MigratorService
	ShutdownHandler *shutdown.ShutdownHandler
}

// provide provides the MigratorService as a singleton.
func provide(c *dig.Container) {

	type serviceDeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
		Validator  *migrator.Validator
	}

	if err := c.Provide(func(deps serviceDeps) *migrator.MigratorService {

		maxReceiptEntries := deps.NodeConfig.Int(CfgMigratorReceiptMaxEntries)
		switch {
		case maxReceiptEntries > iotago.MaxMigratedFundsEntryCount:
			Plugin.Panicf("%s (set to %d) can be max %d", CfgMigratorReceiptMaxEntries, maxReceiptEntries, iotago.MaxMigratedFundsEntryCount)
		case maxReceiptEntries <= 0:
			Plugin.Panicf("%s must be greather than 0", CfgMigratorReceiptMaxEntries)
		}

		return migrator.NewService(
			deps.Validator,
			deps.NodeConfig.String(CfgMigratorStateFilePath),
			deps.NodeConfig.Int(CfgMigratorReceiptMaxEntries),
		)
	}); err != nil {
		Plugin.Panic(err)
	}
}

func configure() {

	var msIndex *uint32
	if *bootstrap {
		msIndex = startIndex
	}

	if err := deps.MigratorService.InitState(msIndex, deps.UTXOManager); err != nil {
		Plugin.LogFatalf("failed to initialize migrator: %s", err)
	}
}

func run() {

	if err := Plugin.Node.Daemon().BackgroundWorker(Plugin.Name, func(shutdownSignal <-chan struct{}) {
		Plugin.LogInfof("Starting %s ... done", Plugin.Name)
		deps.MigratorService.Start(shutdownSignal, func(err error) bool {

			if err := common.IsCriticalError(err); err != nil {
				deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("migrator plugin hit a critical error: %s", err))
				return false
			}

			if err := common.IsSoftError(err); err != nil {
				deps.MigratorService.Events.SoftError.Trigger(err)
			}

			// lets just log the err and halt querying for a configured period
			Plugin.LogWarn(err)
			return timeutil.Sleep(deps.NodeConfig.Duration(CfgMigratorQueryCooldownPeriod), shutdownSignal)
		})
		Plugin.LogInfof("Stopping %s ... done", Plugin.Name)
	}, shutdown.PriorityMigrator); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}
}
