package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/iotaledger/hive.go/logger"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/config"
)

var (
	enabledPlugins  []string
	disabledPlugins []string

	version  = flag.BoolP("version", "v", false, "Prints the HORNET version")
	help     = flag.BoolP("help", "h", false, "Prints the HORNET help (--full for all parameters)")
	helpFull = flag.Bool("full", false, "Prints full HORNET help (only in combination with -h)")
)

func AddPluginStatus(name string, status int) {
	switch status {
	case node.Enabled:
		enabledPlugins = append(enabledPlugins, name)
	case node.Disabled:
		disabledPlugins = append(disabledPlugins, name)
	}
}

func getList(a []string) string {
	sort.Strings(a)
	return strings.Join(a, " ")
}

func ParseConfig() {
	if err := config.FetchConfig(); err != nil {
		panic(err)
	}
	parseParameters()

	if err := logger.InitGlobalLogger(config.NodeConfig); err != nil {
		panic(err)
	}
}

func PrintConfig() {
	config.PrintConfig([]string{config.CfgWebAPIBasicAuthPasswordHash, config.CfgWebAPIBasicAuthPasswordSalt, config.CfgDashboardBasicAuthPasswordHash, config.CfgDashboardBasicAuthPasswordSalt})

	enablePlugins := config.NodeConfig.GetStringSlice(config.CfgNodeEnablePlugins)
	disablePlugins := config.NodeConfig.GetStringSlice(config.CfgNodeDisablePlugins)

	if len(enablePlugins) > 0 {
		fmt.Printf("\nThe following plugins are enabled: %s\n", getList(enablePlugins))
	}
	if len(disablePlugins) > 0 {
		fmt.Printf("\nThe following plugins are disabled: %s\n", getList(disablePlugins))
	}
}

// HideConfigFlags hides all non essential flags from the help/usage text.
func HideConfigFlags() {
	config.HideConfigFlags()
}

// ParseFlags defines and parses the command-line flags from os.Args[1:].
func ParseFlags() {
	config.ParseFlags()
}

// PrintVersion prints out the HORNET version
func PrintVersion() {
	if *version {
		fmt.Println(AppName + " " + AppVersion)
		os.Exit(0)
	}

	if *help {
		if !*helpFull {
			HideConfigFlags()
		}
		flag.Usage()
		os.Exit(0)
	}
}
