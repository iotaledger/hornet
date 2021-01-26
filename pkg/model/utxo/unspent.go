package utxo

import (
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"

	iotago "github.com/iotaledger/iota.go"
)

type OutputConsumer func(output *Output) bool

func (o *Output) unspentDatabaseKey() []byte {
	ms := marshalutil.New(69)
	ms.WriteByte(UTXOStoreKeyPrefixUnspent)
	ms.WriteBytes(o.addressBytes())
	ms.WriteByte(o.outputType)
	ms.WriteBytes(o.outputID[:])
	return ms.Bytes()
}

func outputIDBytesFromUnspentDatabaseKey(key []byte) ([]byte, error) {

	ms := marshalutil.New(key)
	_, err := ms.ReadByte() // prefix
	if err != nil {
		return nil, err
	}

	if _, err := parseAddress(ms); err != nil {
		return nil, err
	}

	_, err = ms.ReadByte() // output type
	if err != nil {
		return nil, err
	}

	return ms.ReadBytes(OutputIDLength)
}

func markAsUnspent(output *Output, mutations kvstore.BatchedMutations) error {
	return mutations.Set(output.unspentDatabaseKey(), []byte{})
}

func deleteFromUnspent(output *Output, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(output.unspentDatabaseKey())
}

func (u *Manager) IsOutputUnspentWithoutLocking(output *Output) (bool, error) {
	return u.utxoStorage.Has(output.unspentDatabaseKey())
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

func (u *Manager) ForEachUnspentOutputWithoutLocking(consumer OutputConsumer, address iotago.Address, outputType ...iotago.OutputType) error {

	var innerErr error

	key := []byte{UTXOStoreKeyPrefixUnspent}

	// Filter by address
	if address != nil {
		addrBytes, err := address.Serialize(iotago.DeSeriModeNoValidation)
		if err != nil {
			return err
		}
		key = byteutils.ConcatBytes(key, addrBytes)
	}

	// Filter by type
	if len(outputType) > 0 {
		key = byteutils.ConcatBytes(key, []byte{outputType[0]})
	}

	if err := u.utxoStorage.IterateKeys(key, func(key kvstore.Key) bool {

		outputIDBytes, err := outputIDBytesFromUnspentDatabaseKey(key)
		if err != nil {
			innerErr = err
			return false
		}
		outputKey := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, outputIDBytes)

		value, err := u.utxoStorage.Get(outputKey)
		if err != nil {
			innerErr = err
			return false
		}

		output := &Output{}
		if err := output.kvStorableLoad(u, outputKey, value); err != nil {
			innerErr = err
			return false
		}

		return consumer(output)
	}); err != nil {
		return err
	}

	return innerErr
}

func (u *Manager) ForEachUnspentOutput(consumer OutputConsumer, address iotago.Address, outputType ...iotago.OutputType) error {

	u.ReadLockLedger()
	defer u.ReadUnlockLedger()

	return u.ForEachUnspentOutputWithoutLocking(consumer, address, outputType...)
}

func (u *Manager) UnspentOutputsForAddress(address iotago.Address, lockLedger bool, maxFind ...int) ([]*Output, error) {

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

func (u *Manager) ComputeAddressBalance(address iotago.Address, lockLedger bool, maxFind ...int) (balance uint64, count int, err error) {

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
