package tangle

import (
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/iotaledger/iota.go/trinary"
)

func GetBundleBucket(bundleHash trinary.Hash) (result *BundleBucket, err error) {
	if cacheResult := BundleBucketCache.ComputeIfAbsent(bundleHash, func() interface{} {
		bundleBucket, dbErr := readBundleBucketFromDatabase(bundleHash)
		if bundleBucket != nil && dbErr == nil {
			return bundleBucket
		} else if dbErr == nil {
			// Start with an empty bucket.
			// This won't get saved into the db until new tx are appended and the modified flag is changed
			return NewBundleBucket(bundleHash, map[trinary.Hash]*CachedTransaction{})
		} else {
			err = dbErr
			return nil
		}
	}); !typeutils.IsInterfaceNil(cacheResult) {
		result = cacheResult.(*BundleBucket)
	}
	return
}
