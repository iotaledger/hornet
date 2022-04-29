package utxo

import (
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go/v3"
)

type UTXOIterateOptions struct {
	readLockLedger bool
	maxResultCount int
}

type UTXOIterateOption func(*UTXOIterateOptions)

func ReadLockLedger(lockLedger bool) UTXOIterateOption {
	return func(args *UTXOIterateOptions) {
		args.readLockLedger = lockLedger
	}
}

func iterateOptions(optionalOptions []UTXOIterateOption) *UTXOIterateOptions {
	result := &UTXOIterateOptions{
		readLockLedger: true,
		maxResultCount: 0,
	}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}

	return result
}

func (u *Manager) ForEachOutput(consumer OutputConsumer, options ...UTXOIterateOption) error {
	opt := iterateOptions(options)

	if opt.readLockLedger {
		u.ReadLockLedger()
		defer u.ReadUnlockLedger()
	}

	var innerErr error
	var i int
	if err := u.utxoStorage.Iterate([]byte{UTXOStoreKeyPrefixOutput}, func(key kvstore.Key, value kvstore.Value) bool {
		if (opt.maxResultCount > 0) && (i >= opt.maxResultCount) {
			return false
		}
		i++

		output := &Output{}
		if err := output.kvStorableLoad(u, key, value); err != nil {
			innerErr = err
			return false
		}

		return consumer(output)
	}); err != nil {
		return err
	}

	return innerErr
}

func (u *Manager) ForEachSpentOutput(consumer SpentConsumer, options ...UTXOIterateOption) error {
	opt := iterateOptions(options)

	if opt.readLockLedger {
		u.ReadLockLedger()
		defer u.ReadUnlockLedger()
	}

	key := []byte{UTXOStoreKeyPrefixOutputSpent}

	var innerErr error
	var i int
	if err := u.utxoStorage.Iterate(key, func(key kvstore.Key, value kvstore.Value) bool {
		if (opt.maxResultCount > 0) && (i >= opt.maxResultCount) {
			return false
		}
		i++

		spent := &Spent{}
		if err := spent.kvStorableLoad(u, key, value); err != nil {
			innerErr = err
			return false
		}

		if err := u.loadOutputOfSpent(spent); err != nil {
			innerErr = err
			return false
		}

		return consumer(spent)
	}); err != nil {
		return err
	}

	return innerErr
}

func (u *Manager) SpentOutputs(options ...UTXOIterateOption) (Spents, error) {
	var spents Spents
	consumerFunc := func(spent *Spent) bool {
		spents = append(spents, spent)
		return true
	}

	if err := u.ForEachSpentOutput(consumerFunc, options...); err != nil {
		return nil, err
	}
	return spents, nil
}

func (u *Manager) ForEachUnspentOutputID(consumer OutputIDConsumer, options ...UTXOIterateOption) error {
	opt := iterateOptions(options)

	if opt.readLockLedger {
		u.ReadLockLedger()
		defer u.ReadUnlockLedger()
	}

	var innerErr error
	var i int
	if err := u.utxoStorage.IterateKeys([]byte{UTXOStoreKeyPrefixOutputUnspent}, func(key kvstore.Key) bool {
		if (opt.maxResultCount > 0) && (i >= opt.maxResultCount) {
			return false
		}
		i++

		outputID, err := outputIDFromDatabaseKey(key)
		if err != nil {
			innerErr = err
			return false
		}

		return consumer(outputID)
	}); err != nil {
		return err
	}

	return innerErr
}

func (u *Manager) ForEachUnspentOutput(consumer OutputConsumer, options ...UTXOIterateOption) error {

	var innerErr error
	if err := u.ForEachUnspentOutputID(func(outputID *iotago.OutputID) bool {
		outputKey := outputStorageKeyForOutputID(outputID)

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
	}, options...); err != nil {
		return err
	}

	return innerErr
}

func (u *Manager) UnspentOutputsIDs(options ...UTXOIterateOption) ([]*iotago.OutputID, error) {
	var outputIDs []*iotago.OutputID
	consumerFunc := func(outputID *iotago.OutputID) bool {
		outputIDs = append(outputIDs, outputID)
		return true
	}

	if err := u.ForEachUnspentOutputID(consumerFunc, options...); err != nil {
		return nil, err
	}
	return outputIDs, nil
}

func (u *Manager) UnspentOutputs(options ...UTXOIterateOption) (Outputs, error) {
	var outputs Outputs
	consumerFunc := func(output *Output) bool {
		outputs = append(outputs, output)
		return true
	}

	if err := u.ForEachUnspentOutput(consumerFunc, options...); err != nil {
		return nil, err
	}
	return outputs, nil
}

func (u *Manager) ComputeLedgerBalance(options ...UTXOIterateOption) (balance uint64, count int, err error) {
	balance = 0
	count = 0
	consumerFunc := func(output *Output) bool {
		count++
		balance += output.Deposit()
		return true
	}

	if err := u.ForEachUnspentOutput(consumerFunc, options...); err != nil {
		return 0, 0, err
	}
	return balance, count, nil
}
