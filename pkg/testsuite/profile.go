package testsuite

import "github.com/iotaledger/hornet/v2/pkg/profile"

var TestProfileCaches = &profile.Caches{
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
}
