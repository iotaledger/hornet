package profile

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/mem"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hornet/v2/pkg/profile"
)

var (
	ErrNotEnoughMemory = errors.New("not enough system memory")
)

func init() {
	CoreComponent = &app.CoreComponent{
		Component: &app.Component{
			Name:      "Profile",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
		},
	}
}

var (
	CoreComponent *app.CoreComponent
	deps          dependencies
)

type dependencies struct {
	dig.In
	Profile *profile.Profile
}

func provide(c *dig.Container) error {

	if err := c.Provide(func() string {
		return ParamsNode.Alias
	}, dig.Name("nodeAlias")); err != nil {
		CoreComponent.LogPanic(err)
	}

	type profileDeps struct {
		dig.In
		ProfilesConfig *configuration.Configuration `name:"profilesConfig"`
	}
	if err := c.Provide(func(d profileDeps) *profile.Profile {
		return loadProfile(d.ProfilesConfig)
	}); err != nil {
		CoreComponent.LogPanic(err)
	}

	return nil
}

func configure() error {

	if ParamsNode.Profile == AutoProfileName {
		CoreComponent.LogInfof("Profile mode 'auto', Using profile '%s'", deps.Profile.Name)
	} else {
		CoreComponent.LogInfof("Using profile '%s'", deps.Profile.Name)
	}

	return nil
}

// loadProfile automatically loads the appropriate profile (given the system memory) if the config value
// is set to 'auto' or the one specified in the config.
func loadProfile(profilesConfig *configuration.Configuration) *profile.Profile {
	profileName := strings.ToLower(ParamsNode.Profile)
	if profileName == AutoProfileName {
		v, err := mem.VirtualMemory()
		if err != nil {
			CoreComponent.LogPanic(err)
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
			CoreComponent.LogPanic(ErrNotEnoughMemory)
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
			CoreComponent.LogPanicf("profile '%s' is not defined in the config", profileName)
		}
		if err := profilesConfig.Unmarshal(profileName, p); err != nil {
			CoreComponent.LogPanic(err)
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
			Milestones: &profile.CacheOpts{
				CacheTime:                  "10s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Blocks: &profile.CacheOpts{
				CacheTime:                  "30s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			UnreferencedBlocks: &profile.CacheOpts{
				CacheTime:                  "500ms",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			IncomingBlocksFilter: &profile.CacheOpts{
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
			Milestones: &profile.CacheOpts{
				CacheTime:                  "5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Blocks: &profile.CacheOpts{
				CacheTime:                  "15s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			UnreferencedBlocks: &profile.CacheOpts{
				CacheTime:                  "500ms",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			IncomingBlocksFilter: &profile.CacheOpts{
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
			Milestones: &profile.CacheOpts{
				CacheTime:                  "2.5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Blocks: &profile.CacheOpts{
				CacheTime:                  "5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			UnreferencedBlocks: &profile.CacheOpts{
				CacheTime:                  "100ms",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			IncomingBlocksFilter: &profile.CacheOpts{
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
			Milestones: &profile.CacheOpts{
				CacheTime:                  "500ms",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			Blocks: &profile.CacheOpts{
				CacheTime:                  "1.5s",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			UnreferencedBlocks: &profile.CacheOpts{
				CacheTime:                  "100ms",
				ReleaseExecutorWorkerCount: 10,
				LeakDetectionOptions: &profile.LeakDetectionOpts{
					Enabled:               false,
					MaxConsumersPerObject: 20,
					MaxConsumerHoldTime:   "100s",
				},
			},
			IncomingBlocksFilter: &profile.CacheOpts{
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
