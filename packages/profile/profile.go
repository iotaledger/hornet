package profile

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2/options"
	"github.com/shirou/gopsutil/mem"

	"github.com/gohornet/hornet/packages/parameter"
)

var (
	once    = sync.Once{}
	profile *Profile

	ErrNotEnoughMemory = errors.New("Not enough system memory")
)

func GetProfile() *Profile {
	once.Do(func() {
		profileName := parameter.NodeConfig.GetString("useProfile")
		if profileName == "auto" {
			v, err := mem.VirtualMemory()
			if err != nil {
				panic(err)
			}

			if v.Total >= 8000000000 {
				profileName = "8gb"
			} else if v.Total >= 4000000000 {
				profileName = "4gb"
			} else if v.Total >= 2000000000 {
				profileName = "2gb"
			} else if v.Total >= 1000000000 {
				profileName = "1gb"
			} else {
				panic(ErrNotEnoughMemory)
			}
		}

		switch profileName {
		case "8gb":
			profile = Profile8GB
			profile.Name = "8gb"
		case "4gb":
			profile = Profile4GB
			profile.Name = "4gb"
		case "2gb":
			profile = Profile2GB
			profile.Name = "2gb"
		case "1gb", "light":
			profile = Profile1GB
			profile.Name = "1gb"
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

var Profile8GB = &Profile{
	Caches: Caches{
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
		BloomFalsePositive:      0.01,
		BlockSize:               4 * 1024,
		SyncWrites:              false,
		NumVersionsToKeep:       1,
		CompactLevel0OnClose:    false,
		KeepL0InMemory:          false,
		VerifyValueChecksum:     false,
		MaxCacheSize:            50000000,
		ZSTDCompressionLevel:    10,
		ValueLogFileSize:        1073741823,
		ValueLogMaxEntries:      1000000,
		ValueThreshold:          32,
		WithTruncate:            false,
		LogRotatesToFlush:       2,
		EventLogging:            false,
	},
}

var Profile4GB = &Profile{
	Caches: Caches{
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
		TableLoadingMode:        options.FileIO,
		ValueLogLoadingMode:     options.FileIO,
		MaxLevels:               7,
		MaxTableSize:            67108864,
		NumCompactors:           2,
		NumLevelZeroTables:      5,
		NumLevelZeroTablesStall: 10,
		NumMemtables:            5,
		BloomFalsePositive:      0.01,
		BlockSize:               4 * 1024,
		SyncWrites:              false,
		NumVersionsToKeep:       1,
		CompactLevel0OnClose:    false,
		KeepL0InMemory:          false,
		VerifyValueChecksum:     false,
		MaxCacheSize:            50000000,
		ZSTDCompressionLevel:    10,
		ValueLogFileSize:        1073741823,
		ValueLogMaxEntries:      1000000,
		ValueThreshold:          32,
		WithTruncate:            false,
		LogRotatesToFlush:       2,
		EventLogging:            false,
	},
}

var Profile2GB = &Profile{
	Caches: Caches{
		Bundles: CacheOpts{
			Size:         10000,
			EvictionSize: 1000,
		},
		Milestones: CacheOpts{
			Size:         1000,
			EvictionSize: 100,
		},
		SpentAddresses: CacheOpts{
			Size:         2000,
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
		TableLoadingMode:        options.FileIO,
		ValueLogLoadingMode:     options.FileIO,
		MaxLevels:               7,
		MaxTableSize:            67108864,
		NumCompactors:           2,
		NumLevelZeroTables:      5,
		NumLevelZeroTablesStall: 10,
		NumMemtables:            5,
		BloomFalsePositive:      0.01,
		BlockSize:               4 * 1024,
		SyncWrites:              false,
		NumVersionsToKeep:       1,
		CompactLevel0OnClose:    false,
		KeepL0InMemory:          false,
		VerifyValueChecksum:     false,
		MaxCacheSize:            50000000,
		ZSTDCompressionLevel:    10,
		ValueLogFileSize:        1073741823,
		ValueLogMaxEntries:      1000000,
		ValueThreshold:          32,
		WithTruncate:            false,
		LogRotatesToFlush:       2,
		EventLogging:            false,
	},
}

var Profile1GB = &Profile{
	Caches: Caches{
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
		BloomFalsePositive:      0.01,
		BlockSize:               4 * 1024,
		SyncWrites:              false,
		NumVersionsToKeep:       1,
		CompactLevel0OnClose:    false,
		KeepL0InMemory:          false,
		VerifyValueChecksum:     false,
		MaxCacheSize:            50000000,
		ZSTDCompressionLevel:    10,
		ValueLogFileSize:        33554431,
		ValueLogMaxEntries:      250000,
		ValueThreshold:          32,
		WithTruncate:            false,
		LogRotatesToFlush:       2,
		EventLogging:            false,
	},
}

type Profile struct {
	Name   string     `json:"name"`
	Caches Caches     `json:"caches"`
	Badger BadgerOpts `json:"badger"`
}

type Caches struct {
	Bundles                   CacheOpts `json:"bundles"`
	Milestones                CacheOpts `json:"milestones"`
	SpentAddresses            CacheOpts `json:"spentAddresses"`
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
	BloomFalsePositive      float64                 `json:"bloomFalsePositive"`
	BlockSize               int                     `json:"blockSize"`
	SyncWrites              bool                    `json:"syncWrites"`
	NumVersionsToKeep       int                     `json:"numVersionsToKeep"`
	CompactLevel0OnClose    bool                    `json:"compactLevel0OnClose"`
	KeepL0InMemory          bool                    `json:"keepL0InMemory"`
	VerifyValueChecksum     bool                    `json:"verifyValueChecksum"`
	MaxCacheSize            int64                   `json:"maxCacheSize"`
	ZSTDCompressionLevel    int                     `json:"ZSTDCompressionLevel"`
	ValueLogFileSize        int64                   `json:"valueLogFileSize"`
	ValueLogMaxEntries      uint32                  `json:"valueLogMaxEntries"`
	ValueThreshold          int                     `json:"valueThreshold"`
	WithTruncate            bool                    `json:"withTruncate"`
	LogRotatesToFlush       int32                   `json:"logRotatesToFlush"`
	EventLogging            bool                    `json:"eventLogging"`
}
