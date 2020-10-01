package utxo

import (
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go"
)

type OutputConsumer func(output *Output) bool

func markAsUnspent(output *Output, mutations kvstore.BatchedMutations) error {
	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixUnspent}, output.UTXOKey())
	return mutations.Set(key, []byte{})
}

func deleteFromUnspent(output *Output, mutations kvstore.BatchedMutations) error {
	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixUnspent}, output.UTXOKey())
	return mutations.Delete(key)
}

func IsOutputUnspent(utxoInputId iotago.UTXOInputID) (bool, error) {
	ReadLockLedger()
	defer ReadUnlockLedger()

	output, err := ReadOutputForTransactionWithoutLocking(utxoInputId)
	if err != nil {
		return false, err
	}

	return output.IsUnspentWithoutLocking()
}

func ForEachUnspentOutput(consumer OutputConsumer, address ...*iotago.Ed25519Address) error {

	ReadLockLedger()
	defer ReadUnlockLedger()

	var innerError error

	key := []byte{UTXOStoreKeyPrefixUnspent}
	if len(address) > 0 {
		if len(address[0]) != iotago.Ed25519AddressBytesLength {
			return ErrInvalidAddressSize
		}
		key = byteutils.ConcatBytes(key, address[0][:])
	}

	if err := utxoStorage.IterateKeys(key, func(key kvstore.Key) bool {

		outputKey := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, key[1+iotago.Ed25519AddressBytesLength:])

		value, err := utxoStorage.Get(outputKey)
		if err != nil {
			innerError = err
			return false
		}

		output := &Output{}
		if err := output.kvStorableLoad(outputKey[1:], value); err != nil {
			innerError = err
			return false
		}

		return consumer(output)
	}); err != nil {
		return err
	}

	return innerError
}

func UnspentOutputsForAddress(address *iotago.Ed25519Address) ([]*Output, error) {

	var outputs []*Output

	consumerFunc := func(output *Output) bool {
		outputs = append(outputs, output)
		return true
	}

	err := ForEachUnspentOutput(consumerFunc, address)

	return outputs, err
}
