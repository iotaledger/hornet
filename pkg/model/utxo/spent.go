package utxo

import (
	"encoding/binary"
	"fmt"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

// Spent are already spent TXOs (transaction outputs) per address
type Spent struct {
	kvStorable

	Address  iotago.Ed25519Address
	OutputID iotago.UTXOInputID

	Output *Output

	TargetTransactionID iotago.SignedTransactionPayloadHash
	ConfirmationIndex   milestone.Index
}

type Spents []*Spent

func NewSpent(output *Output, targetTransactionID iotago.SignedTransactionPayloadHash, confirmationIndex milestone.Index) *Spent {
	return &Spent{
		Address:             output.Address,
		OutputID:            output.OutputID,
		Output:              output,
		TargetTransactionID: targetTransactionID,
		ConfirmationIndex:   confirmationIndex,
	}
}

func (s *Spent) kvStorableKey() (key []byte) {
	return byteutils.ConcatBytes(s.Address[:], s.OutputID[:])
}

func (s *Spent) kvStorableValue() (value []byte) {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(s.ConfirmationIndex))
	return byteutils.ConcatBytes(s.TargetTransactionID[:], bytes)
}

// UnmarshalBinary parses the binary encoded representation of the spent utxo.
func (s *Spent) kvStorableLoad(key []byte, value []byte) error {

	expectedKeyLength := iotago.Ed25519AddressBytesLength + iotago.SignedTransactionPayloadHashLength + 2

	if len(key) < expectedKeyLength {
		return fmt.Errorf("not enough bytes in key to unmarshal object, expected: %d, got: %d", expectedKeyLength, len(key))
	}

	expectedValueLength := iotago.SignedTransactionPayloadHashLength + 4

	if len(value) < expectedValueLength {
		return fmt.Errorf("not enough bytes in value to unmarshal object, expected: %d, got: %d", expectedValueLength, len(value))
	}

	copy(s.Address[:], key[:iotago.Ed25519AddressBytesLength])
	copy(s.OutputID[:], key[iotago.Ed25519AddressBytesLength:iotago.Ed25519AddressBytesLength+iotago.TransactionIDLength+2])

	/*
	   32 bytes            TargetTransactionID
	   4 bytes uint32        ConfirmationIndex
	*/

	copy(s.TargetTransactionID[:], value[:iotago.SignedTransactionPayloadHashLength])
	s.ConfirmationIndex = milestone.Index(binary.LittleEndian.Uint32(value[iotago.SignedTransactionPayloadHashLength : iotago.SignedTransactionPayloadHashLength+4]))

	return nil
}

func spentOutputsForAddress(address *iotago.Ed25519Address) (Spents, error) {

	var spents Spents

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

func SpentOutputsForAddress(address *iotago.Ed25519Address) (Spents, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return spentOutputsForAddress(address)
}

func storeSpentAndRemoveUnspent(spent *Spent, mutations kvstore.BatchedMutations) error {

	key := spent.kvStorableKey()
	unspentKey := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixUnspent}, key)
	spentKey := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSpent}, key)

	mutations.Delete(unspentKey)

	return mutations.Set(spentKey, spent.kvStorableValue())
}

func deleteSpentAndMarkUnspent(spent *Spent, mutations kvstore.BatchedMutations) error {
	if err := deleteSpent(spent, mutations); err != nil {
		return err
	}

	return markAsUnspent(spent.Output, mutations)
}

func deleteSpent(spent *Spent, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSpent}, spent.kvStorableKey()))
}
