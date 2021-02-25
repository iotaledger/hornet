package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"sync"

	"github.com/gohornet/hornet/integration-tests/mock/pkg/hexutil"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"
)

// EnvConfigFileName denotes the name of the environment variable for the config file name.
const EnvConfigFileName = "WHITE_FLAG_MOCK_CONFIG"
const defaultConfigFileName = "config.json"

var config = &Config{}
var configOnce sync.Once

// GetConfig gets the config and loads it from disk if non yet loaded.
func GetConfig(overrideCfgFile ...string) *Config {
	configOnce.Do(func() {
		var cfgFileName string
		// environment var gets priority
		cfgFileName = os.Getenv(EnvConfigFileName)
		if len(cfgFileName) == 0 {
			if len(overrideCfgFile) > 0 {
				cfgFileName = overrideCfgFile[0]
			} else {
				cfgFileName = defaultConfigFileName
			}
		}
		configFileBytes, err := ioutil.ReadFile(cfgFileName)
		if err != nil {
			log.Fatalf("can't read config file: %s", err)
		}
		if err := json.Unmarshal(configFileBytes, config); err != nil {
			log.Fatalf("can't unmarshal config: %s", err)
		}
		log.Println("config file successfully loaded")
	})
	return config
}

// Config holds the configuration of the backend tool.
type Config struct {
	HTTP        HTTPConfig        `json:"http"`
	Coordinator CoordinatorConfig `json:"coordinator"`
	WhiteFlag   WhiteFlagConfig   `json:"white_flag"`
}

// HTTPConfig holds the HTTP server configuration.
type HTTPConfig struct {
	BindAddress string `json:"bind_address"` // address to which the HTTP server binds to.
}

// CoordinatorConfig holds the configuration for the mock coordinator.
type CoordinatorConfig struct {
	Seed      trinary.Trytes       `json:"seed"`       // seed of the coordinator
	Security  consts.SecurityLevel `json:"security"`   // used security level
	TreeDepth int                  `json:"tree_depth"` // the depth of the Merkle tree
	MWM       int                  `json:"mwm"`        // PoW MWM used by the coordinator
}

// WhiteFlagConfig holds information about the white flag confirmations to be mocked.
type WhiteFlagConfig struct {
	Seed       trinary.Trytes         `json:"seed"`       // seed which is used to generate all migration signatures
	Migrations map[uint32][]Migration `json:"migrations"` // migration bundles per milestone index
}

// Migration holds information about a single migration bundle.
type Migration struct {
	// input balance
	// must be at least 1.000.000
	Balance uint64 `json:"balance"`
	// key index of the input
	// used to generate the address and corresponding signature
	Index uint64 `json:"index"`
	// security level of of the input
	Security consts.SecurityLevel `json:"security"`
	// hex encoded 32-byte Ed25519 address
	// used to generate the output migration address
	// random address can be generated using `openssl rand -hex 32`
	Ed25519Address hexutil.Bytes `json:"ed25519_address"`
}
