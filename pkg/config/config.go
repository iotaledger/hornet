package config

import (
	"fmt"
	"os"
	"strings"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/iotaledger/hive.go/parameter"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	// default
	defaultConfigName         = "config"
	defaultPeeringConfigName  = "peering"
	defaultProfilesConfigName = "profiles"

	// FlagSets
	configFlagSet  = flag.NewFlagSet("", flag.ContinueOnError)
	peeringFlagSet = flag.NewFlagSet("", flag.ContinueOnError)

	// flags
	configName         = flag.StringP("config", "c", defaultConfigName, "Filename of the config file without the file extension")
	peeringConfigName  = flag.StringP("peeringConfig", "n", defaultPeeringConfigName, "Filename of the peering config file without the file extension")
	profilesConfigName = flag.String("profilesConfig", defaultProfilesConfigName, "Filename of the profiles config file without the file extension")
	configDirPath      = flag.StringP("config-dir", "d", ".", "Path to the directory containing the config file")

	// Viper
	NodeConfig     = viper.New()
	PeeringConfig  = viper.New()
	ProfilesConfig = viper.New()

	peeringConfigHotReloadAllowed = true
	peeringConfigHotReloadLock    syncutils.Mutex

	// a list of flags which should be printed via --help
	nonHiddenFlags = map[string]struct{}{
		"config":              {},
		"config-dir":          {},
		"node.disablePlugins": {},
		"node.enablePlugins":  {},
		"overwriteCooAddress": {},
		"peeringConfig":       {},
		"profilesConfig":      {},
		"useProfile":          {},
		"version":             {},
	}
)

// HideConfigFlags hides all non essential flags from the help/usage text.
func HideConfigFlags() {
	flag.VisitAll(func(f *flag.Flag) {
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

// FetchConfig fetches config values from a dir defined via CLI flag --config-dir (or the current working dir if not set).
//
// It automatically reads in a single config file starting with "config" (can be changed via the --config CLI flag)
// and ending with: .json, .toml, .yaml or .yml (in this sequence).
func FetchConfig() error {

	// replace dots with underscores in env
	dotReplacer := strings.NewReplacer(".", "_")
	NodeConfig.SetEnvKeyReplacer(dotReplacer)
	PeeringConfig.SetEnvKeyReplacer(dotReplacer)
	ProfilesConfig.SetEnvKeyReplacer(dotReplacer)

	// ensure that envs are read in too
	NodeConfig.AutomaticEnv()
	PeeringConfig.AutomaticEnv()
	ProfilesConfig.AutomaticEnv()

	err := parameter.LoadConfigFile(NodeConfig, *configDirPath, *configName, configFlagSet, !hasFlag(defaultConfigName), true)
	if err != nil {
		return err
	}

	err = parameter.LoadConfigFile(PeeringConfig, *configDirPath, *peeringConfigName, peeringFlagSet, !hasFlag(defaultPeeringConfigName), true)
	if err != nil {
		return err
	}

	err = parameter.LoadConfigFile(ProfilesConfig, *configDirPath, *profilesConfigName, nil, !hasFlag(defaultProfilesConfigName), true)
	if err != nil {
		return err
	}

	return nil
}

func PrintConfig(ignoreSettingsAtPrint ...[]string) {
	parameter.PrintConfig(NodeConfig, ignoreSettingsAtPrint...)
	fmt.Println(CfgPeers, PeeringConfig.GetStringSlice(CfgPeers))
	fmt.Println(CfgPeeringMaxPeers, PeeringConfig.GetStringSlice(CfgPeeringMaxPeers))
	fmt.Println(CfgPeeringAcceptAnyConnection, PeeringConfig.GetStringSlice(CfgPeeringAcceptAnyConnection))
	parameter.PrintConfig(ProfilesConfig)
}

func AllowPeeringConfigHotReload() {
	peeringConfigHotReloadLock.Lock()
	defer peeringConfigHotReloadLock.Unlock()
	peeringConfigHotReloadAllowed = true
}

func DenyPeeringConfigHotReload() {
	peeringConfigHotReloadLock.Lock()
	defer peeringConfigHotReloadLock.Unlock()
	peeringConfigHotReloadAllowed = false
}

func AcquirePeeringConfigHotReload() bool {
	peeringConfigHotReloadLock.Lock()
	defer peeringConfigHotReloadLock.Unlock()

	if !peeringConfigHotReloadAllowed {
		// It is already denied
		return false
	}

	// Deny it for other calls
	peeringConfigHotReloadAllowed = false
	return true
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

// LoadHashFromEnvironment loads a hash from the given environment variable.
func LoadHashFromEnvironment(name string, length ...int) (trinary.Hash, error) {

	hash, exists := os.LookupEnv(name)
	if !exists {
		return "", fmt.Errorf("environment variable '%s' not set", name)
	}

	if len(hash) == 0 {
		return "", fmt.Errorf("environment variable '%s' not set", name)
	}

	hashLength := consts.HashTrytesSize
	if len(length) > 0 {
		hashLength = length[0]
	}

	if !guards.IsTrytesOfExactLength(hash, hashLength) {
		return "", fmt.Errorf("environment variable '%s' contains an invalid hash", name)
	}

	return hash, nil
}
