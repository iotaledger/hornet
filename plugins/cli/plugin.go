package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/packages/parameter"
	"github.com/gohornet/hornet/packages/profile"
)

var (
	// AppVersion version number
	AppVersion = "0.2.12"

	// AppName app code name
	AppName = "HORNET"
)

var (
	PLUGIN = node.NewPlugin("CLI", node.Enabled, configure, run)
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

func configure(ctx *node.Plugin) {

	fmt.Print(fmt.Sprintf(`
              ██╗  ██╗ ██████╗ ██████╗ ███╗   ██╗███████╗████████╗
              ██║  ██║██╔═══██╗██╔══██╗████╗  ██║██╔════╝╚══██╔══╝
              ███████║██║   ██║██████╔╝██╔██╗ ██║█████╗     ██║
              ██╔══██║██║   ██║██╔══██╗██║╚██╗██║██╔══╝     ██║
              ██║  ██║╚██████╔╝██║  ██║██║ ╚████║███████╗   ██║
              ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝   ╚═╝
                                   v%s
`+"\n\n", AppVersion))

	if parameter.NodeConfig.GetString("useProfile") == "auto" {
		ctx.Node.Logger.Infof("Profile mode 'auto', Using profile '%s'", profile.GetProfile().Name)
	} else {
		ctx.Node.Logger.Infof("Using profile '%s'", profile.GetProfile().Name)
	}

	ctx.Node.Logger.Info("Loading plugins ...")
}

func run(ctx *node.Plugin) {
	// do nothing; everything is handled in the configure step
}
