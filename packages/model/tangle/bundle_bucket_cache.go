package tangle

import (
	"github.com/iotaledger/hive.go/lru_cache"

	"github.com/gohornet/hornet/packages/profile"
)

var (
	BundleBucketCache *lru_cache.LRUCache
)

func InitBundleCache() {
	opts := profile.GetProfile().Caches.Bundles
	BundleBucketCache = lru_cache.NewLRUCache(opts.Size, &lru_cache.LRUCacheOptions{
		EvictionCallback:  onEvictBundles,
		EvictionBatchSize: opts.EvictionSize,
	})
}

func onEvictBundles(_ interface{}, values interface{}) {
	valT := values.([]interface{})

	var bundles []*BundleBucket
	for _, obj := range valT {
		bundles = append(bundles, obj.(*BundleBucket))
	}

	err := StoreBundleBucketsInDatabase(bundles)
	if err != nil {
		panic(err)
	}
}

func FlushBundleCache() {
	BundleBucketCache.DeleteAll()
}
