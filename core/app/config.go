package app

import (
	"fmt"
	"os"
	"sort"
	"strings"

	flag "github.com/spf13/pflag"
)

func getList(a []string) string {
	sort.Strings(a)
	return "\n   - " + strings.Join(a, "\n   - ")
}

func normalizeFlagSets(params map[string][]*flag.FlagSet) (map[string]*flag.FlagSet, error) {
	fs := make(map[string]*flag.FlagSet)
	for cfgName, flagSets := range params {

		// check whether the config even exists
		if _, has := cfgNames[cfgName]; !has {
			return nil, fmt.Errorf("%w: %s", ErrConfigDoesNotExist, cfgName)
		}

		flagsUnderSameCfg := flag.NewFlagSet("", flag.ContinueOnError)
		for _, flagSet := range flagSets {
			flagSet.VisitAll(func(f *flag.Flag) {
				flagsUnderSameCfg.AddFlag(f)
			})
		}
		fs[cfgName] = flagsUnderSameCfg
	}
	return fs, nil
}

// parses the configuration and initializes the global logger.
func loadCfg(flagSets map[string]*flag.FlagSet) error {
	if err := nodeConfig.LoadFile(*nodeCfgFilePath); err != nil {
		if hasFlag(flag.CommandLine, CfgConfigFilePathNodeConfig) {
			// if a file was explicitly specified, raise the error
			return err
		}
		fmt.Printf("No config file found via '%s'. Loading default settings.", *nodeCfgFilePath)
	}

	if err := peeringConfig.LoadFile(*peeringCfgFilePath); err != nil {
		if hasFlag(flag.CommandLine, CfgConfigFilePathPeeringConfig) {
			// if a file was explicitly specified, raise the error
			return err
		}
		fmt.Printf("No peering config file found via '%s'. Loading default settings.", *peeringCfgFilePath)
	}

	if err := profileConfig.LoadFile(*profilesCfgFilePath); err != nil {
		if hasFlag(flag.CommandLine, CfgConfigFilePathProfilesConfig) {
			// if a file was explicitly specified, raise the error
			return err
		}
		fmt.Printf("No profiles config file found via '%s'. Loading default settings.", *profilesCfgFilePath)
	}

	// load the flags to set the default values
	if err := nodeConfig.LoadFlagSet(flagSets["nodeConfig"]); err != nil {
		return err
	}

	if err := peeringConfig.LoadFlagSet(flagSets["peeringConfig"]); err != nil {
		return err
	}

	// load the env vars after default values from flags were set (otherwise the env vars are not added because the keys don't exist)
	if err := nodeConfig.LoadEnvironmentVars(""); err != nil {
		return err
	}

	// load the flags again to overwrite env vars that were also set via command line
	if err := nodeConfig.LoadFlagSet(flagSets["nodeConfig"]); err != nil {
		return err
	}

	if err := peeringConfig.LoadFlagSet(flagSets["peeringConfig"]); err != nil {
		return err
	}

	return nil
}

func hasFlag(flagSet *flag.FlagSet, name string) bool {
	has := false
	flagSet.Visit(func(f *flag.Flag) {
		if f.Name == name {
			has = true
		}
	})
	return has
}

// prints the loaded configuration, but hides sensitive information.
func printConfig(maskedKeys []string) {
	nodeConfig.Print(maskedKeys)

	enablePlugins := nodeConfig.Strings(CfgNodeEnablePlugins)
	disablePlugins := nodeConfig.Strings(CfgNodeDisablePlugins)

	if len(enablePlugins) > 0 || len(disablePlugins) > 0 {
		if len(enablePlugins) > 0 {
			fmt.Printf("\nThe following plugins are enabled: %s\n", getList(enablePlugins))
		}
		if len(disablePlugins) > 0 {
			fmt.Printf("\nThe following plugins are disabled: %s\n", getList(disablePlugins))
		}
		fmt.Println()
	}
}

// adds the given flag sets to flag.CommandLine and then parses them.
func parseFlags(flagSets map[string]*flag.FlagSet) {
	for _, flagSet := range flagSets {
		flag.CommandLine.AddFlagSet(flagSet)
	}
	flag.Parse()
}

// HideConfigFlags hides all non essential flags from the help/usage text.
func hideConfigFlags(flagSets map[string]*flag.FlagSet) {
	flag.VisitAll(func(f *flag.Flag) {
		_, notHidden := nonHiddenFlag[f.Name]
		f.Hidden = !notHidden
	})
	flagSets["nodeConfig"].VisitAll(func(f *flag.Flag) {
		_, notHidden := nonHiddenFlag[f.Name]
		f.Hidden = !notHidden
	})
	flagSets["peeringConfig"].VisitAll(func(f *flag.Flag) {
		_, notHidden := nonHiddenFlag[f.Name]
		f.Hidden = !notHidden
	})
}

// prints out the version of this node.
func printVersion(flagSets map[string]*flag.FlagSet) {
	if *version {
		fmt.Println(Name + " " + Version)
		os.Exit(0)
	}

	if *help {
		if !*helpFull {
			// hides all non essential flags from the help/usage text.
			hideConfigFlags(flagSets)
		}
		flag.Usage()
		os.Exit(0)
	}
}
