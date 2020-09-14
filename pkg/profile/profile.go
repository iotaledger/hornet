package profile

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/shirou/gopsutil/mem"

	"github.com/gohornet/hornet/pkg/config"
)

var (
	once    = sync.Once{}
	profile *Profile

	ErrNotEnoughMemory = errors.New("not enough system memory")
)

// LoadProfile automatically loads the appropriate profile (given the system memory) if the config value
// is set to 'auto' or the one specified in the config.
func LoadProfile() *Profile {
	once.Do(func() {
		profileName := strings.ToLower(config.NodeConfig.GetString(config.CfgProfileUseProfile))
		if profileName == config.AutoProfileName {
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
			if !config.ProfilesConfig.IsSet(profileName) {
				panic(fmt.Sprintf("profile '%s' is not defined in the config", profileName))
			}
			if err := config.ProfilesConfig.UnmarshalKey(profileName, p); err != nil {
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
			CacheTimeMs: 10000,
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
			CacheTimeMs: 10000,
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
			CacheTimeMs: 10000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Milestones: CacheOpts{
			CacheTimeMs: 10000,
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
		UnconfirmedTx: CacheOpts{
			CacheTimeMs: 500,
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
		SpentAddresses: CacheOpts{
			CacheTimeMs: 0,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
	},
}

var Profile4GB = &Profile{
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
			CacheTimeMs: 15000,
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
			CacheTimeMs: 15000,
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
			CacheTimeMs: 5000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Transactions: CacheOpts{
			CacheTimeMs: 15000,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		UnconfirmedTx: CacheOpts{
			CacheTimeMs: 500,
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
		SpentAddresses: CacheOpts{
			CacheTimeMs: 0,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
	},
}

var Profile2GB = &Profile{
	Caches: Caches{
		Addresses: CacheOpts{
			CacheTimeMs: 200,
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
			CacheTimeMs: 200,
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
			CacheTimeMs: 1500,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Milestones: CacheOpts{
			CacheTimeMs: 2500,
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
		UnconfirmedTx: CacheOpts{
			CacheTimeMs: 100,
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
		SpentAddresses: CacheOpts{
			CacheTimeMs: 0,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
	},
}

var Profile1GB = &Profile{
	Caches: Caches{
		Addresses: CacheOpts{
			CacheTimeMs: 100,
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
			CacheTimeMs: 100,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		Bundles: CacheOpts{
			CacheTimeMs: 1500,
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
			CacheTimeMs: 500,
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
		UnconfirmedTx: CacheOpts{
			CacheTimeMs: 100,
			LeakDetectionOptions: LeakDetectionOpts{
				Enabled:                false,
				MaxConsumersPerObject:  20,
				MaxConsumerHoldTimeSec: 100,
			},
		},
		IncomingTransactionFilter: CacheOpts{
			CacheTimeMs: 2000,
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
}

type Profile struct {
	Name   string `mapstructure:"name"`
	Caches Caches `mapstructure:"caches"`
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
	UnconfirmedTx             CacheOpts `mapstructure:"unconfirmedTx"`
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
