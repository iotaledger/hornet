package utxo

import (
	"bytes"
	"encoding/binary"
	"sync"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go"
)

var (
	utxoStorage kvstore.KVStore
	utxoLock    sync.RWMutex
)

func ConfigureStorages(store kvstore.KVStore) {
	configureOutputsStorage(store)
}

func ReadLockLedger() {
	utxoLock.RLock()
}

func ReadUnlockLedger() {
	utxoLock.RUnlock()
}

func WriteLockLedger() {
	utxoLock.Lock()
}

func WriteUnlockLedger() {
	utxoLock.Unlock()
}

func configureOutputsStorage(store kvstore.KVStore) {
	utxoStorage = store.WithRealm([]byte{StorePrefixUTXO})
}

func ReadOutputForTransactionWithoutLocking(transactionID *iotago.SignedTransactionPayloadHash, outputIndex uint16) (*Output, error) {
	ReadLockLedger()
	defer ReadUnlockLedger()

	bytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(bytes, outputIndex)
	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, transactionID[:], bytes)
	value, err := utxoStorage.Get(key)
	if err != nil {
		return nil, err
	}

	output := &Output{}
	if err := output.kvStorableLoad(key[1:], value); err != nil {
		return nil, err
	}
	return output, nil
}

func IsOutputUnspent(transactionID *iotago.SignedTransactionPayloadHash, outputIndex uint16) (bool, error) {
	ReadLockLedger()
	defer ReadUnlockLedger()

	output, err := ReadOutputForTransactionWithoutLocking(transactionID, outputIndex)
	if err != nil {
		return false, err
	}

	return output.IsUnspentWithoutLocking()
}

func UnspentOutputsForAddress(address *iotago.Ed25519Address) ([]*Output, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	var outputs []*Output

	addressKeyPrefix := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixUnspent}, address[:])

	err := utxoStorage.IterateKeys(addressKeyPrefix, func(key kvstore.Key) bool {

		outputKey := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, key[33:])

		value, err := utxoStorage.Get(outputKey)
		if err != nil {
			return false
		}

		output := &Output{}
		if err := output.kvStorableLoad(outputKey[1:], value); err != nil {
			return false
		}

		outputs = append(outputs, output)

		return true
	})

	return outputs, err
}

func spentOutputsForAddress(address *iotago.Ed25519Address) ([]*Spent, error) {

	var spents []*Spent

	addressKeyPrefix := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSpent}, address[:])

	err := utxoStorage.Iterate(addressKeyPrefix, func(key kvstore.Key, value kvstore.Value) bool {

		spent := &Spent{}
		if err := spent.kvStorableLoad(key[33:], value); err != nil {
			return false
		}

		outputKey := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, key[33:])

		outputValue, err := utxoStorage.Get(outputKey)
		if err != nil {
			return false
		}

		output := &Output{}
		if err := output.kvStorableLoad(outputKey[1:], outputValue); err != nil {
			return false
		}

		spent.Output = output

		spents = append(spents, spent)

		return true
	})

	return spents, err
}

func SpentOutputsForAddress(address *iotago.Ed25519Address) ([]*Spent, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return spentOutputsForAddress(address)
}

func storeOutput(output *Output, mutations kvstore.BatchedMutations) error {
	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, output.kvStorableKey())
	return mutations.Set(key, output.kvStorableValue())
}

func markAsUnspent(output *Output, mutations kvstore.BatchedMutations) error {
	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixUnspent}, output.UTXOKey())
	return mutations.Set(key, []byte{})
}

func storeSpentAndRemoveUnspent(spent *Spent, mutations kvstore.BatchedMutations) error {

	key := spent.kvStorableKey()
	unspentKey := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixUnspent}, key)
	spentKey := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSpent}, key)

	mutations.Delete(unspentKey)

	return mutations.Set(spentKey, spent.kvStorableValue())
}

func deleteSpent(spent *Spent, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSpent}, spent.kvStorableKey()))
}

