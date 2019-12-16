package database

import (
	"context"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/pb"

	"github.com/gohornet/hornet/packages/syncutils"
)

const (
	StreamNumGoRoutines = 16
)

var (
	ErrKeyNotFound = badger.ErrKeyNotFound

	dbMap = make(map[string]*prefixDb)
	mu    syncutils.Mutex
)

type prefixDb struct {
	db     *badger.DB
	prefix []byte
}

func Get(dbPrefix byte) (Database, error) {
	mu.Lock()
	defer mu.Unlock()

	if db, exists := dbMap[string(dbPrefix)]; exists {
		return db, nil
	}

	badger := GetBadgerInstance()
	db := &prefixDb{
		db:     badger,
		prefix: []byte{dbPrefix},
	}

	dbMap[string(dbPrefix)] = db

	return db, nil
}

func CloseDatabase() error {
	return GetBadgerInstance().Close()
}

func (pdb *prefixDb) keyWithPrefix(key Key) Key {
	return append(pdb.prefix, key...)
}

func (pdb *prefixDb) keyWithoutPrefix(key Key) Key {
	return key[1:]
}

func (k Key) keyWithoutKeyPrefix(prefix KeyPrefix) Key {
	return k[len(prefix):]
}

func (pdb *prefixDb) Set(entry Entry) error {
	wb := pdb.db.NewWriteBatch()
	defer wb.Cancel()

	err := wb.SetEntry(badger.NewEntry(pdb.keyWithPrefix(entry.Key), entry.Value).WithMeta(entry.Meta))
	if err != nil {
		return err
	}
	return wb.Flush()
}

func (pdb *prefixDb) Apply(set []Entry, delete []Key) error {

	wb := pdb.db.NewWriteBatch()
	defer wb.Cancel()

	for _, entry := range set {
		keyPrefix := pdb.keyWithPrefix(entry.Key)
		keyCopy := make([]byte, len(keyPrefix))
		copy(keyCopy, keyPrefix)

		valueCopy := make([]byte, len(entry.Value))
		copy(valueCopy, entry.Value)

		err := wb.SetEntry(badger.NewEntry(keyCopy, valueCopy).WithMeta(entry.Meta))
		if err != nil {
			return err
		}
	}
	for _, key := range delete {
		keyPrefix := pdb.keyWithPrefix(key)
		keyCopy := make([]byte, len(keyPrefix))
		copy(keyCopy, keyPrefix)

		err := wb.Delete(keyCopy)
		if err != nil {
			return err
		}
	}
	return wb.Flush()
}

func (pdb *prefixDb) Contains(key Key) (bool, error) {
	err := pdb.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(pdb.keyWithPrefix(key))
		return err
	})

	if err == ErrKeyNotFound {
		return false, nil
	} else {
		return err == nil, err
	}
}

func (pdb *prefixDb) Get(key Key) (Entry, error) {
	var result Entry

	err := pdb.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(pdb.keyWithPrefix(key))
		if err != nil {
			return err
		}
		result.Key = key
		result.Meta = item.UserMeta()

		result.Value, err = item.ValueCopy(nil)
		return err
	})

	return result, err
}

func (pdb *prefixDb) GetKeyOnly(key Key) (KeyOnlyEntry, error) {
	var result KeyOnlyEntry

	err := pdb.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(pdb.keyWithPrefix(key))
		if err != nil {
			return err
		}
		result.Key = key
		result.Meta = item.UserMeta()

		return nil
	})

	return result, err
}

func (pdb *prefixDb) Delete(key Key) error {
	wb := pdb.db.NewWriteBatch()
	defer wb.Cancel()

	err := wb.Delete(pdb.keyWithPrefix(key))
	if err != nil {
		return err
	}
	return wb.Flush()
}

func (pdb *prefixDb) DeletePrefix(keyPrefix KeyPrefix) error {
	prefixToDelete := append(pdb.prefix, keyPrefix...)
	return pdb.db.DropPrefix(prefixToDelete)
}

func (pdb *prefixDb) ForEach(consumer func(Entry) bool) error {
	err := pdb.db.View(func(txn *badger.Txn) error {
		iteratorOptions := badger.DefaultIteratorOptions
		it := txn.NewIterator(iteratorOptions)
		defer it.Close()
		prefix := pdb.prefix

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			meta := item.UserMeta()

			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			if consumer(Entry{
				Key:   pdb.keyWithoutPrefix(item.Key()),
				Value: value,
				Meta:  meta,
			}) {
				break
			}
		}
		return nil
	})
	return err
}

