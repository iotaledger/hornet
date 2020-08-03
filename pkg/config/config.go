package config

import (
	"fmt"
	"strings"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/iotaledger/hive.go/parameter"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	// default
	defaultConfigName         = "config"
	defaultPeeringConfigName  = "peering"
	defaultProfilesConfigName = "profiles"

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

	err := parameter.LoadConfigFile(NodeConfig, *configDirPath, *configName, true, !hasFlag(defaultConfigName))
	if err != nil {
		return err
	}

	err = parameter.LoadConfigFile(PeeringConfig, *configDirPath, *peeringConfigName, true, !hasFlag(defaultPeeringConfigName))
	if err != nil {
		return err
	}

	err = parameter.LoadConfigFile(ProfilesConfig, *configDirPath, *profilesConfigName, false, !hasFlag(defaultProfilesConfigName))
	if err != nil {
		return err
	}

	return nil
}

func PrintConfig(ignoreSettingsAtPrint ...[]string) {
	parameter.PrintConfig(NodeConfig, ignoreSettingsAtPrint...)
	parameter.PrintConfig(PeeringConfig)
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
func LoadHashFromEnvironment(name string) (trinary.Hash, error) {
	viper.BindEnv(name)
	hash := viper.GetString(name)

	if len(hash) == 0 {
		return "", fmt.Errorf("environment variable '%s' not set", name)
	}

	if !guards.IsTransactionHash(hash) {
		return "", fmt.Errorf("environment variable '%s' contains an invalid hash", name)
	}

	return hash, nil
}
