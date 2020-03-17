package config

import (
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/iotaledger/hive.go/parameter"
	"github.com/iotaledger/hive.go/syncutils"
)

var (
	// default
	defaultConfigName          = "config"
	defaultNeighborsConfigName = "neighbors"
	defaultProfilesConfigName  = "profiles"

	// flags
	configName          = flag.StringP("config", "c", defaultConfigName, "Filename of the config file without the file extension")
	neighborsConfigName = flag.StringP("neighborsConfig", "n", defaultNeighborsConfigName, "Filename of the neighbors config file without the file extension")
	profilesConfigName  = flag.String("profilesConfig", defaultProfilesConfigName, "Filename of the profiles config file without the file extension")
	configDirPath       = flag.StringP("config-dir", "d", ".", "Path to the directory containing the config file")

	// Viper
	NodeConfig      = viper.New()
	NeighborsConfig = viper.New()
	ProfilesConfig  = viper.New()

	neighborsConfigHotReloadAllowed = true
	neighborsConfigHotReloadLock    syncutils.RWMutex
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

	err = parameter.LoadConfigFile(NeighborsConfig, *configDirPath, *neighborsConfigName, false, !hasFlag(defaultNeighborsConfigName))
	if err != nil {
		return err
	}
	parameter.PrintConfig(NeighborsConfig)

	err = parameter.LoadConfigFile(ProfilesConfig, *configDirPath, *profilesConfigName, false, !hasFlag(defaultProfilesConfigName))
	if err != nil {
		return err
	}
	parameter.PrintConfig(ProfilesConfig)

	return nil
}

func AllowNeighborsConfigHotReload() {
	neighborsConfigHotReloadLock.Lock()
	defer neighborsConfigHotReloadLock.Unlock()
	neighborsConfigHotReloadAllowed = true
}

func DenyNeighborsConfigHotReload() {
	neighborsConfigHotReloadLock.Lock()
	defer neighborsConfigHotReloadLock.Unlock()
	neighborsConfigHotReloadAllowed = false
}

func IsNeighborsConfigHotReloadAllowed() bool {
	neighborsConfigHotReloadLock.RLock()
	defer neighborsConfigHotReloadLock.RUnlock()
	return neighborsConfigHotReloadAllowed
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
