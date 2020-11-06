package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/configuration"
	flag "github.com/spf13/pflag"
	"github.com/tcnksm/go-latest"
	"go.uber.org/dig"

	configpkg "github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/shutdown"
)

var (
	// AppVersion version number
	AppVersion          = "0.5.7-rc1"
	LatestGithubVersion = AppVersion

	// AppName app code name
	AppName = "HORNET"

	githubTag *latest.GithubTag
)

var (
	ConfigFlagSet  = flag.NewFlagSet("", flag.ContinueOnError)
	PeeringFlagSet = flag.NewFlagSet("", flag.ContinueOnError)

	version  = flag.BoolP("version", "v", false, "Prints the HORNET version")
	help     = flag.BoolP("help", "h", false, "Prints the HORNET help (--full for all parameters)")
	helpFull = flag.Bool("full", false, "Prints full HORNET help (only in combination with -h)")

	// flags
	configFilePath   = flag.StringP(configpkg.FlagFilePathConfig, "c", "config.json", "file path of the config file")
	peeringFilePath  = flag.StringP(configpkg.FlagFilePathPeeringConfig, "n", "peering.json", "file path of the peering config file")
	profilesFilePath = flag.String(configpkg.FlagFilePathProfilesConfig, "profiles.json", "file path of the profiles config file")

	Config = configpkg.New(config.WithConfigFilePath(*configFilePath), config.WithPeeringFilePath(*peeringFilePath), configpkg.WithProfilesFilePath(*profilesFilePath))
)

func init() {
	CoreModule = &node.CoreModule{
		Name:      "CLI",
		DepsFunc:  func(cDeps dependencies) { deps = cDeps },
		Provide:   provide,
		Configure: configure,
		Run:       run,
	}
}

var (
	CoreModule *node.CoreModule
	log        *logger.Logger
	deps       dependencies
)

type dependencies struct {
	dig.In
	NodeConfig *configuration.Configuration `name:"nodeConfig"`
}

func provide(c *dig.Container) {
	if err := c.Provide(func() *configuration.Configuration {
		return Config.NodeConfig
	}, dig.Name("nodeConfig")); err != nil {
		panic(err)
	}
	if err := c.Provide(func() *configuration.Configuration {
		return Config.PeeringConfig
	}, dig.Name("peeringConfig")); err != nil {
		panic(err)
	}
	if err := c.Provide(func() *configuration.Configuration {
		return Config.ProfilesConfig
	}, dig.Name("profilesConfig")); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(CoreModule.Name)

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
`+"\n\n", AppVersion)

	checkLatestVersion()

	log.Info("Loading plugins ...")
}

func run() {
	// create a background worker that checks for latest version every hour
	CoreModule.Daemon().BackgroundWorker("Version update checker", func(shutdownSignal <-chan struct{}) {
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

	if isPrerelease(AppVersion) {
		// When using pre-release versions, check for any updates
		return true
	}

	return !isPrerelease(version)
}

func checkLatestVersion() {

	res, err := latest.Check(githubTag, fixVersion(AppVersion))
	if err != nil {
		log.Warnf("Update check failed: %s", err.Error())
		return
	}

	if res.Outdated {
		log.Infof("Update to %s available on https://github.com/gohornet/hornet/releases/latest", res.Current)
		LatestGithubVersion = res.Current
	}
}
