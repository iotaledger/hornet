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
	CacheTime                  string             `koanf:"cacheTime"`
	ReleaseExecutorWorkerCount int                `koanf:"releaseExecutorWorkerCount"`
	LeakDetectionOptions       *LeakDetectionOpts `koanf:"leakDetection"`
}

type LeakDetectionOpts struct {
	Enabled               bool   `koanf:"enabled"`
	MaxConsumersPerObject int    `koanf:"maxConsumersPerObject"`
	MaxConsumerHoldTime   string `koanf:"maxConsumerHoldTime"`
}
