package utxo

import (
	"encoding/binary"
	"fmt"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

type SpentConsumer func(spent *Spent) bool

// Spent are already spent TXOs (transaction outputs) per address
type Spent struct {
	kvStorable

	output *Output

	targetTransactionID *iotago.SignedTransactionPayloadHash
	confirmationIndex   milestone.Index
}

func (s *Spent) Output() *Output {
	return s.output
}

func (s *Spent) OutputID() *iotago.UTXOInputID {
	return s.output.outputID
}

func (s *Spent) MessageID() *hornet.MessageID {
	return s.output.messageID
}

func (s *Spent) OutputType() iotago.OutputType {
	return s.output.outputType
}

func (s *Spent) Address() *iotago.Ed25519Address {
	return s.output.address
}

func (s *Spent) Amount() uint64 {
	return s.output.amount
}

func (s *Spent) TargetTransactionID() *iotago.SignedTransactionPayloadHash {
	return s.targetTransactionID
}

func (s *Spent) ConfirmationIndex() milestone.Index {
	return s.confirmationIndex
}

type Spents []*Spent

func NewSpent(output *Output, targetTransactionID *iotago.SignedTransactionPayloadHash, confirmationIndex milestone.Index) *Spent {
	return &Spent{
		output:              output,
		targetTransactionID: targetTransactionID,
		confirmationIndex:   confirmationIndex,
	}
}

func (s *Spent) kvStorableKey() (key []byte) {
	return byteutils.ConcatBytes(s.output.address[:], s.output.outputID[:])
}

func (s *Spent) kvStorableValue() (value []byte) {
	bytes := make([]byte, iotago.UInt32ByteSize)
	binary.LittleEndian.PutUint32(bytes, uint32(s.confirmationIndex))
	return byteutils.ConcatBytes(s.targetTransactionID[:], bytes)
}

// UnmarshalBinary parses the binary encoded representation of the spent utxo.
func (s *Spent) kvStorableLoad(key []byte, value []byte) error {

	expectedKeyLength := iotago.Ed25519AddressBytesLength + iotago.TransactionIDLength + iotago.UInt16ByteSize

	if len(key) < expectedKeyLength {
		return fmt.Errorf("not enough bytes in key to unmarshal object, expected: %d, got: %d", expectedKeyLength, len(key))
	}

	expectedValueLength := iotago.TransactionIDLength + iotago.UInt32ByteSize

	if len(value) < expectedValueLength {
		return fmt.Errorf("not enough bytes in value to unmarshal object, expected: %d, got: %d", expectedValueLength, len(value))
	}

	outputID := key[iotago.Ed25519AddressBytesLength : iotago.Ed25519AddressBytesLength+iotago.TransactionIDLength+iotago.UInt16ByteSize]
	outputKey := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, outputID)

	outputValue, err := utxoStorage.Get(outputKey)
	if err != nil {
		return err
	}

	output := &Output{}
	if err := output.kvStorableLoad(outputID, outputValue); err != nil {
		return err
	}

	s.output = output

	/*
	   32 bytes				TargetTransactionID
	   4 bytes uint32		ReferencedIndex
	*/

	copy(s.targetTransactionID[:], value[:iotago.TransactionIDLength])
	s.confirmationIndex = milestone.Index(binary.LittleEndian.Uint32(value[iotago.TransactionIDLength : iotago.TransactionIDLength+iotago.UInt32ByteSize]))

	return nil
}

func forEachSpentOutputsForAddress(consumer SpentConsumer, address *iotago.Ed25519Address) error {

	addressKeyPrefix := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSpent}, address[:])

	var innerErr error

	if err := utxoStorage.Iterate(addressKeyPrefix, func(key kvstore.Key, value kvstore.Value) bool {

		spent := &Spent{}
		if err := spent.kvStorableLoad(key[1:], value); err != nil {
			innerErr = err
			return false
		}

		return consumer(spent)
	}); err != nil {
		return err
	}

	return innerErr
}

func SpentOutputsForAddress(address *iotago.Ed25519Address, maxFind ...int) (Spents, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	var spents []*Spent

	i := 0
	consumerFunc := func(spent *Spent) bool {
		i++

		if (len(maxFind) > 0) && (i > maxFind[0]) {
			return false
		}

		spents = append(spents, spent)
		return true
	}

	if err := forEachSpentOutputsForAddress(consumerFunc, address); err != nil {
		return nil, err
	}

	return spents, nil
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

	return markAsUnspent(spent.output, mutations)
}

func deleteSpent(spent *Spent, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSpent}, spent.kvStorableKey()))
}

func ReadSpentForAddressAndTransactionWithoutLocking(address *iotago.Ed25519Address, outputID *iotago.UTXOInputID) (*Spent, error) {

	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSpent}, address[:], outputID[:])
	value, err := utxoStorage.Get(key)
	if err != nil {
		return nil, err
	}

	spent := &Spent{}
	if err := spent.kvStorableLoad(key[1:], value); err != nil {
		return nil, err
	}

	return spent, nil
}
