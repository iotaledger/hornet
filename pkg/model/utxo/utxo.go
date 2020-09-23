package utxo

import (
	"sync"

	"github.com/iotaledger/hive.go/kvstore"
)

var (
	utxoLock sync.RWMutex
)

func ConfigureStorages(store kvstore.KVStore) {
	configureOutputsStorage(store)
}

func ReadLockLedger() {
	utxoLock.RLock()
}

func ReadUnlockLedger() {
	utxoLock.RUnlock()
}

func WriteLockLedger() {
	utxoLock.Lock()
}

func WriteUnlockLedger() {
	utxoLock.Unlock()
}

type KVStorable interface {
	StorageKey() (key []byte)
	StorageValue() (value []byte)
	FromStorage(key []byte, value []byte)
}
