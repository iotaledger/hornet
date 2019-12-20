package parameter

import (
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/parameter"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	log = logger.NewLogger("NodeConfig")

	// flags
	configName          = flag.StringP("config", "c", "config", "Filename of the config file without the file extension")
	neighborsConfigName = flag.StringP("neighborsConfig", "n", "neighbors", "Filename of the neighbors config file without the file extension")
	configDirPath       = flag.StringP("config-dir", "d", ".", "Path to the directory containing the config file")

	// Viper
	NodeConfig     = viper.New()
	NeighborConfig = viper.New()
)

// FetchConfig fetches config values from a dir defined via CLI flag --config-dir (or the current working dir if not set).
//
// It automatically reads in a single config file starting with "config" (can be changed via the --config CLI flag)
// and ending with: .json, .toml, .yaml or .yml (in this sequence).
func FetchConfig(printConfig bool, ignoreSettingsAtPrint ...[]string) error {
	// flags
	configName := flag.StringP("config", "c", "config", "Filename of the config file without the file extension")
	configDirPath := flag.StringP("config-dir", "d", ".", "Path to the directory containing the config file")

	config, err := parameter.LoadConfigFile(*configDirPath, *configName)
	if err != nil {
		return err
	}

	parameter.PrintConfig(config, ignoreSettingsAtPrint...)

	NodeConfig = config

	return nil
}
