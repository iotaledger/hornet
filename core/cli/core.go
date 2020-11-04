package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/tcnksm/go-latest"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/profile"
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
		return config.NodeConfig
	}, dig.Name("nodeConfig")); err != nil {
		panic(err)
	}
	if err := c.Provide(func() *configuration.Configuration {
		return config.PeeringConfig
	}, dig.Name("peeringConfig")); err != nil {
		panic(err)
	}
	if err := c.Provide(func() *configuration.Configuration {
		return config.ProfilesConfig
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

	if deps.NodeConfig.String(config.CfgProfileUseProfile) == config.AutoProfileName {
		log.Infof("Profile mode 'auto', Using profile '%s'", profile.LoadProfile().Name)
	} else {
		log.Infof("Using profile '%s'", profile.LoadProfile().Name)
	}

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
