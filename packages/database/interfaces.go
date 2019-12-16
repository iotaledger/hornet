package database

type KeyPrefix []byte
type Key []byte
type Value []byte

type Entry struct {
	Key   Key
	Value Value
	Meta  byte
}

type KeyOnlyEntry struct {
	Key  Key
	Meta byte
}

type Database interface {
	// Read
	Contains(key Key) (bool, error)
	Get(key Key) (Entry, error)
	GetKeyOnly(key Key) (KeyOnlyEntry, error)

	// Write
	Set(entry Entry) error
	Delete(key Key) error
	DeletePrefix(keyPrefix KeyPrefix) error

	// Iteration
	ForEach(func(entry Entry) (stop bool)) error
	ForEachPrefix(keyPrefix KeyPrefix, do func(entry Entry) (stop bool)) error
	ForEachPrefixKeyOnly(keyPrefix KeyPrefix, do func(entry KeyOnlyEntry) (stop bool)) error
	StreamForEach(func(entry Entry) error) error
	StreamForEachKeyOnly(func(entry KeyOnlyEntry) error) error
	StreamForEachPrefix(keyPrefix KeyPrefix, do func(entry Entry) error) error
	StreamForEachPrefixKeyOnly(keyPrefix KeyPrefix, do func(entry KeyOnlyEntry) error) error

	// Transactions
	Apply(set []Entry, delete []Key) error
}
