package profile

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/mem"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/iotaledger/hive.go/configuration"
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
	deps       dependencies
)

type dependencies struct {
	dig.In
	Profile    *profile.Profile
	NodeConfig *configuration.Configuration `name:"nodeConfig"`
}

func provide(c *dig.Container) {

	type profileDeps struct {
		dig.In
		NodeConfig     *configuration.Configuration `name:"nodeConfig"`
		ProfilesConfig *configuration.Configuration `name:"profilesConfig"`
	}
	if err := c.Provide(func(d profileDeps) *profile.Profile {
		return loadProfile(d.NodeConfig, d.ProfilesConfig)
	}); err != nil {
		CorePlugin.Panic(err)
	}
}

func configure() {

	if deps.NodeConfig.String(CfgNodeProfile) == AutoProfileName {
		CorePlugin.LogInfof("Profile mode 'auto', Using profile '%s'", deps.Profile.Name)
	} else {
		CorePlugin.LogInfof("Using profile '%s'", deps.Profile.Name)
	}
}

// loadProfile automatically loads the appropriate profile (given the system memory) if the config value
// is set to 'auto' or the one specified in the config.
func loadProfile(nodeConfig *configuration.Configuration, profilesConfig *configuration.Configuration) *profile.Profile {
	profileName := strings.ToLower(nodeConfig.String(CfgNodeProfile))
	if profileName == AutoProfileName {
		v, err := mem.VirtualMemory()
		if err != nil {
			CorePlugin.Panic(err)
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
			CorePlugin.Panic(ErrNotEnoughMemory)
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
			CorePlugin.Panicf("profile '%s' is not defined in the config", profileName)
		}
		if err := profilesConfig.Unmarshal(profileName, p); err != nil {
			CorePlugin.Panic(err)
		}
		p.Name = profileName
	}
	return p
}

var (
	Profile8GB = &profile.Profile{
		Caches: &profile.Caches{
			Addresses: &profile.CacheOpts{
				CacheTime:                  "10s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Children: &profile.CacheOpts{
				CacheTime:                  "30s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Indexations: &profile.CacheOpts{
				CacheTime:                  "10s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Milestones: &profile.CacheOpts{
				CacheTime:                  "10s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Messages: &profile.CacheOpts{
				CacheTime:                  "30s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			UnreferencedMessages: &profile.CacheOpts{
				CacheTime:                  "500ms",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			IncomingMessagesFilter: &profile.CacheOpts{
				CacheTime:                  "5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
		},
	}

	Profile4GB = &profile.Profile{
		Caches: &profile.Caches{
			Addresses: &profile.CacheOpts{
				CacheTime:                  "5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Children: &profile.CacheOpts{
				CacheTime:                  "15s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Indexations: &profile.CacheOpts{
				CacheTime:                  "5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Milestones: &profile.CacheOpts{
				CacheTime:                  "5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Messages: &profile.CacheOpts{
				CacheTime:                  "15s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			UnreferencedMessages: &profile.CacheOpts{
				CacheTime:                  "500ms",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			IncomingMessagesFilter: &profile.CacheOpts{
				CacheTime:                  "5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
		},
	}

	Profile2GB = &profile.Profile{
		Caches: &profile.Caches{
			Addresses: &profile.CacheOpts{
				CacheTime:                  "200ms",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Children: &profile.CacheOpts{
				CacheTime:                  "5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Indexations: &profile.CacheOpts{
				CacheTime:                  "200ms",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Milestones: &profile.CacheOpts{
				CacheTime:                  "2.5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Messages: &profile.CacheOpts{
				CacheTime:                  "5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			UnreferencedMessages: &profile.CacheOpts{
				CacheTime:                  "100ms",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			IncomingMessagesFilter: &profile.CacheOpts{
				CacheTime:                  "2.5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
		},
	}

	Profile1GB = &profile.Profile{
		Caches: &profile.Caches{
			Addresses: &profile.CacheOpts{
				CacheTime:                  "100ms",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Children: &profile.CacheOpts{
				CacheTime:                  "1.5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Indexations: &profile.CacheOpts{
				CacheTime:                  "100ms",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Milestones: &profile.CacheOpts{
				CacheTime:                  "500ms",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Messages: &profile.CacheOpts{
				CacheTime:                  "1.5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			UnreferencedMessages: &profile.CacheOpts{
				CacheTime:                  "100ms",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			IncomingMessagesFilter: &profile.CacheOpts{
				CacheTime:                  "2s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
		},
	}
)
