package database

import (
	"sync"

	rocksdb "github.com/tecbot/gorocksdb"

	"github.com/iotaledger/hive.go/byteutils"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/types"
)

type RocksDB struct {
	name string
	opts *rocksdb.Options
	db   *rocksdb.DB
	ro   *rocksdb.ReadOptions
	wo   *rocksdb.WriteOptions
	fo   *rocksdb.FlushOptions
}

// NewRocksDB creates a new RocksDB instance.
func NewRocksDB(path string) *RocksDB {

	opts := rocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)

	ro := rocksdb.NewDefaultReadOptions()
	ro.SetFillCache(false)

	wo := rocksdb.NewDefaultWriteOptions()
	wo.SetSync(false)

	fo := rocksdb.NewDefaultFlushOptions()

	db, err := rocksdb.OpenDb(opts, path)
	if err != nil {
		return nil
	}

	return &RocksDB{
		name: path,
		opts: opts,
		db:   db,
		ro:   ro,
		wo:   wo,
		fo:   fo,
	}
}

func (r *RocksDB) Flush() error {
	r.db.Flush(r.fo)
	return nil
}

func (r *RocksDB) Close() error {
	r.db.Close()
	return nil
}

type rocksDBStore struct {
	instance                     *RocksDB
	dbPrefix                     []byte
	accessCallback               kvstore.AccessCallback
	accessCallbackCommandsFilter kvstore.Command
}

// New creates a new KVStore with the underlying pebbleDB.
func NewRocksDBStore(db *RocksDB) kvstore.KVStore {
	return &rocksDBStore{
		instance: db,
	}
}

// AccessCallback configures the store to pass all requests to the KVStore to the given callback.
// This can for example be used for debugging and to examine what the KVStore is doing.
func (s *rocksDBStore) AccessCallback(callback kvstore.AccessCallback, commandsFilter ...kvstore.Command) {
	var accessCallbackCommandsFilter kvstore.Command
	if len(commandsFilter) == 0 {
		accessCallbackCommandsFilter = kvstore.AllCommands
	} else {
		for _, filterCommand := range commandsFilter {
			accessCallbackCommandsFilter |= filterCommand
		}
	}

	s.accessCallback = callback
	s.accessCallbackCommandsFilter = accessCallbackCommandsFilter
}

func (s *rocksDBStore) WithRealm(realm kvstore.Realm) kvstore.KVStore {
	return &rocksDBStore{
		instance: s.instance,
		dbPrefix: realm,
	}
}

func (s *rocksDBStore) Realm() []byte {
	return s.dbPrefix
}

// builds a key usable for the pebble instance using the realm and the given prefix.
func (s *rocksDBStore) buildKeyPrefix(prefix kvstore.KeyPrefix) kvstore.KeyPrefix {
	return byteutils.ConcatBytes(s.dbPrefix, prefix)
}

// Shutdown marks the store as shutdown.
func (s *rocksDBStore) Shutdown() {
	if s.accessCallback != nil {
		s.accessCallback(kvstore.ShutdownCommand)
	}
}

func copyBytes(source []byte, size int) []byte {
	cpy := make([]byte, size)
	copy(cpy, source)
	return cpy
}

func (s *rocksDBStore) Iterate(prefix kvstore.KeyPrefix, consumerFunc kvstore.IteratorKeyValueConsumerFunc) error {
	if s.accessCallback != nil && s.accessCallbackCommandsFilter.HasBits(kvstore.IterateCommand) {
		s.accessCallback(kvstore.IterateCommand, prefix)
	}

	it := s.instance.db.NewIterator(s.instance.ro)
	defer it.Close()

	keyPrefix := s.buildKeyPrefix(prefix)
	it.Seek(keyPrefix)

	for ; it.ValidForPrefix(keyPrefix); it.Next() {
		key := it.Key()
		k := copyBytes(key.Data(), key.Size())[len(s.dbPrefix):]
		key.Free()

		value := it.Value()
		v := copyBytes(value.Data(), value.Size())
		value.Free()

		if !consumerFunc(k, v) {
			break
		}
	}

	return nil
}

func (s *rocksDBStore) IterateKeys(prefix kvstore.KeyPrefix, consumerFunc kvstore.IteratorKeyConsumerFunc) error {
	if s.accessCallback != nil && s.accessCallbackCommandsFilter.HasBits(kvstore.IterateKeysCommand) {
		s.accessCallback(kvstore.IterateKeysCommand, prefix)
	}

	it := s.instance.db.NewIterator(s.instance.ro)
	defer it.Close()

	keyPrefix := s.buildKeyPrefix(prefix)
	it.Seek(keyPrefix)

	for ; it.ValidForPrefix(keyPrefix); it.Next() {
		key := it.Key()
		k := copyBytes(key.Data(), key.Size())[len(s.dbPrefix):]
		key.Free()

		if !consumerFunc(k) {
			break
		}
	}

	return nil
}

func (s *rocksDBStore) clearDB() error {
	err := s.Close()
	if err != nil {
		return err
	}
	err = rocksdb.DestroyDb(s.instance.name, s.instance.opts)
	if err != nil {
		return err
	}
	s.instance = NewRocksDB(s.instance.name)
	return nil
}

func (s *rocksDBStore) Clear() error {
	if s.accessCallback != nil && s.accessCallbackCommandsFilter.HasBits(kvstore.ClearCommand) {
		s.accessCallback(kvstore.ClearCommand)
	}

	return s.DeletePrefix(kvstore.EmptyPrefix)
}

