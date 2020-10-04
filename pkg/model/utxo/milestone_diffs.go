package utxo

import (
	"bytes"
	"encoding/binary"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	iotago "github.com/iotaledger/iota.go"
)

func storeDiff(msIndex milestone.Index, newOutputs Outputs, newSpents Spents, mutations kvstore.BatchedMutations) error {

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

func deleteDiff(msIndex milestone.Index, mutations kvstore.BatchedMutations) error {

	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(msIndex))

	return mutations.Delete(byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixMilestoneDiffs}, key))
}

func GetMilestoneDiffsWithoutLocking(msIndex milestone.Index) (Outputs, Spents, error) {

	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(msIndex))

	value, err := utxoStorage.Get(byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixMilestoneDiffs}, key))
	if err != nil {
		return nil, nil, err
	}

	marshalUtil := marshalutil.New(value)

	var outputs Outputs
	var spents Spents

	outputCount, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, nil, err
	}

	for i := 0; i < int(outputCount); i++ {
		outputIDBytes, err := marshalUtil.ReadBytes(iotago.TransactionIDLength + 2)
		if err != nil {
			return nil, nil, err
		}

		var outputID iotago.UTXOInputID
		copy(outputID[:], outputIDBytes)

		output, err := ReadOutputForTransactionWithoutLocking(outputID)
		if err != nil {
			return nil, nil, err
		}

		outputs = append(outputs, output)
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

		outputIDBytes, err := marshalUtil.ReadBytes(iotago.TransactionIDLength + 2)
		if err != nil {
			return nil, nil, err
		}

		var address iotago.Ed25519Address
		copy(address[:], addressBytes)

		var outputID iotago.UTXOInputID
		copy(outputID[:], outputIDBytes)

		spent, err := ReadSpentForAddressAndTransactionWithoutLocking(&address, &outputID)
		if err != nil {
			return nil, nil, err
		}

		spents = append(spents, spent)
	}

	return outputs, spents, nil
}

func GetMilestoneDiffs(msIndex milestone.Index) (Outputs, Spents, error) {
	ReadLockLedger()
	defer ReadUnlockLedger()

	return GetMilestoneDiffsWithoutLocking(msIndex)
}
