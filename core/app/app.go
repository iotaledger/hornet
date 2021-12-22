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
	Version = "1.1.1"
)

var (
	version  = flag.BoolP("version", "v", false, "Prints the HORNET version")
	help     = flag.BoolP("help", "h", false, "Prints the HORNET help (--full for all parameters)")
	helpFull = flag.Bool("full", false, "Prints full HORNET help (only in combination with -h)")

	// configs
	nodeConfig    = configuration.New()
	peeringConfig = configuration.New()
	profileConfig = configuration.New()

	// config file flags
	configFilesFlagSet  = flag.NewFlagSet("config_files", flag.ContinueOnError)
	nodeCfgFilePath     = configFilesFlagSet.StringP(CfgConfigFilePathNodeConfig, "c", "config.json", "file path of the config file")
	peeringCfgFilePath  = configFilesFlagSet.StringP(CfgConfigFilePathPeeringConfig, "n", "peering.json", "file path of the peering config file")
	profilesCfgFilePath = configFilesFlagSet.String(CfgConfigFilePathProfilesConfig, "profiles.json", "file path of the profiles config file")

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
			Name:           "App",
			Params:         params,
			InitConfigPars: initConfigPars,
			Provide:        provide,
			Configure:      configure,
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
)

func initialize(params map[string][]*flag.FlagSet, maskedKeys []string) (*node.InitConfig, error) {

	configFlagSets, err := normalizeFlagSets(params)
	if err != nil {
		return nil, err
	}

	if toolset.ShouldHandleTools() {
		// Just parse the configFilesFlagSet and ignore errors
		fs := flag.NewFlagSet("", flag.ContinueOnError)
		fs.AddFlagSet(configFilesFlagSet)
		fs.Parse(os.Args[1:])

		if err = loadCfg(configFlagSets); err != nil {
			return nil, err
		}
		toolset.HandleTools(nodeConfig)
		// HandleTools will call os.Exit
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage of %s (%s %s):

Run '%s tools' to list all available tools.

Command line flags:
`, os.Args[0], Name, Version, os.Args[0])
		flag.PrintDefaults()
	}

	var flagSetsToParse = configFlagSets
	flagSetsToParse["config_files"] = configFilesFlagSet

	parseFlags(flagSetsToParse)
	printVersion(flagSetsToParse)

	if err = loadCfg(configFlagSets); err != nil {
		return nil, err
	}

	if err = nodeConfig.SetDefault(logger.ConfigurationKeyDisableCaller, true); err != nil {
		panic(err)
	}

	if err = logger.InitGlobalLogger(nodeConfig); err != nil {
		panic(err)
	}

	fmt.Printf(`
              ██╗  ██╗ ██████╗ ██████╗ ███╗   ██╗███████╗████████╗
              ██║  ██║██╔═══██╗██╔══██╗████╗  ██║██╔════╝╚══██╔══╝
              ███████║██║   ██║██████╔╝██╔██╗ ██║█████╗     ██║
              ██╔══██║██║   ██║██╔══██╗██║╚██╗██║██╔══╝     ██║
              ██║  ██║╚██████╔╝██║  ██║██║ ╚████║███████╗   ██║
              ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝   ╚═╝
                                   v%s
`+"\n", Version)

	printConfig(maskedKeys)

	return &node.InitConfig{
		EnabledPlugins:  nodeConfig.Strings(CfgNodeEnablePlugins),
		DisabledPlugins: nodeConfig.Strings(CfgNodeDisablePlugins),
	}, nil
}

func initConfigPars(c *dig.Container) {

	type cfgResult struct {
		dig.Out
		NodeConfig            *configuration.Configuration `name:"nodeConfig"`
		PeeringConfig         *configuration.Configuration `name:"peeringConfig"`
		ProfileConfig         *configuration.Configuration `name:"profilesConfig"`
		PeeringConfigFilePath string                       `name:"peeringConfigFilePath"`
	}

	if err := c.Provide(func() cfgResult {
		return cfgResult{
			NodeConfig:            nodeConfig,
			PeeringConfig:         peeringConfig,
			ProfileConfig:         profileConfig,
			PeeringConfigFilePath: *peeringCfgFilePath,
		}
	}); err != nil {
		InitPlugin.LogPanic(err)
	}
}

func provide(c *dig.Container) {

	if err := c.Provide(func() *app.AppInfo {
		return &app.AppInfo{
			Name:                Name,
			Version:             Version,
			LatestGitHubVersion: "",
		}
	}); err != nil {
		InitPlugin.LogPanic(err)
	}
}

func configure() {
	InitPlugin.LogInfo("Loading plugins ...")
}
