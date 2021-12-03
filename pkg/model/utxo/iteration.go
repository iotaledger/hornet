package utxo

import (
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/serializer/v2"
)

func (u *Manager) ForEachOutput(consumer OutputConsumer, options ...UTXOIterateOption) error {

	opt := iterateOptions(options)

	if opt.readLockLedger {
		u.ReadLockLedger()
		defer u.ReadUnlockLedger()
	}

	consumerFunc := consumer

	if opt.filterOutputType != nil {

		filterType := *opt.filterOutputType

		consumerFunc = func(output *Output) bool {
			if output.OutputType() == filterType {
				return consumer(output)
			}
			return true
		}
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

		return consumerFunc(output)
	}); err != nil {
		return err
	}

	return innerErr
}

func (u *Manager) ForEachSpentOutput(consumer SpentConsumer, options ...UTXOIterateOption) error {

	consumerFunc := consumer

	opt := iterateOptions(options)

	if opt.readLockLedger {
		u.ReadLockLedger()
		defer u.ReadUnlockLedger()
	}

	var innerErr error

	key := []byte{UTXOStoreKeyPrefixOutputOnAddressSpent}

	// Filter by address
	if opt.address != nil {
		addrBytes, err := opt.address.Serialize(serializer.DeSeriModeNoValidation, nil)
		if err != nil {
			return err
		}
		key = byteutils.ConcatBytes(key, addrBytes)

		// Filter by output type
		if opt.filterOutputType != nil {
			key = byteutils.ConcatBytes(key, []byte{byte(*opt.filterOutputType)})
		}
	} else if opt.filterOutputType != nil {
		// Filter results instead of using prefix iteration
		consumerFunc = func(spent *Spent) bool {
			if spent.OutputType() == *opt.filterOutputType {
				return consumer(spent)
			}
			return true
		}
	}

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

		return consumerFunc(spent)
	}); err != nil {
		return err
	}

	return innerErr
}

func (u *Manager) ForEachUnspentOutput(consumer OutputConsumer, options ...UTXOIterateOption) error {

	consumerFunc := consumer

	opt := iterateOptions(options)

	if opt.readLockLedger {
		u.ReadLockLedger()
		defer u.ReadUnlockLedger()
	}

	var innerErr error

	key := []byte{UTXOStoreKeyPrefixOutputOnAddressUnspent}

	// Filter by address
	if opt.address != nil {
		addrBytes, err := opt.address.Serialize(serializer.DeSeriModeNoValidation, nil)
		if err != nil {
			return err
		}
		key = byteutils.ConcatBytes(key, addrBytes)

		// Filter by type
		if opt.filterOutputType != nil {
			key = byteutils.ConcatBytes(key, []byte{byte(*opt.filterOutputType)})
		}
	} else if opt.filterOutputType != nil {

		// Filter results instead of using prefix iteration
		consumerFunc = func(output *Output) bool {
			if output.OutputType() == *opt.filterOutputType {
				return consumer(output)
			}
			return true
		}
	}

	var i int

	if err := u.utxoStorage.IterateKeys(key, func(key kvstore.Key) bool {

		if (opt.maxResultCount > 0) && (i >= opt.maxResultCount) {
			return false
		}

		i++

		outputID, err := outputIDFromDatabaseKey(key)
		if err != nil {
			innerErr = err
			return false
		}
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

		return consumerFunc(output)
	}); err != nil {
		return err
	}

	return innerErr
}
