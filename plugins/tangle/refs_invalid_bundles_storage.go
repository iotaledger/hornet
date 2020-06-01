package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/profile"
)

var (
	refsAnInvalidBundleStorage *objectstorage.ObjectStorage
)

type invalidBundleReference struct {
	objectstorage.StorableObjectFlags

	hashBytes []byte
}

// ObjectStorage interface

func (r *invalidBundleReference) Update(other objectstorage.StorableObject) {
	panic("invalidBundleReference should never be updated")
}

func (r *invalidBundleReference) ObjectStorageKey() []byte {
	return r.hashBytes
}

func (r *invalidBundleReference) ObjectStorageValue() (_ []byte) {
	return nil
}

func (r *invalidBundleReference) UnmarshalObjectStorageValue(_ []byte) (consumedBytes int, err error) {
	return 0, nil
}

func invalidBundleFactory(key []byte) (objectstorage.StorableObject, int, error) {
	invalidBndl := &invalidBundleReference{
		hashBytes: make([]byte, len(key)),
	}
	copy(invalidBndl.hashBytes, key)
	return invalidBndl, len(key), nil
}

func configureRefsAnInvalidBundleStorage() {
	opts := profile.LoadProfile().Caches.RefsInvalidBundle

	refsAnInvalidBundleStorage = objectstorage.New(
		nil,
		invalidBundleFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(false),
		objectstorage.KeysOnly(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

func GetRefsAnInvalidBundleStorageSize() int {
	return refsAnInvalidBundleStorage.GetSize()
}

// +-0
func PutInvalidBundleReference(txHash hornet.Hash) {
	invalidBundleRef, _, _ := invalidBundleFactory(txHash)

	// Do not force the release, otherwise the object is gone (no persistence enabled)
	refsAnInvalidBundleStorage.ComputeIfAbsent(invalidBundleRef.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject {
		return invalidBundleRef
	}).Release()
}

// +-0
func ContainsInvalidBundleReference(txHash hornet.Hash) bool {
	return refsAnInvalidBundleStorage.Contains(txHash)
}
