package utxo

import (
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

type OutputConsumer func(output *Output) bool

func (o *Output) databaseAddressKey() []byte {
	switch output := o.output.(type) {
	case *iotago.ExtendedOutput:
		bytes, _ := output.Address.Serialize(serializer.DeSeriModeNoValidation, nil)
		return bytes
	case *iotago.AliasOutput:
		return output.AliasID[:]
	case *iotago.NFTOutput:
		return output.NFTID[:]
	case *iotago.FoundryOutput:
		foundryID, err := output.ID()
		if err != nil {
			panic(err)
		}
		return foundryID[:]
	default:
		panic("Unknown output type")
	}
}

func (o *Output) byAddressDatabaseKey(spent bool, spendingConstraints bool) []byte {
	ms := marshalutil.New(70)
	if spent {
		ms.WriteByte(UTXOStoreKeyPrefixOutputOnAddressSpent) // 1 byte
	} else {
		ms.WriteByte(UTXOStoreKeyPrefixOutputOnAddressUnspent) // 1 byte
	}
	ms.WriteBytes(o.databaseAddressKey()) // 21-33 bytes
	ms.WriteBool(spendingConstraints)     // 1 byte
	ms.WriteByte(byte(o.OutputType()))    // 1 byte
	ms.WriteBytes(o.outputID[:])          // 34 bytes
	return ms.Bytes()
}

func (o *Output) aliasDatabaseKey(spent bool) []byte {
	ms := marshalutil.New(70)
	if spent {
		ms.WriteByte(UTXOStoreKeyPrefixAliasSpent) // 1 byte
	} else {
		ms.WriteByte(UTXOStoreKeyPrefixAliasUnspent) // 1 byte
	}
	ms.WriteBytes(o.databaseAddressKey()) // 20 bytes
	ms.WriteBytes(o.outputID[:])          // 34 bytes
	return ms.Bytes()
}

func (o *Output) nftDatabaseKey(spent bool) []byte {
	ms := marshalutil.New(70)
	if spent {
		ms.WriteByte(UTXOStoreKeyPrefixNFTSpent) // 1 byte
	} else {
		ms.WriteByte(UTXOStoreKeyPrefixNFTUnspent) // 1 byte
	}
	ms.WriteBytes(o.databaseAddressKey()) // 20 bytes
	ms.WriteBytes(o.outputID[:])          // 34 bytes
	return ms.Bytes()
}

func (o *Output) foundryDatabaseKey(spent bool) []byte {
	ms := marshalutil.New(70)
	if spent {
		ms.WriteByte(UTXOStoreKeyPrefixFoundrySpent) // 1 byte
	} else {
		ms.WriteByte(UTXOStoreKeyPrefixFoundryUnspent) // 1 byte
	}
	ms.WriteBytes(o.databaseAddressKey()) // 20 bytes
	ms.WriteBytes(o.outputID[:])          // 34 bytes
	return ms.Bytes()
}

func (o *Output) unspentDatabaseKey() []byte {
	switch output := o.output.(type) {
	case *iotago.ExtendedOutput:
		return o.byAddressDatabaseKey(false, output.FeatureBlocks().HasConstraints())
	case *iotago.AliasOutput:
		return o.aliasDatabaseKey(false)
	case *iotago.NFTOutput:
		return o.nftDatabaseKey(false)
	case *iotago.FoundryOutput:
		return o.foundryDatabaseKey(false)
	default:
		panic("Unknown output type")
	}
}

func outputIDFromDatabaseKey(key []byte) (*iotago.OutputID, error) {

	ms := marshalutil.New(key)
	prefix, err := ms.ReadByte() // prefix
	if err != nil {
		return nil, err
	}

	switch prefix {
	case UTXOStoreKeyPrefixOutputOnAddressUnspent, UTXOStoreKeyPrefixOutputOnAddressSpent:
		if _, err := parseAddress(ms); err != nil {
			return nil, err
		}
		ms.ReadSeek(ms.ReadOffset() + 2) // Spending Contrainsts + Output type
	case UTXOStoreKeyPrefixNFTUnspent, UTXOStoreKeyPrefixNFTSpent:
		ms.ReadSeek(ms.ReadOffset() + iotago.NFTIDLength)
	case UTXOStoreKeyPrefixAliasUnspent, UTXOStoreKeyPrefixAliasSpent:
		ms.ReadSeek(ms.ReadOffset() + iotago.AliasIDLength)
	case UTXOStoreKeyPrefixFoundryUnspent, UTXOStoreKeyPrefixFoundrySpent:
		ms.ReadSeek(ms.ReadOffset() + iotago.FoundryIDLength)
	}

	return ParseOutputID(ms)
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

func (u *Manager) IsOutputUnspent(outputID *iotago.OutputID) (bool, error) {
	u.ReadLockLedger()
	defer u.ReadUnlockLedger()

	output, err := u.ReadOutputByOutputIDWithoutLocking(outputID)
	if err != nil {
		return false, err
	}

	return u.IsOutputUnspentWithoutLocking(output)
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
		outputKey := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, outputID[:])

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
