package config

import (
	"fmt"

	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/configuration"
)

var (
	// default
	defaultConfigName         = "config.json"
	defaultPeeringConfigName  = "peering.json"
	defaultProfilesConfigName = "profiles.json"

	// FlagSets
	configFlagSet  = flag.NewFlagSet("", flag.ContinueOnError)
	peeringFlagSet = flag.NewFlagSet("", flag.ContinueOnError)

	// flags
	configFilePath   = flag.StringP("config", "c", defaultConfigName, "file path of the config file")
	peeringFilePath  = flag.StringP("peeringConfig", "n", defaultPeeringConfigName, "file path of the peering config file")
	profilesFilePath = flag.String("profilesConfig", defaultProfilesConfigName, "file path of the profiles config file")

	// Configurations
	NodeConfig     = configuration.New()
	PeeringConfig  = configuration.New()
	ProfilesConfig = configuration.New()

	// a list of flags which should be printed via --help
	nonHiddenFlags = map[string]struct{}{
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
)

// HideConfigFlags hides all non essential flags from the help/usage text.
func HideConfigFlags() {
	flag.VisitAll(func(f *flag.Flag) {
		_, notHidden := nonHiddenFlags[f.Name]
		f.Hidden = !notHidden
	})
	configFlagSet.VisitAll(func(f *flag.Flag) {
		_, notHidden := nonHiddenFlags[f.Name]
		f.Hidden = !notHidden
	})
	peeringFlagSet.VisitAll(func(f *flag.Flag) {
		_, notHidden := nonHiddenFlags[f.Name]
		f.Hidden = !notHidden
	})
}

// ParseFlags defines and parses the command-line flags from os.Args[1:].
func ParseFlags() {
	flag.CommandLine.AddFlagSet(configFlagSet)
	flag.CommandLine.AddFlagSet(peeringFlagSet)
	flag.Parse()
}

// FetchConfig fetches all config values (order: default, files, env, flags).
func FetchConfig() error {

	if err := NodeConfig.LoadFile(*configFilePath); err != nil {
		if hasFlag(defaultConfigName) {
			// if a file was explicitly specified, raise the error
			return err
		}
		fmt.Printf("No config file found via '%s'. Loading default settings.", *configFilePath)
	}

	if err := PeeringConfig.LoadFile(*peeringFilePath); err != nil {
		if hasFlag(defaultPeeringConfigName) {
			// if a file was explicitly specified, raise the error
			return err
		}
		fmt.Printf("No peering config file found via '%s'. Loading default settings.", *peeringFilePath)
	}

	if err := ProfilesConfig.LoadFile(*profilesFilePath); err != nil {
		if hasFlag(defaultProfilesConfigName) {
			// if a file was explicitly specified, raise the error
			return err
		}
		fmt.Printf("No profiles config file found via '%s'. Loading default settings.", *profilesFilePath)
	}

	// load the flags to set the default values
	if err := NodeConfig.LoadFlagSet(configFlagSet); err != nil {
		return err
	}

	if err := PeeringConfig.LoadFlagSet(peeringFlagSet); err != nil {
		return err
	}

	// load the env vars after default values from flags were set (otherwise the env vars are not added because the keys don't exist)
	if err := NodeConfig.LoadEnvironmentVars(""); err != nil {
		return err
	}

	// load the flags again to overwrite env vars that were also set via command line
	if err := NodeConfig.LoadFlagSet(configFlagSet); err != nil {
		return err
	}

	if err := PeeringConfig.LoadFlagSet(peeringFlagSet); err != nil {
		return err
	}

	return nil
}

func PrintConfig(ignoreSettingsAtPrint ...[]string) {
	NodeConfig.Print(ignoreSettingsAtPrint...)
}

func hasFlag(name string) bool {
	has := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			has = true
		}
	})
	return has
}
