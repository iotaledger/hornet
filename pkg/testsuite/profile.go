package testsuite

import "github.com/gohornet/hornet/pkg/profile"

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
}
