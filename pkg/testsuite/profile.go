package testsuite

import "github.com/gohornet/hornet/pkg/profile"

var TestProfileCaches = &profile.Caches{
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
}
