package profile

import (
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2/options"
	"github.com/iotaledger/hive.go/parameter"
)

var once = sync.Once{}
var profile *Profile

func GetProfile() *Profile {
	once.Do(func() {
		profileName := parameter.NodeConfig.GetString("useProfile")
		switch profileName {
		case "default":
			profile = DefaultProfile
			profile.Name = "default"
		case "light":
			profile = LightProfile
			profile.Name = "light"
		default:
			p := &Profile{}
			key := fmt.Sprintf("profiles.%s", profileName)
			if !parameter.NodeConfig.IsSet(key) {
				panic(fmt.Sprintf("profile '%s' is not defined in the config", profileName))
			}
			if err := parameter.NodeConfig.UnmarshalKey(key, p); err != nil {
				panic(err)
			}
			p.Name = profileName
			profile = p
		}
	})
	return profile
}

var DefaultProfile = &Profile{
	Caches: Caches{
		RequestQueue: CacheOpts{
			Size: 100000,
		},
		Approvers: CacheOpts{
			Size:         100000,
			EvictionSize: 1000,
		},
		Bundles: CacheOpts{
			Size:         20000,
			EvictionSize: 1000,
		},
		Milestones: CacheOpts{
			Size:         1000,
			EvictionSize: 100,
		},
		SpentAddresses: CacheOpts{
			Size:         5000,
			EvictionSize: 1000,
		},
		Transactions: CacheOpts{
			Size:         50000,
			EvictionSize: 1000,
		},
		IncomingTransactionFilter: CacheOpts{
			Size: 5000,
		},
		RefsInvalidBundle: CacheOpts{
			Size: 10000,
		},
	},
	Badger: BadgerOpts{
		LevelOneSize:            268435456,
		LevelSizeMultiplier:     10,
		TableLoadingMode:        options.MemoryMap,
		ValueLogLoadingMode:     options.MemoryMap,
		MaxLevels:               7,
		MaxTableSize:            67108864,
		NumCompactors:           2,
		NumLevelZeroTables:      5,
		NumLevelZeroTablesStall: 10,
		NumMemtables:            5,
		NumVersionsToKeep:       1,
		SyncWrites:              true,
		CompactLevel0OnClose:    true,
		ValueLogFileSize:        1073741823,
		ValueLogMaxEntries:      1000000,
		ValueThreshold:          32,
		LogRotatesToFlush:       2,
		MaxCacheSize:            50000000,
	},
}

var LightProfile = &Profile{
	Caches: Caches{
		RequestQueue: CacheOpts{
			Size: 100000,
		},
		Approvers: CacheOpts{
			Size:         10000,
			EvictionSize: 1000,
		},
		Bundles: CacheOpts{
			Size:         5000,
			EvictionSize: 1000,
		},
		Milestones: CacheOpts{
			Size:         150,
			EvictionSize: 100,
		},
		SpentAddresses: CacheOpts{
			Size:         2000,
			EvictionSize: 1000,
		},
		Transactions: CacheOpts{
			Size:         10000,
			EvictionSize: 1000,
		},
		IncomingTransactionFilter: CacheOpts{
			Size: 5000,
		},
		RefsInvalidBundle: CacheOpts{
			Size: 10000,
		},
	},
	Badger: BadgerOpts{
		LevelOneSize:            67108864,
		LevelSizeMultiplier:     10,
		TableLoadingMode:        options.FileIO,
		ValueLogLoadingMode:     options.FileIO,
		MaxLevels:               5,
		MaxTableSize:            16777216,
		NumCompactors:           1,
		NumLevelZeroTables:      1,
		NumLevelZeroTablesStall: 2,
		NumMemtables:            1,
		NumVersionsToKeep:       1,
		SyncWrites:              false,
		CompactLevel0OnClose:    true,
		ValueLogFileSize:        33554431,
		ValueLogMaxEntries:      250000,
		ValueThreshold:          32,
		LogRotatesToFlush:       2,
		MaxCacheSize:            50000000,
	},
}

type Profile struct {
	Name   string     `json:"name"`
	Caches Caches     `json:"caches"`
	Badger BadgerOpts `json:"badger"`
}

type Caches struct {
	RequestQueue              CacheOpts `json:"requestQueue"`
	Approvers                 CacheOpts `json:"approvers"`
	Bundles                   CacheOpts `json:"bundles"`
	Milestones                CacheOpts `json:"milestones"`
	SpentAddresses            CacheOpts `json:"spentAddresses"`
	Transactions              CacheOpts `json:"transactions"`
	IncomingTransactionFilter CacheOpts `json:"incomingTransactionFilter"`
	RefsInvalidBundle         CacheOpts `json:"refsInvalidBundle"`
}

type CacheOpts struct {
	Size         int    `json:"size"`
	EvictionSize uint64 `json:"evictionSize"`
}

type BadgerOpts struct {
	LevelOneSize            int64                   `json:"levelOneSize"`
	LevelSizeMultiplier     int                     `json:"levelSizeMultiplier"`
	TableLoadingMode        options.FileLoadingMode `json:"tableLoadingMode"`
	ValueLogLoadingMode     options.FileLoadingMode `json:"valueLogLoadingMode"`
	MaxLevels               int                     `json:"maxLevels"`
	MaxTableSize            int64                   `json:"maxTableSize"`
	NumCompactors           int                     `json:"numCompactors"`
	NumLevelZeroTables      int                     `json:"numLevelZeroTables"`
	NumLevelZeroTablesStall int                     `json:"numLevelZeroTablesStall"`
	NumMemtables            int                     `json:"numMemtables"`
	NumVersionsToKeep       int                     `json:"numVersionsToKeep"`
	SyncWrites              bool                    `json:"syncWrites"`
	CompactLevel0OnClose    bool                    `json:"compactLevel0OnClose"`
	ValueLogFileSize        int64                   `json:"valueLogFileSize"`
	ValueLogMaxEntries      uint32                  `json:"valueLogMaxEntries"`
	ValueThreshold          int                     `json:"valueThreshold"`
	LogRotatesToFlush       int32                   `json:"logRotatesToFlush"`
	MaxCacheSize            int64                   `json:"maxCacheSize"`
}
