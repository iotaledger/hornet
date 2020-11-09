package profile

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/mem"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/profile"
)

var (
	ErrNotEnoughMemory = errors.New("not enough system memory")
)

func init() {
	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
			Name:      "Profile",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
		},
	}
}

var (
	CorePlugin *node.CorePlugin
	log        *logger.Logger
	deps       dependencies
)

type dependencies struct {
	dig.In
	Profile    *profile.Profile
	NodeConfig *configuration.Configuration `name:"nodeConfig"`
}

func provide(c *dig.Container) {
	log = logger.NewLogger(CorePlugin.Name)

	type deps struct {
		dig.In
		NodeConfig     *configuration.Configuration `name:"nodeConfig"`
		ProfilesConfig *configuration.Configuration `name:"profilesConfig"`
	}
	if err := c.Provide(func(d deps) *profile.Profile {
		return loadProfile(d.NodeConfig, d.ProfilesConfig)
	}); err != nil {
		panic(err)
	}
}

func configure() {
	if deps.NodeConfig.String(CfgProfileUseProfile) == AutoProfileName {
		log.Infof("Profile mode 'auto', Using profile '%s'", deps.Profile.Name)
	} else {
		log.Infof("Using profile '%s'", deps.Profile.Name)
	}
}

// loadProfile automatically loads the appropriate profile (given the system memory) if the config value
// is set to 'auto' or the one specified in the config.
func loadProfile(nodeConfig *configuration.Configuration, profilesConfig *configuration.Configuration) *profile.Profile {
	profileName := strings.ToLower(nodeConfig.String(CfgProfileUseProfile))
	if profileName == AutoProfileName {
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

	var p *profile.Profile
	switch profileName {
	case "8gb":
		p = Profile8GB
		p.Name = "8gb"
	case "4gb":
		p = Profile4GB
		p.Name = "4gb"
	case "2gb":
		p = Profile2GB
		p.Name = "2gb"
	case "1gb", "light":
		p = Profile1GB
		p.Name = "1gb"
	default:
		p = &profile.Profile{}
		if !profilesConfig.Exists(profileName) {
			panic(fmt.Sprintf("profile '%s' is not defined in the config", profileName))
		}
		if err := profilesConfig.Unmarshal(profileName, p); err != nil {
			panic(err)
		}
		p.Name = profileName
	}
	return p
}

var (
	Profile8GB = &profile.Profile{
		Caches: &profile.Caches{
			Addresses: &profile.CacheOpts{
				CacheTimeMs: 10000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Children: &profile.CacheOpts{
				CacheTimeMs: 30000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Indexations: &profile.CacheOpts{
				CacheTimeMs: 10000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Milestones: &profile.CacheOpts{
				CacheTimeMs: 10000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Messages: &profile.CacheOpts{
				CacheTimeMs: 30000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			UnreferencedMessages: &profile.CacheOpts{
				CacheTimeMs: 500,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			IncomingMessagesFilter: &profile.CacheOpts{
				CacheTimeMs: 5000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
		},
	}

	Profile4GB = &profile.Profile{
		Caches: &profile.Caches{
			Addresses: &profile.CacheOpts{
				CacheTimeMs: 5000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Children: &profile.CacheOpts{
				CacheTimeMs: 15000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Indexations: &profile.CacheOpts{
				CacheTimeMs: 5000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Milestones: &profile.CacheOpts{
				CacheTimeMs: 5000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Messages: &profile.CacheOpts{
				CacheTimeMs: 15000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			UnreferencedMessages: &profile.CacheOpts{
				CacheTimeMs: 500,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			IncomingMessagesFilter: &profile.CacheOpts{
				CacheTimeMs: 5000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
		},
	}

	Profile2GB = &profile.Profile{
		Caches: &profile.Caches{
			Addresses: &profile.CacheOpts{
				CacheTimeMs: 200,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Children: &profile.CacheOpts{
				CacheTimeMs: 5000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Indexations: &profile.CacheOpts{
				CacheTimeMs: 200,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Milestones: &profile.CacheOpts{
				CacheTimeMs: 2500,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Messages: &profile.CacheOpts{
				CacheTimeMs: 5000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			UnreferencedMessages: &profile.CacheOpts{
				CacheTimeMs: 100,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			IncomingMessagesFilter: &profile.CacheOpts{
				CacheTimeMs: 2500,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
		},
	}

	Profile1GB = &profile.Profile{
		Caches: &profile.Caches{
			Addresses: &profile.CacheOpts{
				CacheTimeMs: 100,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Children: &profile.CacheOpts{
				CacheTimeMs: 1500,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Indexations: &profile.CacheOpts{
				CacheTimeMs: 100,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Milestones: &profile.CacheOpts{
				CacheTimeMs: 500,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			Messages: &profile.CacheOpts{
				CacheTimeMs: 1500,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			UnreferencedMessages: &profile.CacheOpts{
				CacheTimeMs: 100,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
			IncomingMessagesFilter: &profile.CacheOpts{
				CacheTimeMs: 2000,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:                false,
					MaxConsumersPerObject:  20,
					MaxConsumerHoldTimeSec: 100,
				},
			},
		},
	}
)