func (s *rocksDBStore) Get(key kvstore.Key) (kvstore.Value, error) {
	if s.accessCallback != nil && s.accessCallbackCommandsFilter.HasBits(kvstore.GetCommand) {
		s.accessCallback(kvstore.GetCommand, key)
	}

	v, err := s.instance.db.GetBytes(s.instance.ro, byteutils.ConcatBytes(s.dbPrefix, key))
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, kvstore.ErrKeyNotFound
	}
	return v, nil
}

func (s *rocksDBStore) Set(key kvstore.Key, value kvstore.Value) error {
	if s.accessCallback != nil && s.accessCallbackCommandsFilter.HasBits(kvstore.SetCommand) {
		s.accessCallback(kvstore.SetCommand, key, value)
	}

	return s.instance.db.Put(s.instance.wo, byteutils.ConcatBytes(s.dbPrefix, key), value)
}

func (s *rocksDBStore) Has(key kvstore.Key) (bool, error) {
	if s.accessCallback != nil && s.accessCallbackCommandsFilter.HasBits(kvstore.HasCommand) {
		s.accessCallback(kvstore.HasCommand, key)
	}

	v, err := s.instance.db.Get(s.instance.ro, byteutils.ConcatBytes(s.dbPrefix, key))
	if err != nil {
		return false, err
	}
	return v.Exists(), nil
}

func (s *rocksDBStore) Delete(key kvstore.Key) error {
	if s.accessCallback != nil && s.accessCallbackCommandsFilter.HasBits(kvstore.DeleteCommand) {
		s.accessCallback(kvstore.DeleteCommand, key)
	}

	return s.instance.db.Delete(s.instance.wo, byteutils.ConcatBytes(s.dbPrefix, key))
}

func (s *rocksDBStore) DeletePrefix(prefix kvstore.KeyPrefix) error {
	if s.accessCallback != nil && s.accessCallbackCommandsFilter.HasBits(kvstore.DeletePrefixCommand) {
		s.accessCallback(kvstore.DeletePrefixCommand, prefix)
	}

	keyPrefix := s.buildKeyPrefix(prefix)
	if len(keyPrefix) == 0 {
		return s.clearDB()
	}

	writeBatch := rocksdb.NewWriteBatch()
	defer writeBatch.Destroy()

	it := s.instance.db.NewIterator(s.instance.ro)
	defer it.Close()

	it.Seek(keyPrefix)

	for ; it.ValidForPrefix(keyPrefix); it.Next() {
		key := it.Key()
		writeBatch.Delete(key.Data())
	}

	return s.instance.db.Write(s.instance.wo, writeBatch)
}

func (s *rocksDBStore) Batched() kvstore.BatchedMutations {
	return &batchedMutations{
		kvStore:          s,
		store:            s.instance,
		dbPrefix:         s.dbPrefix,
		setOperations:    make(map[string]kvstore.Value),
		deleteOperations: make(map[string]types.Empty),
	}
}

func (s *rocksDBStore) Flush() error {
	return s.instance.Flush()
}

func (s *rocksDBStore) Close() error {
	return s.instance.Close()
}

// batchedMutations is a wrapper around a WriteBatch of a rocksDB.
type batchedMutations struct {
	kvStore          *rocksDBStore
	store            *RocksDB
	dbPrefix         []byte
	setOperations    map[string]kvstore.Value
	deleteOperations map[string]types.Empty
	operationsMutex  sync.Mutex
}

func (b *batchedMutations) Set(key kvstore.Key, value kvstore.Value) error {
	if b.kvStore.accessCallback != nil && b.kvStore.accessCallbackCommandsFilter.HasBits(kvstore.SetCommand) {
		b.kvStore.accessCallback(kvstore.SetCommand, key, value)
	}

	stringKey := byteutils.ConcatBytesToString(b.dbPrefix, key)

	b.operationsMutex.Lock()
	defer b.operationsMutex.Unlock()

	delete(b.deleteOperations, stringKey)
	b.setOperations[stringKey] = value

	return nil
}

func (b *batchedMutations) Delete(key kvstore.Key) error {
	if b.kvStore.accessCallback != nil && b.kvStore.accessCallbackCommandsFilter.HasBits(kvstore.DeleteCommand) {
		b.kvStore.accessCallback(kvstore.DeleteCommand, key)
	}

	stringKey := byteutils.ConcatBytesToString(b.dbPrefix, key)

	b.operationsMutex.Lock()
	defer b.operationsMutex.Unlock()

	delete(b.setOperations, stringKey)
	b.deleteOperations[stringKey] = types.Void

	return nil
}

func (b *batchedMutations) Cancel() {
	b.operationsMutex.Lock()
	defer b.operationsMutex.Unlock()

	b.setOperations = make(map[string]kvstore.Value)
	b.deleteOperations = make(map[string]types.Empty)
}

func (b *batchedMutations) Commit() error {
	writeBatch := rocksdb.NewWriteBatch()
	defer writeBatch.Destroy()

	b.operationsMutex.Lock()
	defer b.operationsMutex.Unlock()

	for key, value := range b.setOperations {
		writeBatch.Put([]byte(key), value)
	}

	for key := range b.deleteOperations {
		writeBatch.Delete([]byte(key))
	}

	return b.store.db.Write(b.store.wo, writeBatch)
}
