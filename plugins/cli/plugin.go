package cli

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/tcnksm/go-latest"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/packages/parameter"
	"github.com/gohornet/hornet/packages/profile"
	"github.com/gohornet/hornet/packages/shutdown"
)

var (
	// AppVersion version number
	AppVersion          = "0.4.0-dev"
	LatestGithubVersion = AppVersion

	// AppName app code name
	AppName = "HORNET"

	githubTag *latest.GithubTag
)

var (
	PLUGIN = node.NewPlugin("CLI", node.Enabled, configure, run)
	log    *logger.Logger
)

func onAddPlugin(name string, status int) {
	AddPluginStatus(node.GetPluginIdentifier(name), status)
}

func init() {
	for name, status := range node.GetPlugins() {
		onAddPlugin(name, status)
	}

	node.Events.AddPlugin.Attach(events.NewClosure(onAddPlugin))

	flag.Usage = printUsage
}

func parseParameters() {
	for _, pluginName := range parameter.NodeConfig.GetStringSlice("node.disableplugins") {
		node.DisabledPlugins[strings.ToLower(pluginName)] = true
	}
	for _, pluginName := range parameter.NodeConfig.GetStringSlice("node.enableplugins") {
		node.EnabledPlugins[strings.ToLower(pluginName)] = true
	}
}

func configure(plugin *node.Plugin) {

	log = logger.NewLogger(plugin.Name)

	githubTag = &latest.GithubTag{
		Owner:             "gohornet",
		Repository:        "hornet",
		FixVersionStrFunc: latest.DeleteFrontV(),
	}

	fmt.Print(fmt.Sprintf(`
              ██╗  ██╗ ██████╗ ██████╗ ███╗   ██╗███████╗████████╗
              ██║  ██║██╔═══██╗██╔══██╗████╗  ██║██╔════╝╚══██╔══╝
              ███████║██║   ██║██████╔╝██╔██╗ ██║█████╗     ██║
              ██╔══██║██║   ██║██╔══██╗██║╚██╗██║██╔══╝     ██║
              ██║  ██║╚██████╔╝██║  ██║██║ ╚████║███████╗   ██║
              ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝   ╚═╝
                                   v%s
`+"\n\n", AppVersion))

	checkLatestVersion()

	if parameter.NodeConfig.GetString("useProfile") == "auto" {
		log.Infof("Profile mode 'auto', Using profile '%s'", profile.GetProfile().Name)
	} else {
		log.Infof("Using profile '%s'", profile.GetProfile().Name)
	}

	log.Info("Loading plugins ...")
}

func checkLatestVersion() {

	res, _ := latest.Check(githubTag, AppVersion)

	if res.Outdated {
		log.Infof("Update to %s available on https://github.com/gohornet/hornet/releases/latest", res.Current)
		LatestGithubVersion = res.Current
	}
}

func run(plugin *node.Plugin) {

	// create a background worker that checks for latest version every hour
	daemon.BackgroundWorker("Version update checker", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(checkLatestVersion, 1*time.Hour, shutdownSignal)
	}, shutdown.ShutdownPriorityUpdateCheck)
}
