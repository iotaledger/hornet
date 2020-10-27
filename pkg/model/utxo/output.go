package utxo

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/hornet"
)

type Output struct {
	kvStorable

	outputID   *iotago.UTXOInputID
	messageID  *hornet.MessageID
	outputType iotago.OutputType
	address    *iotago.Ed25519Address
	amount     uint64
}

func (o *Output) OutputID() *iotago.UTXOInputID {
	return o.outputID
}

func (o *Output) MessageID() *hornet.MessageID {
	return o.messageID
}

func (o *Output) OutputType() iotago.OutputType {
	return o.outputType
}

func (o *Output) Address() *iotago.Ed25519Address {
	return o.address
}

func (o *Output) Amount() uint64 {
	return o.amount
}

type Outputs []*Output

func (o Outputs) InputToOutputMapping() iotago.InputToOutputMapping {

	mapping := iotago.InputToOutputMapping{}
	for _, output := range o {
		mapping[*output.outputID] = &iotago.SigLockedSingleOutput{
			Address: output.address,
			Amount:  output.amount,
		}
	}
	return mapping
}

func GetOutput(outputID *iotago.UTXOInputID, messageID *hornet.MessageID, address *iotago.Ed25519Address, amount uint64) *Output {
	return &Output{
		outputID:   outputID,
		messageID:  messageID,
		outputType: iotago.OutputSigLockedSingleOutput,
		address:    address,
		amount:     amount,
	}
}

func NewOutput(messageID *hornet.MessageID, transaction *iotago.Transaction, index uint16) (*Output, error) {

	var deposit *iotago.SigLockedSingleOutput
	switch unsignedTx := transaction.Essence.(type) {
	case *iotago.TransactionEssence:
		if len(unsignedTx.Outputs) < int(index) {
			return nil, errors.New("deposit not found")
		}
		output := unsignedTx.Outputs[int(index)]
		switch d := output.(type) {
		case *iotago.SigLockedSingleOutput:
			deposit = d
		default:
			return nil, errors.New("unsuported output type")
		}
	default:
		return nil, errors.New("unsupported transaction type")
	}

	var address *iotago.Ed25519Address
	switch a := deposit.Address.(type) {
	case *iotago.Ed25519Address:
		address = a
	default:
		return nil, errors.New("unsupported deposit address")
	}

	txID, err := transaction.ID()
	if err != nil {
		return nil, err
	}

	bytes := make([]byte, iotago.UInt16ByteSize)
	binary.LittleEndian.PutUint16(bytes, index)

	var outputID iotago.UTXOInputID
	copy(outputID[:iotago.TransactionIDLength], txID[:])
	copy(outputID[iotago.TransactionIDLength:iotago.TransactionIDLength+iotago.UInt16ByteSize], bytes)

	return &Output{
		outputID:   &outputID,
		messageID:  messageID,
		outputType: iotago.OutputSigLockedSingleOutput,
		address:    address,
		amount:     deposit.Amount,
	}, nil
}

func (o *Output) UTXOKey() (key []byte) {
	return byteutils.ConcatBytes(o.address[:], o.outputID[:])
}

func (o *Output) kvStorableKey() (key []byte) {
	return o.outputID[:]
}

func (o *Output) kvStorableValue() (value []byte) {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, o.amount)
	return byteutils.ConcatBytes(o.messageID.Slice(), []byte{o.outputType}, o.address[:], bytes)
}

func (o *Output) kvStorableLoad(key []byte, value []byte) error {

	expectedKeyLength := iotago.TransactionIDLength + iotago.UInt16ByteSize

	if len(key) < expectedKeyLength {
		return fmt.Errorf("not enough bytes in key to unmarshal object, expected: %d, got: %d", expectedKeyLength, len(key))
	}

	expectedValueLength := iotago.MessageIDLength + iotago.OneByte + iotago.Ed25519AddressBytesLength + iotago.UInt64ByteSize

	if len(value) < expectedValueLength {
		return fmt.Errorf("not enough bytes in value to unmarshal object, expected: %d, got: %d", expectedValueLength, len(value))
	}

	o.outputID = &iotago.UTXOInputID{}
	copy(o.outputID[:], key[:iotago.TransactionIDLength+iotago.UInt16ByteSize])
	o.messageID = hornet.MessageIDFromBytes(value[:iotago.MessageIDLength])
	o.outputType = value[iotago.MessageIDLength]

	o.address = &iotago.Ed25519Address{}
	copy(o.address[:], value[iotago.MessageIDLength+iotago.OneByte:iotago.MessageIDLength+iotago.OneByte+iotago.Ed25519AddressBytesLength])
	o.amount = binary.LittleEndian.Uint64(value[iotago.MessageIDLength+iotago.OneByte+iotago.Ed25519AddressBytesLength : iotago.MessageIDLength+iotago.OneByte+iotago.Ed25519AddressBytesLength+iotago.UInt64ByteSize])

	return nil
}

func (o *Output) IsUnspentWithoutLocking() (bool, error) {
	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixUnspent}, o.UTXOKey())
	return utxoStorage.Has(key)
}

func storeOutput(output *Output, mutations kvstore.BatchedMutations) error {
	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, output.kvStorableKey())
	return mutations.Set(key, output.kvStorableValue())
}

func deleteOutput(output *Output, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, output.kvStorableKey()))
}

func ForEachOutputWithoutLocking(consumer OutputConsumer) error {

	var innerErr error

	if err := utxoStorage.Iterate([]byte{UTXOStoreKeyPrefixOutput}, func(key kvstore.Key, value kvstore.Value) bool {

		output := &Output{}
		if err := output.kvStorableLoad(key[1:], value); err != nil {
			innerErr = err
			return false
		}

		return consumer(output)
	}); err != nil {
		return err
	}

	return innerErr
}

func ForEachOutput(consumer OutputConsumer) error {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return ForEachOutputWithoutLocking(consumer)
}

func ReadOutputByOutputIDWithoutLocking(outputID *iotago.UTXOInputID) (*Output, error) {

	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, outputID[:])
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

func ReadOutputByOutputID(outputID *iotago.UTXOInputID) (*Output, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return ReadOutputByOutputIDWithoutLocking(outputID)
}
