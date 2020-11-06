package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gohornet/hornet/pkg/toolset"
	"github.com/iotaledger/hive.go/configuration"
	flag "github.com/spf13/pflag"
	"github.com/tcnksm/go-latest"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/pkg/shutdown"
)

var (
	// Version version number
	Version             = "0.5.3"
	LatestGitHubVersion = Version

	// Name app code name
	Name = "HORNET"

	githubTag *latest.GithubTag
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
		"node.disablePlugins": {},
		"node.enablePlugins":  {},
		"peeringConfig":       {},
		"profilesConfig":      {},
		"useProfile":          {},
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
			Name:      "Init",
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
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
)

func initialize(params map[string][]*flag.FlagSet, maskedKeys []string) (*node.InitConfig, error) {
	flagSets, err := normalizeFlagSets(params)
	if err != nil {
		return nil, err
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
}

func configure() {
	log = logger.NewLogger(InitPlugin.Name)

	githubTag = &latest.GithubTag{
		Owner:             "gohornet",
		Repository:        "hornet",
		FixVersionStrFunc: fixVersion,
		TagFilterFunc:     includeVersionInCheck,
	}

	fmt.Printf("\n\n"+`
              ██╗  ██╗ ██████╗ ██████╗ ███╗   ██╗███████╗████████╗
              ██║  ██║██╔═══██╗██╔══██╗████╗  ██║██╔════╝╚══██╔══╝
              ███████║██║   ██║██████╔╝██╔██╗ ██║█████╗     ██║
              ██╔══██║██║   ██║██╔══██╗██║╚██╗██║██╔══╝     ██║
              ██║  ██║╚██████╔╝██║  ██║██║ ╚████║███████╗   ██║
              ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝   ╚═╝
                                   v%s
`+"\n\n", Version)

	checkLatestVersion()

	log.Info("Loading plugins ...")
}

func run() {
	// create a background worker that checks for latest version every hour
	// TODO: move this into a separate plugin
	_ = InitPlugin.Node.Daemon().BackgroundWorker("Version update checker", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(checkLatestVersion, 1*time.Hour, shutdownSignal)
	}, shutdown.PriorityUpdateCheck)
}

func fixVersion(version string) string {
	ver := strings.Replace(version, "v", "", 1)
	if !strings.Contains(ver, "-rc.") {
		ver = strings.Replace(ver, "-rc", "-rc.", 1)
	}
	return ver
}

func includeVersionInCheck(version string) bool {
	isPrerelease := func(ver string) bool {
		return strings.Contains(ver, "-rc")
	}

	if isPrerelease(Version) {
		// When using pre-release versions, check for any updates
		return true
	}

	return !isPrerelease(version)
}

func checkLatestVersion() {

	res, err := latest.Check(githubTag, fixVersion(Version))
	if err != nil {
		log.Warnf("Update check failed: %s", err.Error())
		return
	}

	if res.Outdated {
		log.Infof("Update to %s available on https://github.com/gohornet/hornet/releases/latest", res.Current)
		LatestGitHubVersion = res.Current
	}
}
