package utxo

import (
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2"
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

func MaxResultCount(count int) UTXOIterateOption {
	return func(args *UTXOIterateOptions) {
		args.maxResultCount = count
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

func (u *Manager) forEachUnspentOutput(consumer OutputConsumer, keyPrefix kvstore.KeyPrefix, readLockLedger bool, maxResultCount int) error {
	if readLockLedger {
		u.ReadLockLedger()
		defer u.ReadUnlockLedger()
	}

	var innerErr error
	var i int
	if err := u.utxoStorage.IterateKeys(keyPrefix, func(key kvstore.Key) bool {
		if (maxResultCount > 0) && (i >= maxResultCount) {
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

		return consumer(output)
	}); err != nil {
		return err
	}

	return innerErr
}

func (u *Manager) ForEachUnspentExtendedOutput(filterAddress iotago.Address, consumer OutputConsumer, options ...UTXOIterateOption) error {
	opt := iterateOptions(options)
	key := []byte{UTXOStoreKeyPrefixExtendedOutputUnspent}

	// Filter by Address
	if filterAddress != nil {
		addrBytes, err := filterAddress.Serialize(serializer.DeSeriModeNoValidation, nil)
		if err != nil {
			return err
		}
		key = byteutils.ConcatBytes(key, addrBytes[:])
	}
	return u.forEachUnspentOutput(consumer, key, opt.readLockLedger, opt.maxResultCount)
}

func (u *Manager) ForEachUnspentNFTOutput(filterNFTID *iotago.NFTID, consumer OutputConsumer, options ...UTXOIterateOption) error {
	opt := iterateOptions(options)
	key := []byte{UTXOStoreKeyPrefixNFTUnspent}

	// Filter by ID
	if filterNFTID != nil {
		key = byteutils.ConcatBytes(key, (*filterNFTID)[:])
	}
	return u.forEachUnspentOutput(consumer, key, opt.readLockLedger, opt.maxResultCount)
}

func (u *Manager) ForEachUnspentAliasOutput(filterAliasID *iotago.AliasID, consumer OutputConsumer, options ...UTXOIterateOption) error {
	opt := iterateOptions(options)
	key := []byte{UTXOStoreKeyPrefixAliasUnspent}

	// Filter by ID
	if filterAliasID != nil {
		key = byteutils.ConcatBytes(key, (*filterAliasID)[:])
	}
	return u.forEachUnspentOutput(consumer, key, opt.readLockLedger, opt.maxResultCount)
}

func (u *Manager) ForEachUnspentFoundryOutput(filterFoundryID *iotago.FoundryID, consumer OutputConsumer, options ...UTXOIterateOption) error {
	opt := iterateOptions(options)
	key := []byte{UTXOStoreKeyPrefixFoundryUnspent}

	// Filter by ID
	if filterFoundryID != nil {
		key = byteutils.ConcatBytes(key, (*filterFoundryID)[:])
	}
	return u.forEachUnspentOutput(consumer, key, opt.readLockLedger, opt.maxResultCount)
}

func (u *Manager) ForEachUnspentOutputWithIssuer(issuer iotago.Address, filterOptions *FilterOptions, consumer OutputConsumer, options ...UTXOIterateOption) error {
	opt := iterateOptions(options)
	addrBytes, err := issuer.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return err
	}
	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixIssuerLookup}, addrBytes[:])
	if filterOptions != nil {
		if filterOptions.filterOutputType != nil {
			key = byteutils.ConcatBytes(key, []byte{byte(*filterOptions.filterOutputType)})
		}
	}

	return u.forEachUnspentOutput(consumer, key, opt.readLockLedger, opt.maxResultCount)
}

func (u *Manager) ForEachUnspentOutputWithSender(sender iotago.Address, filterOptions *FilterOptions, consumer OutputConsumer, options ...UTXOIterateOption) error {
	opt := iterateOptions(options)
	addrBytes, err := sender.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return err
	}
	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSenderLookup}, addrBytes[:])
	if filterOptions != nil {
		if filterOptions.filterOutputType != nil {
			key = byteutils.ConcatBytes(key, []byte{byte(*filterOptions.filterOutputType)})
		}
	}
	return u.forEachUnspentOutput(consumer, key, opt.readLockLedger, opt.maxResultCount)
}

func (u *Manager) ForEachUnspentOutputWithSenderAndIndexTag(sender iotago.Address, indexTag []byte, filterOptions *FilterOptions, consumer OutputConsumer, options ...UTXOIterateOption) error {
	if len(indexTag) > iotago.MaxIndexationTagLength {
		indexTag = indexTag[:iotago.MaxIndexationTagLength]
	}
	opt := iterateOptions(options)
	addrBytes, err := sender.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return err
	}
	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSenderAndIndexLookup}, addrBytes[:], append(indexTag, make([]byte, iotago.MaxIndexationTagLength-len(indexTag))...))
	if filterOptions != nil {
		if filterOptions.filterOutputType != nil {
			key = byteutils.ConcatBytes(key, []byte{byte(*filterOptions.filterOutputType)})
		}
	}

	return u.forEachUnspentOutput(consumer, key, opt.readLockLedger, opt.maxResultCount)
}

type FilterOptions struct {
	filterHasSpendingConstraints *bool
	filterOutputType             *iotago.OutputType
}

func FilterOutputType(outputType iotago.OutputType) *FilterOptions {
	opts := &FilterOptions{}
	return opts.FilterOutputType(outputType)
}

func (f *FilterOptions) FilterOutputType(outputType iotago.OutputType) *FilterOptions {
	oType := outputType
	f.filterOutputType = &oType
	return f
}

