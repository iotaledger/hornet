package tangle

import (
	"github.com/gohornet/hornet/packages/datastructure"
)

var (
	bundleBucketCache *datastructure.LRUCache
)

func InitBundleCache() {
	bundleBucketCache = datastructure.NewLRUCache(BundleCacheSize, &datastructure.LRUCacheOptions{
		EvictionCallback:  onEvictBundles,
		EvictionBatchSize: 1000,
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
	bundleBucketCache.DeleteAll()
}
