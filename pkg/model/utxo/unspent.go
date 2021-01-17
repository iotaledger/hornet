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

func (u *Manager) IsOutputUnspentWithoutLocking(output *Output) (bool, error) {
	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixUnspent}, output.UTXOKey())
	return u.utxoStorage.Has(key)
}

func (u *Manager) IsOutputUnspent(outputID *iotago.UTXOInputID) (bool, error) {
	u.ReadLockLedger()
	defer u.ReadUnlockLedger()

	output, err := u.ReadOutputByOutputIDWithoutLocking(outputID)
	if err != nil {
		return false, err
	}

	return u.IsOutputUnspentWithoutLocking(output)
}

func (u *Manager) ForEachUnspentOutputWithoutLocking(consumer OutputConsumer, address ...*iotago.Ed25519Address) error {

	var innerErr error

	key := []byte{UTXOStoreKeyPrefixUnspent}
	if len(address) > 0 {
		if len(address[0]) != iotago.Ed25519AddressBytesLength {
			return ErrInvalidAddressSize
		}
		key = byteutils.ConcatBytes(key, address[0][:])
	}

	if err := u.utxoStorage.IterateKeys(key, func(key kvstore.Key) bool {

		outputKey := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, key[1+iotago.Ed25519AddressBytesLength:])

		value, err := u.utxoStorage.Get(outputKey)
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

func (u *Manager) ForEachUnspentOutput(consumer OutputConsumer, address ...*iotago.Ed25519Address) error {

	u.ReadLockLedger()
	defer u.ReadUnlockLedger()

	return u.ForEachUnspentOutputWithoutLocking(consumer, address...)
}

func (u *Manager) UnspentOutputsForAddress(address *iotago.Ed25519Address, lockLedger bool, maxFind ...int) ([]*Output, error) {

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

	if lockLedger {
		if err := u.ForEachUnspentOutput(consumerFunc, address); err != nil {
			return nil, err
		}
	} else {
		if err := u.ForEachUnspentOutputWithoutLocking(consumerFunc, address); err != nil {
			return nil, err
		}
	}

	return outputs, nil
}

func (u *Manager) ComputeAddressBalance(address *iotago.Ed25519Address, lockLedger bool, maxFind ...int) (balance uint64, count int, err error) {

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

	if lockLedger {
		if err := u.ForEachUnspentOutput(consumerFunc, address); err != nil {
			return 0, 0, err
		}
	} else {
		if err := u.ForEachUnspentOutputWithoutLocking(consumerFunc, address); err != nil {
			return 0, 0, err
		}
	}

	return balance, i, nil
}