func storeDiff(msIndex milestone.Index, newOutputs []*Output, newSpents []*Spent, mutations kvstore.BatchedMutations) error {

	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(msIndex))

	var value bytes.Buffer

	outputCount := make([]byte, 4)
	binary.LittleEndian.PutUint32(outputCount, uint32(len(newOutputs)))

	value.Write(outputCount)
	for _, output := range newOutputs {
		value.Write(output.kvStorableKey())
	}

	spentCount := make([]byte, 4)
	binary.LittleEndian.PutUint32(spentCount, uint32(len(newSpents)))

	value.Write(spentCount)
	for _, spent := range newSpents {
		value.Write(spent.kvStorableKey())
	}

	return mutations.Set(byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixMilestoneDiffs}, key), value.Bytes())
}

func getMilestoneDiffs(msIndex milestone.Index) ([]*Output, []*Spent, error) {

	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(msIndex))

	value, err := utxoStorage.Get(byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixMilestoneDiffs}, key))
	if err != nil {
		return nil, nil, err
	}

	marshalUtil := marshalutil.New(value)

	var outputs []*Output
	var spents []*Spent

	outputCount, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, nil, err
	}

	for i := 0; i < int(outputCount); i++ {
		transactionIDBytes, err := marshalUtil.ReadBytes(iotago.SignedTransactionPayloadHashLength)
		if err != nil {
			return nil, nil, err
		}

		outputIndex, err := marshalUtil.ReadUint16()
		if err != nil {
			return nil, nil, err
		}

		var transactionID iotago.SignedTransactionPayloadHash
		copy(transactionID[:], transactionIDBytes)

		outputs = append(outputs, &Output{TransactionID: transactionID, OutputIndex: outputIndex})
	}

	spentCount, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, nil, err
	}

	for i := 0; i < int(spentCount); i++ {
		addressBytes, err := marshalUtil.ReadBytes(iotago.Ed25519AddressBytesLength)
		if err != nil {
			return nil, nil, err
		}

		transactionIDBytes, err := marshalUtil.ReadBytes(iotago.SignedTransactionPayloadHashLength)
		if err != nil {
			return nil, nil, err
		}

		outputIndex, err := marshalUtil.ReadUint16()
		if err != nil {
			return nil, nil, err
		}

		var address iotago.Ed25519Address
		copy(address[:], addressBytes)

		var transactionID iotago.SignedTransactionPayloadHash
		copy(transactionID[:], transactionIDBytes)

		spents = append(spents, &Spent{Address: address, TransactionID: transactionID, OutputIndex: outputIndex})
	}

	return outputs, spents, nil
}

func deleteMilestoneDiffs(msIndex milestone.Index, mutations kvstore.BatchedMutations) error {

	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(msIndex))

	return mutations.Delete(byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixMilestoneDiffs}, key))
}

func PruneMilestoneIndex(msIndex milestone.Index) error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	_, spents, err := getMilestoneDiffs(msIndex)
	if err != nil {
		return err
	}

	mutation := utxoStorage.Batched()

	for _, spent := range spents {
		err = deleteOutput(&Output{TransactionID: spent.TransactionID, OutputIndex: spent.OutputIndex}, mutation)
		if err != nil {
			mutation.Cancel()
			return err
		}

		err = deleteSpent(spent, mutation)
		if err != nil {
			mutation.Cancel()
			return err
		}
	}

	err = deleteMilestoneDiffs(msIndex, mutation)
	if err != nil {
		mutation.Cancel()
		return err
	}

	return mutation.Commit()
}

func ApplyConfirmation(msIndex milestone.Index, newOutputs []*Output, newSpents []*Spent) error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	mutation := utxoStorage.Batched()

	for _, output := range newOutputs {
		if err := storeOutput(output, mutation); err != nil {
			mutation.Cancel()
			return err
		}
		if err := markAsUnspent(output, mutation); err != nil {
			mutation.Cancel()
			return err
		}
	}

	for _, spent := range newSpents {
		if err := storeSpentAndRemoveUnspent(spent, mutation); err != nil {
			mutation.Cancel()
			return err
		}
	}

	if err := storeDiff(msIndex, newOutputs, newSpents, mutation); err != nil {
		mutation.Cancel()
		return err
	}

	return mutation.Commit()
}
