package profile

import (
	"errors"
	"fmt"
	"strings"
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
		profileName := strings.ToLower(parameter.NodeConfig.GetString("useProfile"))
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
			if !parameter.ProfilesConfig.IsSet(profileName) {
				panic(fmt.Sprintf("profile '%s' is not defined in the config", profileName))
			}
			if err := parameter.ProfilesConfig.UnmarshalKey(profileName, p); err != nil {
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
		Addresses: CacheOpts{
			CacheTimeMs: 60000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Approvers: CacheOpts{
			CacheTimeMs: 60000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Tags: CacheOpts{
			CacheTimeMs: 60000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Bundles: CacheOpts{
			CacheTimeMs: 60000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		BundleTransactions: CacheOpts{
			CacheTimeMs: 5000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Milestones: CacheOpts{
			CacheTimeMs: 60000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Transactions: CacheOpts{
			CacheTimeMs: 60000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		FirstSeenTx: CacheOpts{
			CacheTimeMs: 60000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		IncomingTransactionFilter: CacheOpts{
			CacheTimeMs: 5000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		RefsInvalidBundle: CacheOpts{
			CacheTimeMs: 180000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		SpentAddresses: CacheOpts{
			CacheTimeMs: 0,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
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
		Addresses: CacheOpts{
			CacheTimeMs: 30000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Approvers: CacheOpts{
			CacheTimeMs: 30000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Tags: CacheOpts{
			CacheTimeMs: 30000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Bundles: CacheOpts{
			CacheTimeMs: 30000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		BundleTransactions: CacheOpts{
			CacheTimeMs: 2500,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Milestones: CacheOpts{
			CacheTimeMs: 30000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Transactions: CacheOpts{
			CacheTimeMs: 30000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		FirstSeenTx: CacheOpts{
			CacheTimeMs: 30000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		IncomingTransactionFilter: CacheOpts{
			CacheTimeMs: 5000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		RefsInvalidBundle: CacheOpts{
			CacheTimeMs: 180000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		SpentAddresses: CacheOpts{
			CacheTimeMs: 0,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
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
		Addresses: CacheOpts{
			CacheTimeMs: 5000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Approvers: CacheOpts{
			CacheTimeMs: 5000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Tags: CacheOpts{
			CacheTimeMs: 5000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Bundles: CacheOpts{
			CacheTimeMs: 5000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		BundleTransactions: CacheOpts{
			CacheTimeMs: 500,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Milestones: CacheOpts{
			CacheTimeMs: 5000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Transactions: CacheOpts{
			CacheTimeMs: 5000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		FirstSeenTx: CacheOpts{
			CacheTimeMs: 5000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		IncomingTransactionFilter: CacheOpts{
			CacheTimeMs: 2500,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		RefsInvalidBundle: CacheOpts{
			CacheTimeMs: 180000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		SpentAddresses: CacheOpts{
			CacheTimeMs: 0,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
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
		Addresses: CacheOpts{
			CacheTimeMs: 1500,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Approvers: CacheOpts{
			CacheTimeMs: 1500,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Tags: CacheOpts{
			CacheTimeMs: 1500,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Bundles: CacheOpts{
			CacheTimeMs: 500,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		BundleTransactions: CacheOpts{
			CacheTimeMs: 500,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Milestones: CacheOpts{
			CacheTimeMs: 1500,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Transactions: CacheOpts{
			CacheTimeMs: 1500,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		FirstSeenTx: CacheOpts{
			CacheTimeMs: 1500,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		IncomingTransactionFilter: CacheOpts{
			CacheTimeMs: 1500,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		RefsInvalidBundle: CacheOpts{
			CacheTimeMs: 180000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		SpentAddresses: CacheOpts{
			CacheTimeMs: 0,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
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
	Name   string     `mapstructure:"name"`
	Caches Caches     `mapstructure:"caches"`
	Badger BadgerOpts `mapstructure:"badger"`
}

type Caches struct {
	Addresses                 CacheOpts `mapstructure:"addresses"`
	Bundles                   CacheOpts `mapstructure:"bundles"`
	BundleTransactions        CacheOpts `mapstructure:"bundleTransactions"`
	Approvers                 CacheOpts `mapstructure:"approvers"`
	Tags                      CacheOpts `mapstructure:"tags"`
	Milestones                CacheOpts `mapstructure:"milestones"`
	Transactions              CacheOpts `mapstructure:"transactions"`
	IncomingTransactionFilter CacheOpts `mapstructure:"incomingTransactionFilter"`
	RefsInvalidBundle         CacheOpts `mapstructure:"refsInvalidBundle"`
	FirstSeenTx               CacheOpts `mapstructure:"firstSeenTx"`
	SpentAddresses            CacheOpts `mapstructure:"spentAddresses"`
}

type CacheOpts struct {
	CacheTimeMs          uint64            `mapstructure:"cacheTimeMs"`
	LeakDetectionOptions LeakDetectionOpts `mapstructure:"leakDetection"`
}

type LeakDetectionOpts struct {
	Enabled                bool   `mapstructure:"enabled"`
	MaxConsumersPerObject  int    `mapstructure:"maxConsumersPerObject"`
	MaxConsumerHoldTimeSec uint64 `mapstructure:"maxConsumerHoldTimeSec"`
}

type BadgerOpts struct {
	LevelOneSize            int64                   `mapstructure:"levelOneSize"`
	LevelSizeMultiplier     int                     `mapstructure:"levelSizeMultiplier"`
	TableLoadingMode        options.FileLoadingMode `mapstructure:"tableLoadingMode"`
	ValueLogLoadingMode     options.FileLoadingMode `mapstructure:"valueLogLoadingMode"`
	MaxLevels               int                     `mapstructure:"maxLevels"`
	MaxTableSize            int64                   `mapstructure:"maxTableSize"`
	NumCompactors           int                     `mapstructure:"numCompactors"`
	NumLevelZeroTables      int                     `mapstructure:"numLevelZeroTables"`
	NumLevelZeroTablesStall int                     `mapstructure:"numLevelZeroTablesStall"`
	NumMemtables            int                     `mapstructure:"numMemtables"`
	BloomFalsePositive      float64                 `mapstructure:"bloomFalsePositive"`
	BlockSize               int                     `mapstructure:"blockSize"`
	SyncWrites              bool                    `mapstructure:"syncWrites"`
	NumVersionsToKeep       int                     `mapstructure:"numVersionsToKeep"`
	CompactLevel0OnClose    bool                    `mapstructure:"compactLevel0OnClose"`
	KeepL0InMemory          bool                    `mapstructure:"keepL0InMemory"`
	VerifyValueChecksum     bool                    `mapstructure:"verifyValueChecksum"`
	MaxCacheSize            int64                   `mapstructure:"maxCacheSize"`
	ZSTDCompressionLevel    int                     `mapstructure:"ZSTDCompressionLevel"`
	CompressionType         options.CompressionType `mapstructure:"CompressionType"`
	ValueLogFileSize        int64                   `mapstructure:"valueLogFileSize"`
	ValueLogMaxEntries      uint32                  `mapstructure:"valueLogMaxEntries"`
	ValueThreshold          int                     `mapstructure:"valueThreshold"`
	WithTruncate            bool                    `mapstructure:"withTruncate"`
	LogRotatesToFlush       int32                   `mapstructure:"logRotatesToFlush"`
	EventLogging            bool                    `mapstructure:"eventLogging"`
}
