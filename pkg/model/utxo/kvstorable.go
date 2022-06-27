package utxo

type kvStorable interface {
	KVStorableKey() (key []byte)
	KVStorableValue() (value []byte)
	kvStorableLoad(utxoManager *Manager, key []byte, value []byte) error
}
