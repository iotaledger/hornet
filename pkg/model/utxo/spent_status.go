package utxo

import (
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

type OutputConsumer func(output *Output) bool

type databaseKey []byte

func (o *Output) databaseAddressKey() databaseKey {
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

func (o *Output) byAddressDatabaseKey(spent bool, spendingConstraints bool) databaseKey {
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

func (o *Output) aliasDatabaseKey(spent bool) databaseKey {
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

func (o *Output) nftDatabaseKey(spent bool) databaseKey {
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

func (o *Output) foundryDatabaseKey(spent bool) databaseKey {
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

func (o *Output) unspentDatabaseKeys() []databaseKey {
	switch output := o.output.(type) {
	case *iotago.ExtendedOutput:
		return []databaseKey{o.byAddressDatabaseKey(false, output.FeatureBlocks().HasConstraints())}
	case *iotago.AliasOutput:
		return []databaseKey{o.aliasDatabaseKey(false)}
	case *iotago.NFTOutput:
		return []databaseKey{o.nftDatabaseKey(false)}
	case *iotago.FoundryOutput:
		return []databaseKey{o.foundryDatabaseKey(false)}
	default:
		panic("Unknown output type")
	}
}

func (o *Output) spentDatabaseKeys() []databaseKey {
	switch output := o.output.(type) {
	case *iotago.ExtendedOutput:
		return []databaseKey{o.byAddressDatabaseKey(true, output.FeatureBlocks().HasConstraints())}
	case *iotago.AliasOutput:
		return []databaseKey{o.aliasDatabaseKey(true)}
	case *iotago.NFTOutput:
		return []databaseKey{o.nftDatabaseKey(true)}
	case *iotago.FoundryOutput:
		return []databaseKey{o.foundryDatabaseKey(true)}
	default:
		panic("Unknown output type")
	}
}

func outputIDFromDatabaseKey(key databaseKey) (*iotago.OutputID, error) {

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
	for _, key := range output.spentDatabaseKeys() {
		if err := mutations.Delete(key); err != nil {
			return err
		}
	}

	for _, key := range output.unspentDatabaseKeys() {
		if err := mutations.Set(key, []byte{}); err != nil {
			return err
		}
	}
	return nil
}

func markAsSpent(output *Output, mutations kvstore.BatchedMutations) error {
	for _, key := range output.unspentDatabaseKeys() {
		if err := mutations.Delete(key); err != nil {
			return err
		}
	}

	for _, key := range output.spentDatabaseKeys() {
		if err := mutations.Set(key, []byte{}); err != nil {
			return err
		}
	}
	return nil
}

func deleteSpentUnspentMarkings(output *Output, mutations kvstore.BatchedMutations) error {
	for _, key := range output.unspentDatabaseKeys() {
		if err := mutations.Delete(key); err != nil {
			return err
		}
	}

	for _, key := range output.spentDatabaseKeys() {
		if err := mutations.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func (u *Manager) IsOutputUnspentWithoutLocking(output *Output) (bool, error) {
	// Looking up the first key should be enough, since that is the main key
	return u.utxoStorage.Has(output.unspentDatabaseKeys()[0])
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

func storeSpentAndMarkOutputAsSpent(spent *Spent, mutations kvstore.BatchedMutations) error {
	if err := storeSpent(spent, mutations); err != nil {
		return err
	}
	return markAsSpent(spent.output, mutations)
}

func deleteSpentAndMarkOutputAsUnspent(spent *Spent, mutations kvstore.BatchedMutations) error {
	if err := deleteSpent(spent, mutations); err != nil {
		return err
	}
	return markAsUnspent(spent.output, mutations)
}