func (pdb *prefixDb) ForEachPrefix(keyPrefix KeyPrefix, consumer func(Entry) bool) error {
	err := pdb.db.View(func(txn *badger.Txn) error {
		iteratorOptions := badger.DefaultIteratorOptions
		it := txn.NewIterator(iteratorOptions)
		defer it.Close()
		prefix := append(pdb.prefix, keyPrefix...)

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			meta := item.UserMeta()

			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			if consumer(Entry{
				Key:   pdb.keyWithoutPrefix(item.Key()).keyWithoutKeyPrefix(keyPrefix),
				Value: value,
				Meta:  meta,
			}) {
				break
			}
		}
		return nil
	})
	return err
}

func (pdb *prefixDb) ForEachPrefixKeyOnly(keyPrefix KeyPrefix, consumer func(KeyOnlyEntry) bool) error {
	err := pdb.db.View(func(txn *badger.Txn) error {
		iteratorOptions := badger.DefaultIteratorOptions
		iteratorOptions.PrefetchValues = false
		it := txn.NewIterator(iteratorOptions)
		defer it.Close()
		prefix := append(pdb.prefix, keyPrefix...)

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			meta := item.UserMeta()

			if consumer(KeyOnlyEntry{
				Key:  pdb.keyWithoutPrefix(item.Key()).keyWithoutKeyPrefix(keyPrefix),
				Meta: meta,
			}) {
				break
			}
		}
		return nil
	})
	return err
}

func (pdb *prefixDb) StreamForEach(consumer func(Entry) error) error {
	stream := pdb.db.NewStream()

	stream.NumGo = StreamNumGoRoutines
	stream.Prefix = pdb.prefix
	stream.ChooseKey = nil
	stream.KeyToList = nil

	// Send is called serially, while Stream.Orchestrate is running.
	stream.Send = func(list *pb.KVList) error {
		for _, kv := range list.Kv {
			var meta byte
			tmpMeta := kv.GetUserMeta()
			if len(tmpMeta) > 0 {
				meta = tmpMeta[0]
			}
			err := consumer(Entry{
				Key:   pdb.keyWithoutPrefix(kv.GetKey()),
				Value: kv.GetValue(),
				Meta:  meta,
			})
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Run the stream
	return stream.Orchestrate(context.Background())
}

func (pdb *prefixDb) StreamForEachKeyOnly(consumer func(KeyOnlyEntry) error) error {
	stream := pdb.db.NewStream()

	stream.NumGo = StreamNumGoRoutines
	stream.Prefix = pdb.prefix
	stream.ChooseKey = nil
	stream.KeyToList = nil

	// Send is called serially, while Stream.Orchestrate is running.
	stream.Send = func(list *pb.KVList) error {
		for _, kv := range list.Kv {
			var meta byte
			tmpMeta := kv.GetUserMeta()
			if len(tmpMeta) > 0 {
				meta = tmpMeta[0]
			}
			err := consumer(KeyOnlyEntry{
				Key:  pdb.keyWithoutPrefix(kv.GetKey()),
				Meta: meta,
			})
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Run the stream
	return stream.Orchestrate(context.Background())
}

func (pdb *prefixDb) StreamForEachPrefix(keyPrefix KeyPrefix, consumer func(Entry) error) error {
	stream := pdb.db.NewStream()

	stream.NumGo = StreamNumGoRoutines
	stream.Prefix = append(pdb.prefix, keyPrefix...)
	stream.ChooseKey = nil
	stream.KeyToList = nil

	// Send is called serially, while Stream.Orchestrate is running.
	stream.Send = func(list *pb.KVList) error {
		for _, kv := range list.Kv {
			var meta byte
			tmpMeta := kv.GetUserMeta()
			if len(tmpMeta) > 0 {
				meta = tmpMeta[0]
			}
			err := consumer(Entry{
				Key:   pdb.keyWithoutPrefix(kv.GetKey()).keyWithoutKeyPrefix(keyPrefix),
				Value: kv.GetValue(),
				Meta:  meta,
			})
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Run the stream
	return stream.Orchestrate(context.Background())
}

func (pdb *prefixDb) StreamForEachPrefixKeyOnly(keyPrefix KeyPrefix, consumer func(KeyOnlyEntry) error) error {
	stream := pdb.db.NewStream()

	stream.NumGo = StreamNumGoRoutines
	stream.Prefix = append(pdb.prefix, keyPrefix...)
	stream.ChooseKey = nil
	stream.KeyToList = nil

	// Send is called serially, while Stream.Orchestrate is running.
	stream.Send = func(list *pb.KVList) error {
		for _, kv := range list.Kv {
			var meta byte
			tmpMeta := kv.GetUserMeta()
			if len(tmpMeta) > 0 {
				meta = tmpMeta[0]
			}
			err := consumer(KeyOnlyEntry{
				Key:  pdb.keyWithoutPrefix(kv.GetKey()).keyWithoutKeyPrefix(keyPrefix),
				Meta: meta,
			})
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Run the stream
	return stream.Orchestrate(context.Background())
}
