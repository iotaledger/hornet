package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/iotaledger/hive.go/logger"
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/plugins/dashboard"
	"github.com/gohornet/hornet/plugins/restapi"
)

func getList(a []string) string {
	sort.Strings(a)
	return strings.Join(a, " ")
}

// ParseConfig parses the configuration and initializes the global logger.
func ParseConfig() {
	if err := Config.FetchConfig(); err != nil {
		panic(err)
	}

	if err := logger.InitGlobalLogger(Config.NodeConfig); err != nil {
		panic(err)
	}
}

// PrintConfig prints the loaded configuration, but hides sensitive information.
func PrintConfig() {
	Config.PrintConfig([]string{restapi.CfgRestAPIBasicAuthPasswordHash, restapi.CfgRestAPIBasicAuthPasswordSalt, dashboard.CfgDashboardBasicAuthPasswordHash, dashboard.CfgDashboardBasicAuthPasswordSalt})

	enablePlugins := Config.NodeConfig.Strings(CfgNodeEnablePlugins)
	disablePlugins := Config.NodeConfig.Strings(CfgNodeDisablePlugins)

	if len(enablePlugins) > 0 {
		fmt.Printf("\nThe following plugins are enabled: %s", getList(enablePlugins))
	}
	if len(disablePlugins) > 0 {
		fmt.Printf("\nThe following plugins are disabled: %s", getList(disablePlugins))
	}
}

// ParseFlags defines and parses the command-line flags from os.Args[1:].
func ParseFlags() {
	Config.ParseFlags()
}

// PrintVersion prints out the HORNET version
func PrintVersion() {
	if *version {
		fmt.Println(AppName + " " + AppVersion)
		os.Exit(0)
	}

	if *help {
		if !*helpFull {
			// HideConfigFlags hides all non essential flags from the help/usage text.
			Config.HideConfigFlags()
		}
		flag.Usage()
		os.Exit(0)
	}
}
