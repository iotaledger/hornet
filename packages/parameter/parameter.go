package parameter

import (
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/parameter"
)

var (
	log = logger.NewLogger("NodeConfig")

	// flags
	configName          = flag.StringP("config", "c", "config", "Filename of the config file without the file extension")
	neighborsConfigName = flag.StringP("neighborsConfig", "n", "neighbors", "Filename of the neighbors config file without the file extension")
	configDirPath       = flag.StringP("config-dir", "d", ".", "Path to the directory containing the config file")

	// Viper
	NodeConfig      = viper.New()
	NeighborsConfig = viper.New()
)

// FetchConfig fetches config values from a dir defined via CLI flag --config-dir (or the current working dir if not set).
//
// It automatically reads in a single config file starting with "config" (can be changed via the --config CLI flag)
// and ending with: .json, .toml, .yaml or .yml (in this sequence).
func FetchConfig(printConfig bool, ignoreSettingsAtPrint ...[]string) error {
	// flags

	err := parameter.LoadConfigFile(NodeConfig, *configDirPath, *configName, true, false)
	if err != nil {
		return err
	}

	parameter.PrintConfig(NodeConfig, ignoreSettingsAtPrint...)

	err = parameter.LoadConfigFile(NeighborsConfig, *configDirPath, *neighborsConfigName, false, false)
	if err != nil {
		return err
	}

	parameter.PrintConfig(NeighborsConfig, ignoreSettingsAtPrint...)

	return nil
}
