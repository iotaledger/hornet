package migrator

import (
	"fmt"
	"net/http"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/iota.go/api"

	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
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
	}
	if err := c.Provide(func(deps serviceDependencies) (*migrator.MigratorService, error) {
		iotaAPI, err := api.ComposeAPI(api.HTTPClientSettings{
			URI:    deps.NodeConfig.String(CfgMigratorAPIAddress),
			Client: &http.Client{Timeout: deps.NodeConfig.Duration(CfgMigratorAPITimeout)},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize API: %w", err)
		}
		return migrator.NewService(
			iotaAPI,
			deps.NodeConfig.String(CfgMigratorStateFilePath),
			deps.NodeConfig.String(CfgMigratorCoordinatorAddress),
			deps.NodeConfig.Int(CfgMigratorCoordinatorMerkleTreeDepth),
		), nil
	}); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(Plugin.Name)

	var state *migrator.State
	if *bootstrap {
		state = &migrator.State{
			LatestMilestoneIndex: *startIndex,
		}
	}
	// TODO: perform sanity check, that the latest migration milestone has MigratedAt lower than the state
	if err := deps.MigratorService.InitState(state); err != nil {
		log.Fatalf("failed to initialize migrator: %s", err)
	}
}

func run() {
	err := Plugin.Node.Daemon().BackgroundWorker(Plugin.Name, func(shutdownSignal <-chan struct{}) {
		log.Infof("Starting %s ... done", Plugin.Name)
		if err := deps.MigratorService.Start(shutdownSignal); err != nil {
			log.Panic(err)
		}
		log.Infof("Stopping %s ... done", Plugin.Name)
	}, shutdown.PriorityMigrator)
	if err != nil {
		log.Panicf("failed to start worker: %s", err)
	}
}
