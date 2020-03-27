package config

import (
	"sync"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/iotaledger/hive.go/parameter"
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
	peeringConfigHotReloadLock    sync.Mutex
)

// FetchConfig fetches config values from a dir defined via CLI flag --config-dir (or the current working dir if not set).
//
// It automatically reads in a single config file starting with "config" (can be changed via the --config CLI flag)
// and ending with: .json, .toml, .yaml or .yml (in this sequence).
func FetchConfig(printConfig bool, ignoreSettingsAtPrint ...[]string) error {
	err := parameter.LoadConfigFile(NodeConfig, *configDirPath, *configName, true, !hasFlag(defaultConfigName))
	if err != nil {
		return err
	}
	parameter.PrintConfig(NodeConfig, ignoreSettingsAtPrint...)

	err = parameter.LoadConfigFile(PeeringConfig, *configDirPath, *peeringConfigName, false, !hasFlag(defaultPeeringConfigName))
	if err != nil {
		return err
	}
	parameter.PrintConfig(PeeringConfig)

	err = parameter.LoadConfigFile(ProfilesConfig, *configDirPath, *profilesConfigName, false, !hasFlag(defaultProfilesConfigName))
	if err != nil {
		return err
	}
	parameter.PrintConfig(ProfilesConfig)

	return nil
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
		return false
	}

	peeringConfigHotReloadAllowed = false
	return peeringConfigHotReloadAllowed
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
