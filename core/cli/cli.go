package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/iotaledger/hive.go/logger"
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/config"
)

var (
	version  = flag.BoolP("version", "v", false, "Prints the HORNET version")
	help     = flag.BoolP("help", "h", false, "Prints the HORNET help (--full for all parameters)")
	helpFull = flag.Bool("full", false, "Prints full HORNET help (only in combination with -h)")
)

func getList(a []string) string {
	sort.Strings(a)
	return strings.Join(a, " ")
}

// ParseConfig parses the configuration and initializes the global logger.
func ParseConfig() {
	if err := config.FetchConfig(); err != nil {
		panic(err)
	}

	if err := logger.InitGlobalLogger(config.NodeConfig); err != nil {
		panic(err)
	}
}

// PrintConfig prints the loaded configuration, but hides sensitive information.
func PrintConfig() {
	config.PrintConfig([]string{config.CfgRestAPIBasicAuthPasswordHash, config.CfgRestAPIBasicAuthPasswordSalt, config.CfgDashboardBasicAuthPasswordHash, config.CfgDashboardBasicAuthPasswordSalt})

	enablePlugins := config.NodeConfig.Strings(config.CfgNodeEnablePlugins)
	disablePlugins := config.NodeConfig.Strings(config.CfgNodeDisablePlugins)

	if len(enablePlugins) > 0 {
		fmt.Printf("\nThe following plugins are enabled: %s", getList(enablePlugins))
	}
	if len(disablePlugins) > 0 {
		fmt.Printf("\nThe following plugins are disabled: %s", getList(disablePlugins))
	}
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
			// HideConfigFlags hides all non essential flags from the help/usage text.
			config.HideConfigFlags()
		}
		flag.Usage()
		os.Exit(0)
	}
}
