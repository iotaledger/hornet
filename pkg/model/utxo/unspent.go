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

func IsOutputUnspent(outputID *iotago.UTXOInputID) (bool, error) {
	ReadLockLedger()
	defer ReadUnlockLedger()

	output, err := ReadOutputByOutputIDWithoutLocking(outputID)
	if err != nil {
		return false, err
	}

	return output.IsUnspentWithoutLocking()
}

func ForEachUnspentOutputWithoutLocking(consumer OutputConsumer, address ...*iotago.Ed25519Address) error {

	var innerErr error

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
			innerErr = err
			return false
		}

		output := &Output{}
		if err := output.kvStorableLoad(outputKey[1:], value); err != nil {
			innerErr = err
			return false
		}

		return consumer(output)
	}); err != nil {
		return err
	}

	return innerErr
}

func ForEachUnspentOutput(consumer OutputConsumer, address ...*iotago.Ed25519Address) error {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return ForEachUnspentOutputWithoutLocking(consumer, address...)
}

func UnspentOutputsForAddress(address *iotago.Ed25519Address, maxFind ...int) ([]*Output, error) {

	var outputs []*Output

	i := 0
	consumerFunc := func(output *Output) bool {
		i++

		if (len(maxFind) > 0) && (i > maxFind[0]) {
			return false
		}

		outputs = append(outputs, output)
		return true
	}

	if err := ForEachUnspentOutput(consumerFunc, address); err != nil {
		return nil, err
	}

	return outputs, nil
}

func AddressBalance(address *iotago.Ed25519Address, maxFind ...int) (balance uint64, count int, err error) {

	balance = 0
	i := 0
	consumerFunc := func(output *Output) bool {
		i++

		if (len(maxFind) > 0) && (i > maxFind[0]) {
			return false
		}

		balance += output.amount
		return true
	}

	if err := ForEachUnspentOutput(consumerFunc, address); err != nil {
		return 0, 0, err
	}

	return balance, i, nil
}
