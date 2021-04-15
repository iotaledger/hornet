package utxo

type kvStorable interface {
	kvStorableKey() (key []byte)
	kvStorableValue() (value []byte)
	kvStorableLoad(utxoManager *Manager, key []byte, value []byte) error
}
