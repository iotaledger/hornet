package utxo

import (
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

type Output struct {
	kvStorable

	outputID          *iotago.OutputID
	messageID         hornet.MessageID
	confirmationIndex milestone.Index
	output            iotago.Output
}

func (o *Output) OutputID() *iotago.OutputID {
	return o.outputID
}

func (o *Output) mapKey() string {
	return string(o.outputID[:])
}

func (o *Output) MessageID() hornet.MessageID {
	return o.messageID
}

func (o *Output) MilestoneIndex() milestone.Index {
	return o.confirmationIndex
}

func (o *Output) OutputType() iotago.OutputType {
	return o.output.Type()
}

func (o *Output) Output() iotago.Output {
	return o.output
}

// TODO: rename to "Owner" or "Ident"
func (o *Output) Address() iotago.Address {
	switch output := o.output.(type) {
	case iotago.TransIndepIdentOutput:
		return output.Ident()
	case iotago.TransDepIdentOutput:
		return output.Chain().ToAddress()
	default:
		panic("unsupported output type")
	}
}

func (o *Output) AddressBytes() []byte {
	// This never throws an error for current Ed25519 addresses
	bytes, _ := o.Address().Serialize(serializer.DeSeriModeNoValidation, nil)
	return bytes
}

func (o *Output) Amount() uint64 {
	return o.output.Deposit()
}

type Outputs []*Output

func (o Outputs) InputToOutputMapping() (iotago.OutputSet, error) {
	outputSet := make(iotago.OutputSet)
	for _, output := range o {
		outputSet[*output.outputID] = output.output
	}
	return outputSet, nil
}

func CreateOutput(outputID *iotago.OutputID, messageID hornet.MessageID, milestoneIndex milestone.Index, output iotago.Output) *Output {
	return &Output{
		outputID:          outputID,
		messageID:         messageID,
		confirmationIndex: milestoneIndex,
		output:            output,
	}
}

func NewOutput(messageID hornet.MessageID, milestoneIndex milestone.Index, transaction *iotago.Transaction, index uint16) (*Output, error) {

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

	return CreateOutput(&outputID, messageID, milestoneIndex, output), nil
}

//- kvStorable

func outputStorageKeyForOutputID(outputID *iotago.OutputID) []byte {
	ms := marshalutil.New(35)
	ms.WriteByte(UTXOStoreKeyPrefixOutput) // 1 byte
	ms.WriteBytes(outputID[:])             // 34 bytes
	return ms.Bytes()
}

func (o *Output) kvStorableKey() (key []byte) {
	return outputStorageKeyForOutputID(o.outputID)
}

func (o *Output) kvStorableValue() (value []byte) {
	ms := marshalutil.New(36)
	ms.WriteBytes(o.messageID)                  // 32 bytes
	ms.WriteUint32(uint32(o.confirmationIndex)) // 4 bytes

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

	// Read Milestone
	if o.confirmationIndex, err = parseMilestoneIndex(valueUtil); err != nil {
		return err
	}

	outputType, err := valueUtil.ReadByte()
	if err != nil {
		return err
	}
	valueUtil.ReadSeek(-1)

	output, err := iotago.OutputSelector(uint32(outputType))
	_, err = output.Deserialize(valueUtil.ReadRemainingBytes(), serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return err
	}
	o.output = output

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

func (u *Manager) ReadOutputByOutputIDWithoutLocking(outputID *iotago.OutputID) (*Output, error) {
	key := outputStorageKeyForOutputID(outputID)
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
