package app

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/app"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/toolset"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"
)

var (
	// Name of the app.
	Name = "HORNET"

	// Version of the app.
	Version = "0.6.0-alpha"
)

var (
	version  = flag.BoolP("version", "v", false, "Prints the HORNET version")
	help     = flag.BoolP("help", "h", false, "Prints the HORNET help (--full for all parameters)")
	helpFull = flag.Bool("full", false, "Prints full HORNET help (only in combination with -h)")

	// configs
	nodeConfig    = configuration.New()
	peeringConfig = configuration.New()
	profileConfig = configuration.New()

	// flags
	nodeCfgFilePath     = flag.StringP(CfgConfigFilePathNodeConfig, "c", "config.json", "file path of the config file")
	peeringCfgFilePath  = flag.StringP(CfgConfigFilePathPeeringConfig, "n", "peering.json", "file path of the peering config file")
	profilesCfgFilePath = flag.String(CfgConfigFilePathProfilesConfig, "profiles.json", "file path of the profiles config file")

	nonHiddenFlag = map[string]struct{}{
		"config":              {},
		"config-dir":          {},
		"node.profile":        {},
		"node.disablePlugins": {},
		"node.enablePlugins":  {},
		"peeringConfig":       {},
		"profilesConfig":      {},
		"version":             {},
		"help":                {},
	}

	cfgNames = map[string]struct{}{
		"nodeConfig":    {},
		"peeringConfig": {},
		"profileConfig": {},
	}

	ErrConfigDoesNotExist = errors.New("config does not exist")
)

func init() {
	InitPlugin = &node.InitPlugin{
		Pluggable: node.Pluggable{
			Name:      "App",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
		},
		Configs: map[string]*configuration.Configuration{
			"nodeConfig":    nodeConfig,
			"peeringConfig": peeringConfig,
			"profileConfig": profileConfig,
		},
		Init: initialize,
	}
}

var (
	InitPlugin *node.InitPlugin
	log        *logger.Logger
	deps       dependencies
)

type dependencies struct {
	dig.In
	AppInfo *app.AppInfo
}

func initialize(params map[string][]*flag.FlagSet, maskedKeys []string) (*node.InitConfig, error) {
	flagSets, err := normalizeFlagSets(params)
	if err != nil {
		return nil, err
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage of %s:
%s %s

Run '%s tools' to list all available tools.

Command line flags:
`, os.Args[0], Name, Version, os.Args[0])
		flag.PrintDefaults()
	}

	parseFlags(flagSets)
	printVersion(flagSets)

	if err := loadCfg(flagSets); err != nil {
		return nil, err
	}

	if err := logger.InitGlobalLogger(nodeConfig); err != nil {
		panic(err)
	}

	toolset.HandleTools()
	printConfig(maskedKeys)

	return &node.InitConfig{
		EnabledPlugins:  nodeConfig.Strings(CfgNodeEnablePlugins),
		DisabledPlugins: nodeConfig.Strings(CfgNodeDisablePlugins),
	}, nil
}

func provide(c *dig.Container) {

	if err := c.Provide(func() *app.AppInfo {
		return &app.AppInfo{
			Name:                Name,
			Version:             Version,
			LatestGitHubVersion: "",
		}
	}); err != nil {
		panic(err)
	}
	if err := c.Provide(func() *configuration.Configuration {
		return nodeConfig
	}, dig.Name("nodeConfig")); err != nil {
		panic(err)
	}
	if err := c.Provide(func() *configuration.Configuration {
		return peeringConfig
	}, dig.Name("peeringConfig")); err != nil {
		panic(err)
	}
	if err := c.Provide(func() *configuration.Configuration {
		return profileConfig
	}, dig.Name("profilesConfig")); err != nil {
		panic(err)
	}
	if err := c.Provide(func() string {
		return *peeringCfgFilePath
	}, dig.Name("peeringConfigFilePath")); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(InitPlugin.Name)

	fmt.Printf("\n\n"+`
              ██╗  ██╗ ██████╗ ██████╗ ███╗   ██╗███████╗████████╗
              ██║  ██║██╔═══██╗██╔══██╗████╗  ██║██╔════╝╚══██╔══╝
              ███████║██║   ██║██████╔╝██╔██╗ ██║█████╗     ██║
              ██╔══██║██║   ██║██╔══██╗██║╚██╗██║██╔══╝     ██║
              ██║  ██║╚██████╔╝██║  ██║██║ ╚████║███████╗   ██║
              ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝   ╚═╝
                                   v%s
`+"\n\n", deps.AppInfo.Version)

	log.Info("Loading plugins ...")
}
