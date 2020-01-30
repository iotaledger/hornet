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

			if v.Total >= 8000000000*0.95 {
				profileName = "8gb"
			} else if v.Total >= 4000000000*0.95 {
				profileName = "4gb"
			} else if v.Total >= 2000000000*0.95 {
				profileName = "2gb"
			} else if v.Total >= 1000000000*0.95 {
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
		Approvers: CacheOpts{
			CacheTimeMs: 60000,
		},
		Bundles: CacheOpts{
			CacheTimeMs: 60000,
		},
		Milestones: CacheOpts{
			CacheTimeMs: 60000,
		},
		Transactions: CacheOpts{
			CacheTimeMs: 60000,
		},
		IncomingTransactionFilter: CacheOpts{
			CacheTimeMs: 5000,
		},
		RefsInvalidBundle: CacheOpts{
			CacheTimeMs: 180000,
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
		ZSTDCompressionLevel:    1,
		CompressionType:         options.None,
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
		Approvers: CacheOpts{
			CacheTimeMs: 30000,
		},
		Bundles: CacheOpts{
			CacheTimeMs: 30000,
		},
		Milestones: CacheOpts{
			CacheTimeMs: 30000,
		},
		Transactions: CacheOpts{
			CacheTimeMs: 30000,
		},
		IncomingTransactionFilter: CacheOpts{
			CacheTimeMs: 5000,
		},
		RefsInvalidBundle: CacheOpts{
			CacheTimeMs: 180000,
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
		ZSTDCompressionLevel:    1,
		CompressionType:         options.None,
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
		Approvers: CacheOpts{
			CacheTimeMs: 5000,
		},
		Bundles: CacheOpts{
			CacheTimeMs: 5000,
		},
		Milestones: CacheOpts{
			CacheTimeMs: 5000,
		},
		Transactions: CacheOpts{
			CacheTimeMs: 5000,
		},
		IncomingTransactionFilter: CacheOpts{
			CacheTimeMs: 2500,
		},
		RefsInvalidBundle: CacheOpts{
			CacheTimeMs: 180000,
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
		ZSTDCompressionLevel:    1,
		CompressionType:         options.None,
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
		Approvers: CacheOpts{
			CacheTimeMs: 1500,
		},
		Bundles: CacheOpts{
			CacheTimeMs: 1500,
		},
		Milestones: CacheOpts{
			CacheTimeMs: 1500,
		},
		Transactions: CacheOpts{
			CacheTimeMs: 1500,
		},
		IncomingTransactionFilter: CacheOpts{
			CacheTimeMs: 1500,
		},
		RefsInvalidBundle: CacheOpts{
			CacheTimeMs: 180000,
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
		ZSTDCompressionLevel:    1,
		CompressionType:         options.None,
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
	Approvers                 CacheOpts `json:"approvers"`
	Milestones                CacheOpts `json:"milestones"`
	Transactions              CacheOpts `json:"transactions"`
	IncomingTransactionFilter CacheOpts `json:"incomingTransactionFilter"`
	RefsInvalidBundle         CacheOpts `json:"refsInvalidBundle"`
}

type CacheOpts struct {
	CacheTimeMs  uint64 `json:"cacheTimeMs"`
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
	CompressionType         options.CompressionType `json:"CompressionType"`
	ValueLogFileSize        int64                   `json:"valueLogFileSize"`
	ValueLogMaxEntries      uint32                  `json:"valueLogMaxEntries"`
	ValueThreshold          int                     `json:"valueThreshold"`
	WithTruncate            bool                    `json:"withTruncate"`
	LogRotatesToFlush       int32                   `json:"logRotatesToFlush"`
	EventLogging            bool                    `json:"eventLogging"`
}
