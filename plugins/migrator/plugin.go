package migrator

import (
	"time"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/model/migrator"
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
		return migrator.NewService(deps.Validator, deps.NodeConfig.String(CfgMigratorStateFilePath)), nil
	}); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(Plugin.Name)

	var state *migrator.State
	if *bootstrap {
		state = &migrator.State{LatestMilestoneIndex: *startIndex}
	}
	// TODO: perform sanity check, that the latest migration milestone has MigratedAt lower than the state
	if err := deps.MigratorService.InitState(state); err != nil {
		log.Fatalf("failed to initialize migrator: %s", err)
	}
}

func run() {
	err := Plugin.Node.Daemon().BackgroundWorker(Plugin.Name, func(shutdownSignal <-chan struct{}) {
		log.Infof("Starting %s ... done", Plugin.Name)
		deps.MigratorService.Start(shutdownSignal, func(err error) bool {
			// lets just log the err and halt querying for a configured period
			log.Warn(err)
			time.Sleep(deps.NodeConfig.Duration(CfgMigratorQueryCooldownPeriod))
			return false
		})
		log.Infof("Stopping %s ... done", Plugin.Name)
	}, shutdown.PriorityMigrator)
	if err != nil {
		log.Panicf("failed to start worker: %s", err)
	}
}
