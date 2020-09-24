package utxo

type kvStorable interface {
	kvStorableKey() (key []byte)
	kvStorableValue() (value []byte)
	kvStorableLoad(key []byte, value []byte) error
}
