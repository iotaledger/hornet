package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gohornet/hornet/packages/node"
	"github.com/iotaledger/hive.go/parameter"
)

var enabledPlugins []string
var disabledPlugins []string

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

func printUsage() {
	fmt.Fprintf(
		os.Stderr,
		"\n"+
			"HORNET\n\n"+
			"  A lightweight modular IOTA node.\n\n"+
			"Usage:\n\n"+
			"  %s [OPTIONS]\n\n"+
			"Options:\n",
		filepath.Base(os.Args[0]),
	)
	flag.PrintDefaults()

	fmt.Fprintf(os.Stderr, "\nThe following plugins are enabled: %s\n", getList(parameter.NodeConfig.GetStringSlice("node.enableplugins")))
	fmt.Fprintf(os.Stderr, "\nThe following plugins are disabled: %s\n", getList(parameter.NodeConfig.GetStringSlice("node.disableplugins")))
}
