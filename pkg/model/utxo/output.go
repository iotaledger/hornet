package utxo

import (
	"encoding/binary"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	OutputIDLength = iotago.TransactionIDLength + serializer.UInt16ByteSize
)

type Output struct {
	kvStorable

	outputID  *iotago.OutputID
	messageID hornet.MessageID
	output    iotago.Output
}

func (o *Output) OutputID() *iotago.OutputID {
	return o.outputID
}

func (o *Output) MessageID() hornet.MessageID {
	return o.messageID
}

func (o *Output) OutputType() iotago.OutputType {
	return o.output.Type()
}

func (o *Output) Output() iotago.Output {
	return o.output
}

func (o *Output) Address() iotago.Address {
	switch output := o.output.(type) {
	case *iotago.SimpleOutput:
		return output.Address
	case *iotago.ExtendedOutput:
		return output.Address
	case *iotago.NFTOutput:
		return output.Address
	case *iotago.FoundryOutput:
		return output.Address
	case *iotago.AliasOutput:
		return output.AliasID.ToAddress()
	}
	panic("unsupported output type")
}

func (o *Output) Amount() uint64 {
	return o.output.Deposit()
}

func (o *Output) AddressBytes() []byte {
	// This never throws an error for current Ed25519 addresses
	bytes, _ := o.Address().Serialize(serializer.DeSeriModeNoValidation, nil)
	return bytes
}

func (o *Output) UTXOInput() *iotago.UTXOInput {
	input := &iotago.UTXOInput{}
	copy(input.TransactionID[:], o.outputID[:iotago.TransactionIDLength])
	input.TransactionOutputIndex = binary.LittleEndian.Uint16(o.outputID[iotago.TransactionIDLength : iotago.TransactionIDLength+serializer.UInt16ByteSize])
	return input
}

type Outputs []*Output

func (o Outputs) InputToOutputMapping() (iotago.OutputSet, error) {
	outputSet := make(iotago.OutputSet)
	for _, output := range o {
		outputSet[*output.outputID] = output.output
	}
	return outputSet, nil
}

func CreateOutput(outputID *iotago.OutputID, messageID hornet.MessageID, output iotago.Output) *Output {
	return &Output{
		outputID:  outputID,
		messageID: messageID,
		output:    output,
	}
}

func NewOutput(messageID hornet.MessageID, transaction *iotago.Transaction, index uint16) (*Output, error) {

	txID, err := transaction.ID()
	if err != nil {
		return nil, err
	}

	var output iotago.Output
	if len(transaction.Essence.Outputs) < int(index) {
		return nil, errors.New("output not found")
	}
	output = transaction.Essence.Outputs[int(index)]
	outputID := iotago.OutputIDFromTransactionIDAndIndex(*txID, index)

	return CreateOutput(&outputID, messageID, output), nil
}

//- kvStorable

func (o *Output) kvStorableKey() (key []byte) {
	ms := marshalutil.New(35)
	ms.WriteByte(UTXOStoreKeyPrefixOutput) // 1 byte
	ms.WriteBytes(o.outputID[:])           // 34 bytes
	return ms.Bytes()
}

func (o *Output) kvStorableValue() (value []byte) {
	ms := marshalutil.New(32)
	ms.WriteBytes(o.messageID) // 32 bytes

	bytes, err := o.output.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		panic(err)
	}
	ms.WriteBytes(bytes)

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

	outputType, err := valueUtil.ReadUint32()
	if err != nil {
		return err
	}
	valueUtil.ReadSeek(-4)

	output, err := iotago.OutputSelector(outputType)
	_, err = output.Deserialize(valueUtil.ReadRemainingBytes(), serializer.DeSeriModeNoValidation, nil)
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

func (u *Manager) ReadOutputByOutputIDWithoutLocking(outputID *iotago.OutputID) (*Output, error) {

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

func (u *Manager) ReadOutputByOutputID(outputID *iotago.OutputID) (*Output, error) {

	u.ReadLockLedger()
	defer u.ReadUnlockLedger()

	return u.ReadOutputByOutputIDWithoutLocking(outputID)
}