func FilterHasSpendingConstraints(hasSpendingConstraints bool) *FilterOptions {
	opts := &FilterOptions{}
	return opts.FilterHasSpendingConstraints(hasSpendingConstraints)
}

func (f *FilterOptions) FilterHasSpendingConstraints(hasSpendingConstraints bool) *FilterOptions {
	constraints := hasSpendingConstraints
	f.filterHasSpendingConstraints = &constraints
	return f
}

func (u *Manager) ForEachUnspentOutputOnAddress(address iotago.Address, filterOptions *FilterOptions, consumer OutputConsumer, options ...UTXOIterateOption) error {
	opt := iterateOptions(options)

	ms := marshalutil.New(36)
	ms.WriteByte(UTXOStoreKeyPrefixAddressLookup)
	addrBytes, err := address.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return err
	}
	ms.WriteBytes(addrBytes)

	consumerFunc := consumer
	if filterOptions != nil {
		keyPrefixIterationPossible := true
		if filterOptions.filterHasSpendingConstraints != nil {
			ms.WriteBool(*filterOptions.filterHasSpendingConstraints)
		} else {
			// If we are not filtering with spending constraints, we cannot use keyPrefix iteration
			keyPrefixIterationPossible = false
		}

		if filterOptions.filterOutputType != nil {
			if keyPrefixIterationPossible {
				ms.WriteByte(byte(*filterOptions.filterOutputType))
			} else {
				consumerFunc = func(output *Output) bool {
					if output.OutputType() == *filterOptions.filterOutputType {
						return consumer(output)
					}
					return true
				}
			}
		}
	}

	return u.forEachUnspentOutput(consumerFunc, ms.Bytes(), opt.readLockLedger, opt.maxResultCount)
}

func (u *Manager) ForEachUnspentOutput(consumer OutputConsumer, options ...UTXOIterateOption) error {
	opt := iterateOptions(options)

	if opt.readLockLedger {
		u.ReadLockLedger()
		defer u.ReadUnlockLedger()
	}

	if opt.maxResultCount != 0 {
		panic("invalid filter options")
	}

	if err := u.ForEachUnspentExtendedOutput(nil, consumer); err != nil {
		return err
	}
	if err := u.ForEachUnspentNFTOutput(nil, consumer); err != nil {
		return err
	}
	if err := u.ForEachUnspentAliasOutput(nil, consumer); err != nil {
		return err
	}
	if err := u.ForEachUnspentFoundryOutput(nil, consumer); err != nil {
		return err
	}
	return nil
}

func (u *Manager) UnspentExtendedOutputs(filterAddress iotago.Address, options ...UTXOIterateOption) (Outputs, error) {
	var outputs Outputs
	consumerFunc := func(output *Output) bool {
		outputs = append(outputs, output)
		return true
	}

	if err := u.ForEachUnspentExtendedOutput(filterAddress, consumerFunc, options...); err != nil {
		return nil, err
	}
	return outputs, nil
}

func (u *Manager) UnspentNFTOutputs(filterNFTID *iotago.NFTID, options ...UTXOIterateOption) (Outputs, error) {
	var outputs Outputs
	consumerFunc := func(output *Output) bool {
		outputs = append(outputs, output)
		return true
	}

	if err := u.ForEachUnspentNFTOutput(filterNFTID, consumerFunc, options...); err != nil {
		return nil, err
	}
	return outputs, nil
}

func (u *Manager) UnspentAliasOutputs(filterAliasID *iotago.AliasID, options ...UTXOIterateOption) (Outputs, error) {
	var outputs Outputs
	consumerFunc := func(output *Output) bool {
		outputs = append(outputs, output)
		return true
	}

	if err := u.ForEachUnspentAliasOutput(filterAliasID, consumerFunc, options...); err != nil {
		return nil, err
	}
	return outputs, nil
}

func (u *Manager) UnspentFoundryOutputs(filterFoundryID *iotago.FoundryID, options ...UTXOIterateOption) (Outputs, error) {
	var outputs Outputs
	consumerFunc := func(output *Output) bool {
		outputs = append(outputs, output)
		return true
	}

	if err := u.ForEachUnspentFoundryOutput(filterFoundryID, consumerFunc, options...); err != nil {
		return nil, err
	}
	return outputs, nil
}

func (u *Manager) UnspentOutputsOnAddress(address iotago.Address, filterOptions *FilterOptions, options ...UTXOIterateOption) (Outputs, error) {
	var outputs Outputs
	consumerFunc := func(output *Output) bool {
		outputs = append(outputs, output)
		return true
	}

	if err := u.ForEachUnspentOutputOnAddress(address, filterOptions, consumerFunc, options...); err != nil {
		return nil, err
	}
	return outputs, nil
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

func (u *Manager) ComputeAddressBalance(address iotago.Address, filterOptions *FilterOptions, options ...UTXOIterateOption) (balance uint64, count int, err error) {
	balance = 0
	count = 0
	consumerFunc := func(output *Output) bool {
		count++
		balance += output.Amount()
		return true
	}
	if err := u.ForEachUnspentOutputOnAddress(address, filterOptions, consumerFunc, options...); err != nil {
		return 0, 0, err
	}
	return balance, count, err
}

func (u *Manager) ComputeLedgerBalance(options ...UTXOIterateOption) (balance uint64, count int, err error) {
	balance = 0
	count = 0
	consumerFunc := func(output *Output) bool {
		count++
		balance += output.Amount()
		return true
	}

	if err := u.ForEachUnspentOutput(consumerFunc, options...); err != nil {
		return 0, 0, err
	}
	return balance, count, nil
}
