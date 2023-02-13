package utxo

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer"
	"github.com/iotaledger/hornet/pkg/model/hornet"
	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	OutputIDLength = iotago.TransactionIDLength + serializer.UInt16ByteSize
)

// LexicalOrderedOutputs are outputs
// ordered in lexical order by their outputID.
type LexicalOrderedOutputs []*Output

func (l LexicalOrderedOutputs) Len() int {
	return len(l)
}

func (l LexicalOrderedOutputs) Less(i, j int) bool {
	return bytes.Compare(l[i].outputID[:], l[j].outputID[:]) < 0
}

func (l LexicalOrderedOutputs) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

type Output struct {
	//nolint:unused
	kvStorable

	outputID   *iotago.UTXOInputID
	messageID  hornet.MessageID
	outputType iotago.OutputType
	address    iotago.Address
	amount     uint64
}

func (o *Output) OutputID() *iotago.UTXOInputID {
	return o.outputID
}

func (o *Output) MessageID() hornet.MessageID {
	return o.messageID
}

func (o *Output) OutputType() iotago.OutputType {
	return o.outputType
}

func (o *Output) Address() iotago.Address {
	return o.address
}

func (o *Output) Amount() uint64 {
	return o.amount
}

func (o *Output) AddressBytes() []byte {
	// This never throws an error for current Ed25519 addresses
	bytes, _ := o.address.Serialize(serializer.DeSeriModeNoValidation)
	return bytes
}

func (o *Output) UTXOInput() *iotago.UTXOInput {
	input := &iotago.UTXOInput{}
	copy(input.TransactionID[:], o.outputID[:iotago.TransactionIDLength])
	input.TransactionOutputIndex = binary.LittleEndian.Uint16(o.outputID[iotago.TransactionIDLength : iotago.TransactionIDLength+serializer.UInt16ByteSize])
	return input
}

type Outputs []*Output

func (o Outputs) InputToOutputMapping() (iotago.InputToOutputMapping, error) {

	mapping := iotago.InputToOutputMapping{}
	for _, output := range o {

		switch output.OutputType() {
		case iotago.OutputSigLockedDustAllowanceOutput:
			mapping[*output.outputID] = &iotago.SigLockedDustAllowanceOutput{
				Address: output.address,
				Amount:  output.amount,
			}

		case iotago.OutputSigLockedSingleOutput:
			mapping[*output.outputID] = &iotago.SigLockedSingleOutput{
				Address: output.address,
				Amount:  output.amount,
			}

		default:
			return nil, fmt.Errorf("unsupported output type")
		}

	}
	return mapping, nil
}

func CreateOutput(outputID *iotago.UTXOInputID, messageID hornet.MessageID, outputType iotago.OutputType, address iotago.Address, amount uint64) *Output {
	return &Output{
		outputID:   outputID,
		messageID:  messageID,
		outputType: outputType,
		address:    address,
		amount:     amount,
	}
}

func NewOutput(messageID hornet.MessageID, transaction *iotago.Transaction, index uint16) (*Output, error) {

	var output iotago.Output
	switch unsignedTx := transaction.Essence.(type) {
	case *iotago.TransactionEssence:
		if len(unsignedTx.Outputs) < int(index) {
			return nil, errors.New("deposit not found")
		}
		txOutput := unsignedTx.Outputs[int(index)]
		switch out := txOutput.(type) {
		case *iotago.SigLockedSingleOutput:
			output = out
		case *iotago.SigLockedDustAllowanceOutput:
			output = out
		default:
			return nil, errors.New("unsupported output type")
		}
	default:
		return nil, errors.New("unsupported transaction type")
	}

	var address *iotago.Ed25519Address
	outputAddress, err := output.Target()
	if err != nil {
		return nil, err
	}
	switch a := outputAddress.(type) {
	case *iotago.Ed25519Address:
		address = a
	default:
		return nil, errors.New("unsupported deposit address")
	}

	txID, err := transaction.ID()
	if err != nil {
		return nil, err
	}

	bytes := make([]byte, serializer.UInt16ByteSize)
	binary.LittleEndian.PutUint16(bytes, index)

	var outputID iotago.UTXOInputID
	copy(outputID[:iotago.TransactionIDLength], txID[:])
	copy(outputID[iotago.TransactionIDLength:iotago.TransactionIDLength+serializer.UInt16ByteSize], bytes)

	amount, err := output.Deposit()
	if err != nil {
		return nil, err
	}

	return CreateOutput(&outputID, messageID, output.Type(), address, amount), nil
}

//- kvStorable

func (o *Output) kvStorableKey() (key []byte) {
	ms := marshalutil.New(35)
	ms.WriteByte(UTXOStoreKeyPrefixOutput) // 1 byte
	ms.WriteBytes(o.outputID[:])           // 34 bytes
	return ms.Bytes()
}

func (o *Output) kvStorableValue() (value []byte) {
	ms := marshalutil.New(74)
	ms.WriteBytes(o.messageID)      // 32 bytes
	ms.WriteByte(o.outputType)      // 1 byte
	ms.WriteBytes(o.AddressBytes()) // 33 bytes
	ms.WriteUint64(o.amount)        // 8 bytes
	return ms.Bytes()
}

func (o *Output) kvStorableLoad(_ *Manager, key []byte, value []byte) error {

	// Parse key
	keyUtil := marshalutil.New(key)

	// Read prefix output
	_, err := keyUtil.ReadByte()
	if err != nil {
		return err
	}

	// Read OutputID
	if o.outputID, err = ParseOutputID(keyUtil); err != nil {
		return err
	}

	// Parse value
	valueUtil := marshalutil.New(value)

	// Read MessageID
	if o.messageID, err = ParseMessageID(valueUtil); err != nil {
		return err
	}

	// Read OutputType
	o.outputType, err = valueUtil.ReadByte()
	if err != nil {
		return err
	}

	// Read Address
	if o.address, err = parseAddress(valueUtil); err != nil {
		return err
	}

	// Read Amount
	o.amount, err = valueUtil.ReadUint64()
	if err != nil {
		return err
	}

	return nil
}

//- Helper

func storeOutput(output *Output, mutations kvstore.BatchedMutations) error {
	return mutations.Set(output.kvStorableKey(), output.kvStorableValue())
}

func deleteOutput(output *Output, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(output.kvStorableKey())
}

//- Manager

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

func (u *Manager) ReadOutputByOutputIDWithoutLocking(outputID *iotago.UTXOInputID) (*Output, error) {

	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, outputID[:])
	value, err := u.utxoStorage.Get(key)
	if err != nil {
		return nil, err
	}

	output := &Output{}
	if err := output.kvStorableLoad(u, key, value); err != nil {
		return nil, err
	}
	return output, nil
}

func (u *Manager) ReadOutputByOutputID(outputID *iotago.UTXOInputID) (*Output, error) {

	u.ReadLockLedger()
	defer u.ReadUnlockLedger()

	return u.ReadOutputByOutputIDWithoutLocking(outputID)
}
