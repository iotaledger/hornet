package utxo

import (
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

type OutputConsumer func(output *Output) bool

type lookupKey []byte

func lookupKeyExtendedOutputByAddress(address iotago.Address, outputID *iotago.OutputID) lookupKey {
	ms := marshalutil.New(68)
	ms.WriteByte(UTXOStoreKeyPrefixLookupExtendedOutputs) // 1 byte
	addressBytes, _ := address.Serialize(serializer.DeSeriModeNoValidation, nil)
	ms.WriteBytes(addressBytes) // 21-33 bytes
	ms.WriteBytes(outputID[:])  // 34 bytes
	return ms.Bytes()
}

func lookupKeyAliasOutputByAliasID(aliasID iotago.AliasID, outputID *iotago.OutputID) lookupKey {
	ms := marshalutil.New(55)
	ms.WriteByte(UTXOStoreKeyPrefixLookupAliasOutputs) // 1 byte
	ms.WriteBytes(aliasID[:])                          // 20 bytes
	ms.WriteBytes(outputID[:])                         // 34 bytes
	return ms.Bytes()
}

func lookupKeyNFTOutputyByNFTID(nftID iotago.NFTID, outputID *iotago.OutputID) lookupKey {
	ms := marshalutil.New(55)
	ms.WriteByte(UTXOStoreKeyPrefixLookupNFTOutputs) // 1 byte
	ms.WriteBytes(nftID[:])                          // 20 bytes
	ms.WriteBytes(outputID[:])                       // 34 bytes
	return ms.Bytes()
}

func lookupKeyFoundryOutputByFoundryID(foundryID iotago.FoundryID, outputID *iotago.OutputID) lookupKey {
	ms := marshalutil.New(61)
	ms.WriteByte(UTXOStoreKeyPrefixLookupFoundryOutputs) // 1 byte
	ms.WriteBytes(foundryID[:])                          // 26 bytes
	ms.WriteBytes(outputID[:])                           // 34 bytes
	return ms.Bytes()
}

func lookupKeyByAddress(address iotago.Address, spendingConstraints bool, outputType iotago.OutputType, outputID *iotago.OutputID) lookupKey {
	ms := marshalutil.New(70)
	ms.WriteByte(UTXOStoreKeyPrefixLookupByAddress) // 1 byte
	addressBytes, _ := address.Serialize(serializer.DeSeriModeNoValidation, nil)
	ms.WriteBytes(addressBytes)       // 21-33 bytes
	ms.WriteBool(spendingConstraints) // 1 byte
	ms.WriteByte(byte(outputType))    // 1 byte
	ms.WriteBytes(outputID[:])        // 34 bytes
	return ms.Bytes()
}

func lookupKeysForFeatureBlocks(blocks iotago.FeatureBlocks, outputType iotago.OutputType, outputID *iotago.OutputID) []lookupKey {

	blockSet := blocks.MustSet()
	var keys []lookupKey
	if issuerBlock := blockSet.IssuerFeatureBlock(); issuerBlock != nil {
		ms := marshalutil.New(69)
		ms.WriteByte(UTXOStoreKeyPrefixLookupByIssuer) // 1 byte
		addressBytes, _ := issuerBlock.Address.Serialize(serializer.DeSeriModeNoValidation, nil)
		ms.WriteBytes(addressBytes)    // 21-33 bytes
		ms.WriteByte(byte(outputType)) // 1 byte
		ms.WriteBytes(outputID[:])     // 34 bytes
		keys = append(keys, ms.Bytes())
	}

	if senderBlock := blockSet.SenderFeatureBlock(); senderBlock != nil {
		ms := marshalutil.New(69)
		ms.WriteByte(UTXOStoreKeyPrefixLookupBySender) // 1 byte
		addressBytes, _ := senderBlock.Address.Serialize(serializer.DeSeriModeNoValidation, nil)
		ms.WriteBytes(addressBytes)    // 21-33 bytes
		ms.WriteByte(byte(outputType)) // 1 byte
		ms.WriteBytes(outputID[:])     // 34 bytes
		keys = append(keys, ms.Bytes())

		if indexationBlock := blockSet.IndexationFeatureBlock(); indexationBlock != nil {

			paddedTag := func(tag []byte) []byte {
				return append(tag, make([]byte, iotago.MaxIndexationTagLength-len(tag))...)
			}

			ms := marshalutil.New(133)
			ms.WriteByte(UTXOStoreKeyPrefixLoolupBySenderAndIndex) // 1 byte
			addressBytes, _ := senderBlock.Address.Serialize(serializer.DeSeriModeNoValidation, nil)
			ms.WriteBytes(addressBytes)                   // 21-33 bytes
			ms.WriteBytes(paddedTag(indexationBlock.Tag)) // 64 bytes
			ms.WriteByte(byte(outputType))                // 1 byte
			ms.WriteBytes(outputID[:])                    // 34 bytes
			keys = append(keys, ms.Bytes())
		}
	}

	return keys
}

func (o *Output) lookupKeys() []lookupKey {
	return append(append(o.unspentLookupKeys(), o.addressLookupKeys()...), o.featureLookupKeys()...)
}

func (o *Output) unspentLookupKeys() []lookupKey {
	switch output := o.output.(type) {
	case *iotago.ExtendedOutput:
		return []lookupKey{
			lookupKeyExtendedOutputByAddress(output.Address, o.outputID),
		}
	case *iotago.AliasOutput:

		aliasID := output.AliasID
		if aliasID.Empty() {
			// Use implicit AliasID
			aliasID = iotago.AliasIDFromOutputID(*o.outputID)
		}

		return []lookupKey{
			lookupKeyAliasOutputByAliasID(aliasID, o.outputID),
		}
	case *iotago.NFTOutput:
		return []lookupKey{
			lookupKeyNFTOutputyByNFTID(output.NFTID, o.outputID),
		}
	case *iotago.FoundryOutput:
		foundryID, err := output.ID()
		if err != nil {
			panic(err)
		}
		return []lookupKey{
			lookupKeyFoundryOutputByFoundryID(foundryID, o.outputID),
		}
	default:
		panic("Unknown output type")
	}
}

func (o *Output) addressLookupKeys() []lookupKey {
	switch output := o.output.(type) {
	case *iotago.ExtendedOutput:
		return []lookupKey{
			lookupKeyByAddress(output.Address, output.FeatureBlocks().HasConstraints(), o.OutputType(), o.outputID),
		}
	case *iotago.AliasOutput:
		return []lookupKey{
			lookupKeyByAddress(output.StateController, output.FeatureBlocks().HasConstraints(), o.OutputType(), o.outputID),
			lookupKeyByAddress(output.GovernanceController, output.FeatureBlocks().HasConstraints(), o.OutputType(), o.outputID),
		}
	case *iotago.NFTOutput:
		return []lookupKey{
			lookupKeyByAddress(output.Address, output.FeatureBlocks().HasConstraints(), o.OutputType(), o.outputID),
		}
	case *iotago.FoundryOutput:
		return []lookupKey{
			lookupKeyByAddress(output.Address, output.FeatureBlocks().HasConstraints(), o.OutputType(), o.outputID),
		}
	default:
		panic("Unknown output type")
	}
}

func (o *Output) featureLookupKeys() []lookupKey {
	switch output := o.output.(type) {
	case *iotago.ExtendedOutput:
		return lookupKeysForFeatureBlocks(output.FeatureBlocks(), o.OutputType(), o.outputID)
	case *iotago.AliasOutput:
		return lookupKeysForFeatureBlocks(output.FeatureBlocks(), o.OutputType(), o.outputID)
	case *iotago.NFTOutput:
		return lookupKeysForFeatureBlocks(output.FeatureBlocks(), o.OutputType(), o.outputID)
	case *iotago.FoundryOutput:
		return nil
	default:
		panic("Unknown output type")
	}
}

func outputIDFromDatabaseKey(key lookupKey) (*iotago.OutputID, error) {

	ms := marshalutil.New([]byte(key))
	prefix, err := ms.ReadByte() // prefix
	if err != nil {
		return nil, err
	}

	switch prefix {
	case UTXOStoreKeyPrefixLookupExtendedOutputs:
		if _, err := parseAddress(ms); err != nil {
			return nil, err
		}
	case UTXOStoreKeyPrefixLookupNFTOutputs:
		ms.ReadSeek(ms.ReadOffset() + iotago.NFTIDLength)
	case UTXOStoreKeyPrefixLookupAliasOutputs:
		ms.ReadSeek(ms.ReadOffset() + iotago.AliasIDLength)
	case UTXOStoreKeyPrefixLookupFoundryOutputs:
		ms.ReadSeek(ms.ReadOffset() + iotago.FoundryIDLength)
	case UTXOStoreKeyPrefixLookupByAddress:
		if _, err := parseAddress(ms); err != nil {
			return nil, err
		}
		ms.ReadSeek(ms.ReadOffset() + 2) // skip over spending constraints and output type
	case UTXOStoreKeyPrefixLookupByIssuer:
		if _, err := parseAddress(ms); err != nil {
			return nil, err
		}
		ms.ReadSeek(ms.ReadOffset() + 1) // skip over output type
	case UTXOStoreKeyPrefixLookupBySender:
		if _, err := parseAddress(ms); err != nil {
			return nil, err
		}
		ms.ReadSeek(ms.ReadOffset() + 1) // skip over output type
	case UTXOStoreKeyPrefixLoolupBySenderAndIndex:
		if _, err := parseAddress(ms); err != nil {
			return nil, err
		}
		ms.ReadSeek(ms.ReadOffset() + 65) // skip over index and output type
	default:
		panic("unhandled prefix")
	}

	return ParseOutputID(ms)
}

func markAsUnspent(output *Output, mutations kvstore.BatchedMutations) error {
	for _, key := range output.lookupKeys() {
		if err := mutations.Set(key, []byte{}); err != nil {
			return err
		}
	}
	return nil
}

func markAsSpent(output *Output, mutations kvstore.BatchedMutations) error {
	return deleteOutputLookups(output, mutations)
}

func deleteOutputLookups(output *Output, mutations kvstore.BatchedMutations) error {
	for _, key := range output.lookupKeys() {
		if err := mutations.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func (u *Manager) IsOutputUnspentWithoutLocking(output *Output) (bool, error) {
	// Looking up the first key should be enough, since that is the main key
	return u.utxoStorage.Has(output.lookupKeys()[0])
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
