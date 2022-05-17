package profile

type Profile struct {
	Name   string  `koanf:"name"`
	Caches *Caches `koanf:"caches"`
}

type Caches struct {
	Addresses            *CacheOpts `koanf:"addresses"`
	Children             *CacheOpts `koanf:"children"`
	Milestones           *CacheOpts `koanf:"milestones"`
	Blocks               *CacheOpts `koanf:"blocks"`
	IncomingBlocksFilter *CacheOpts `koanf:"incomingBlocksFilter"`
	UnreferencedBlocks   *CacheOpts `koanf:"unreferencedBlocks"`
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
