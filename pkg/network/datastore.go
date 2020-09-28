package network

import (
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

// KVStoreDatastore implements datastore.Datastore for a kvstore.KVStore.
type KVStoreDatastore struct {
	kv kvstore.KVStore
}

func (kvd *KVStoreDatastore) Get(key datastore.Key) (value []byte, err error) {
	return kvd.kv.Get(key.Bytes())
}

func (kvd *KVStoreDatastore) Has(key datastore.Key) (exists bool, err error) {
	return kvd.kv.Has(key.Bytes())
}

func (kvd *KVStoreDatastore) GetSize(key datastore.Key) (size int, err error) {
	v, err := kvd.kv.Get(key.Bytes())
	if err != nil {
		return 0, err
	}
	return len(v), nil
}

func (kvd *KVStoreDatastore) Query(q query.Query) (query.Results, error) {
	panic("implement me")
}

func (kvd *KVStoreDatastore) Put(key datastore.Key, value []byte) error {
	panic("implement me")
}

func (kvd *KVStoreDatastore) Delete(key datastore.Key) error {
	panic("implement me")
}

func (kvd *KVStoreDatastore) Sync(prefix datastore.Key) error {
	panic("implement me")
}

func (kvd *KVStoreDatastore) Close() error {
	panic("implement me")
}
