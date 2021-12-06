package utxo

import (
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
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

	key := []byte{UTXOStoreKeyPrefixOutputSpent}

	if opt.filterOutputType != nil {
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
	if opt.filterAddress != nil {
		addrBytes, err := opt.filterAddress.Serialize(serializer.DeSeriModeNoValidation, nil)
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

type UTXOIterateOptions struct {
	readLockLedger           bool
	maxResultCount           int
	filterAddress            iotago.Address
	filterAliasID            *iotago.AliasID
	filterNFTID              *iotago.NFTID
	filterFoundryID          *iotago.FoundryID
	filterSpendingContraints *bool
	filterOutputType         *iotago.OutputType
}

type UTXOIterateOption func(*UTXOIterateOptions)

func ReadLockLedger(lockLedger bool) UTXOIterateOption {
	return func(args *UTXOIterateOptions) {
		args.readLockLedger = lockLedger
	}
}

func MaxResultCount(count int) UTXOIterateOption {
	return func(args *UTXOIterateOptions) {
		args.maxResultCount = count
	}
}

func FilterAddress(address iotago.Address) UTXOIterateOption {
	return func(args *UTXOIterateOptions) {
		args.filterAddress = address
	}
}

func FilterAliasID(aliasID iotago.AliasID) UTXOIterateOption {
	return func(args *UTXOIterateOptions) {
		args.filterAliasID = &aliasID
	}
}

func FilterNFTID(nftID iotago.NFTID) UTXOIterateOption {
	return func(args *UTXOIterateOptions) {
		args.filterNFTID = &nftID
	}
}

func FilterFoundryID(foundryID iotago.FoundryID) UTXOIterateOption {
	return func(args *UTXOIterateOptions) {
		args.filterFoundryID = &foundryID
	}
}

func FilterSpendingContraints(spendingContraints bool) UTXOIterateOption {
	return func(args *UTXOIterateOptions) {
		args.filterSpendingContraints = &spendingContraints
	}
}

func FilterOutputType(outputType iotago.OutputType) UTXOIterateOption {
	return func(args *UTXOIterateOptions) {
		args.filterOutputType = &outputType
	}
}

func iterateOptions(optionalOptions []UTXOIterateOption) *UTXOIterateOptions {
	result := &UTXOIterateOptions{
		readLockLedger:           true,
		maxResultCount:           0,
		filterAddress:            nil,
		filterAliasID:            nil,
		filterNFTID:              nil,
		filterFoundryID:          nil,
		filterSpendingContraints: nil,
		filterOutputType:         nil,
	}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}

	return result
}

func (u *Manager) SpentOutputs(options ...UTXOIterateOption) (Spents, error) {

	var spents []*Spent

	consumerFunc := func(spent *Spent) bool {
		spents = append(spents, spent)
		return true
	}

	if err := u.ForEachSpentOutput(consumerFunc, options...); err != nil {
		return nil, err
	}

	return spents, nil
}

func (u *Manager) UnspentOutputs(options ...UTXOIterateOption) ([]*Output, error) {

	var outputs []*Output
	consumerFunc := func(output *Output) bool {
		outputs = append(outputs, output)
		return true
	}

	if err := u.ForEachUnspentOutput(consumerFunc, options...); err != nil {
		return nil, err
	}

	return outputs, nil
}

func (u *Manager) ComputeBalance(options ...UTXOIterateOption) (balance uint64, count int, err error) {

	balance = 0
	count = 0
	consumerFunc := func(output *Output) bool {
		balance += output.Amount()
		count++
		return true
	}

	if err := u.ForEachUnspentOutput(consumerFunc, options...); err != nil {
		return 0, 0, err
	}

	return balance, count, nil
}
