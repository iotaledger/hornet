package profile

type Profile struct {
	Name   string  `koanf:"name"`
	Caches *Caches `koanf:"caches"`
}

type Caches struct {
	Addresses              *CacheOpts `koanf:"addresses"`
	Children               *CacheOpts `koanf:"children"`
	Indexations            *CacheOpts `koanf:"indexations"`
	Milestones             *CacheOpts `koanf:"milestones"`
	Messages               *CacheOpts `koanf:"messages"`
	IncomingMessagesFilter *CacheOpts `koanf:"incomingMessagesFilter"`
	UnreferencedMessages   *CacheOpts `koanf:"unreferencedMessages"`
}

type CacheOpts struct {
	CacheTimeMs          uint64             `koanf:"cacheTimeMs"`
	LeakDetectionOptions *LeakDetectionOpts `koanf:"leakDetection"`
}

type LeakDetectionOpts struct {
	Enabled                bool   `koanf:"enabled"`
	MaxConsumersPerObject  int    `koanf:"maxConsumersPerObject"`
	MaxConsumerHoldTimeSec uint64 `koanf:"maxConsumerHoldTimeSec"`
}
