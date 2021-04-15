package migrator

import (
	"fmt"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	iotago "github.com/iotaledger/iota.go/v2"

	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/core/gracefulshutdown"
	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"
)

const (
	// CfgMigratorBootstrap configures whether the migration process is bootstrapped.
	CfgMigratorBootstrap = "migratorBootstrap"
	// CfgMigratorStartIndex configures the index of the first milestone to migrate.
	CfgMigratorStartIndex = "migratorStartIndex"
)

func init() {
	flag.CommandLine.MarkHidden(CfgMigratorBootstrap)
	flag.CommandLine.MarkHidden(CfgMigratorStartIndex)

	Plugin = &node.Plugin{
		Status: node.Disabled,
		Pluggable: node.Pluggable{
			Name:      "Migrator",
			DepsFunc:  func(cDeps pluginDependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *node.Plugin

	log  *logger.Logger
	deps pluginDependencies

	bootstrap  = flag.Bool(CfgMigratorBootstrap, false, "bootstrap the migration process")
	startIndex = flag.Uint32(CfgMigratorStartIndex, 1, "index of the first milestone to migrate")
)

type pluginDependencies struct {
	dig.In
	UTXOManager     *utxo.Manager
	NodeConfig      *configuration.Configuration `name:"nodeConfig"`
	MigratorService *migrator.MigratorService
}

// provide provides the MigratorService as a singleton.
func provide(c *dig.Container) {
	type serviceDependencies struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
		Validator  *migrator.Validator
	}
	if err := c.Provide(func(deps serviceDependencies) (*migrator.MigratorService, error) {

		maxReceiptEntries := deps.NodeConfig.Int(CfgMigratorReceiptMaxEntries)
		switch {
		case maxReceiptEntries > iotago.MaxMigratedFundsEntryCount:
			panic(fmt.Sprintf("%s (set to %d) can be max %d", CfgMigratorReceiptMaxEntries, maxReceiptEntries, iotago.MaxMigratedFundsEntryCount))
		case maxReceiptEntries <= 0:
			panic(fmt.Sprintf("%s must be greather than 0", CfgMigratorReceiptMaxEntries))
		}

		return migrator.NewService(
			deps.Validator,
			deps.NodeConfig.String(CfgMigratorStateFilePath),
			deps.NodeConfig.Int(CfgMigratorReceiptMaxEntries),
		), nil
	}); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(Plugin.Name)

	var msIndex *uint32
	if *bootstrap {
		msIndex = startIndex
	}

	if err := deps.MigratorService.InitState(msIndex, deps.UTXOManager); err != nil {
		log.Fatalf("failed to initialize migrator: %s", err)
	}
}

func run() {
	if err := Plugin.Node.Daemon().BackgroundWorker(Plugin.Name, func(shutdownSignal <-chan struct{}) {
		log.Infof("Starting %s ... done", Plugin.Name)
		deps.MigratorService.Start(shutdownSignal, func(err error) bool {

			if err := common.IsCriticalError(err); err != nil {
				gracefulshutdown.SelfShutdown(fmt.Sprintf("migrator plugin hit a critical error: %s", err.Error()))
				return false
			}

			if err := common.IsSoftError(err); err != nil {
				deps.MigratorService.Events.SoftError.Trigger(err)
			}

			// lets just log the err and halt querying for a configured period
			log.Warn(err)
			return timeutil.Sleep(deps.NodeConfig.Duration(CfgMigratorQueryCooldownPeriod), shutdownSignal)
		})
		log.Infof("Stopping %s ... done", Plugin.Name)
	}, shutdown.PriorityMigrator); err != nil {
		log.Panicf("failed to start worker: %s", err)
	}
}
