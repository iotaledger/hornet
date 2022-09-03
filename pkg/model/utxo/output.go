package utxo

import (
	"bytes"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

// LexicalOrderedOutputs are outputs ordered in lexical order by their outputID.
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
	kvStorable

	outputID          iotago.OutputID
	blockID           iotago.BlockID
	msIndexBooked     iotago.MilestoneIndex
	msTimestampBooked uint32

	outputData []byte
	outputOnce sync.Once
	output     iotago.Output
}

func (o *Output) OutputID() iotago.OutputID {
	return o.outputID
}

func (o *Output) MapKey() string {
	return string(o.outputID[:])
}

func (o *Output) BlockID() iotago.BlockID {
	return o.blockID
}

func (o *Output) MilestoneIndexBooked() iotago.MilestoneIndex {
	return o.msIndexBooked
}

func (o *Output) MilestoneTimestampBooked() uint32 {
	return o.msTimestampBooked
}

func (o *Output) OutputType() iotago.OutputType {
	return o.Output().Type()
}

func (o *Output) Output() iotago.Output {
	o.outputOnce.Do(func() {
		if o.output == nil {
			var err error
			outputType := o.outputData[0]
			o.output, err = iotago.OutputSelector(uint32(outputType))
			if err != nil {
				panic(err)
			}
			_, err = o.output.Deserialize(o.outputData, serializer.DeSeriModeNoValidation, nil)
			if err != nil {
				panic(err)
			}
		}
	})

	return o.output
}

func (o *Output) Bytes() []byte {
	return o.outputData
}

func (o *Output) Deposit() uint64 {
	return o.Output().Deposit()
}

type Outputs []*Output

func (o Outputs) ToOutputSet() iotago.OutputSet {
	outputSet := make(iotago.OutputSet)
	for _, output := range o {
		outputSet[output.outputID] = output.Output()
	}

	return outputSet
}

func CreateOutput(outputID iotago.OutputID, blockID iotago.BlockID, msIndexBooked iotago.MilestoneIndex, msTimestampBooked uint32, output iotago.Output) *Output {

	outputBytes, err := output.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		panic(err)
	}

	o := &Output{
		outputID:          outputID,
		blockID:           blockID,
		msIndexBooked:     msIndexBooked,
		msTimestampBooked: msTimestampBooked,
		outputData:        outputBytes,
	}

	o.outputOnce.Do(func() {
		o.output = output
	})

	return o
}

func NewOutput(blockID iotago.BlockID, msIndexBooked iotago.MilestoneIndex, msTimestampBooked uint32, transaction *iotago.Transaction, index uint16) (*Output, error) {

	txID, err := transaction.ID()
	if err != nil {
		return nil, err
	}

	var output iotago.Output
	if len(transaction.Essence.Outputs) <= int(index) {
		return nil, errors.New("output not found")
	}
	output = transaction.Essence.Outputs[int(index)]
	outputID := iotago.OutputIDFromTransactionIDAndIndex(txID, index)

	return CreateOutput(outputID, blockID, msIndexBooked, msTimestampBooked, output), nil
}

//- kvStorable

func outputStorageKeyForOutputID(outputID iotago.OutputID) []byte {
	ms := marshalutil.New(35)
	ms.WriteByte(UTXOStoreKeyPrefixOutput) // 1 byte
	ms.WriteBytes(outputID[:])             // 34 bytes

	return ms.Bytes()
}

func (o *Output) KVStorableKey() (key []byte) {
	return outputStorageKeyForOutputID(o.outputID)
}

func (o *Output) KVStorableValue() (value []byte) {
	ms := marshalutil.New(40)
	ms.WriteBytes(o.blockID[:])         // 32 bytes
	ms.WriteUint32(o.msIndexBooked)     // 4 bytes
	ms.WriteUint32(o.msTimestampBooked) // 4 bytes
	ms.WriteBytes(o.outputData)

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

	// Read BlockID
	if o.blockID, err = ParseBlockID(valueUtil); err != nil {
		return err
	}

	// Read Milestone
	if o.msIndexBooked, err = valueUtil.ReadUint32(); err != nil {
		return err
	}

	if o.msTimestampBooked, err = valueUtil.ReadUint32(); err != nil {
		return err
	}

	o.outputData = valueUtil.ReadRemainingBytes()

	return nil
}

//- Helper

func storeOutput(output *Output, mutations kvstore.BatchedMutations) error {
	return mutations.Set(output.KVStorableKey(), output.KVStorableValue())
}

func deleteOutput(output *Output, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(output.KVStorableKey())
}

//- Manager

func (u *Manager) ReadOutputByOutputIDWithoutLocking(outputID iotago.OutputID) (*Output, error) {
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

func (u *Manager) ReadRawOutputBytesByOutputIDWithoutLocking(outputID iotago.OutputID) ([]byte, error) {
	key := outputStorageKeyForOutputID(outputID)
	value, err := u.utxoStorage.Get(key)
	if err != nil {
		return nil, err
	}

	// blockID + milestoneIndex + milestoneTimestamp
	offset := iotago.BlockIDLength + serializer.UInt32ByteSize + serializer.UInt32ByteSize
	if len(value) <= offset {
		return nil, errors.New("invalid UTXO output length")
	}

	return value[offset:], nil
}

func (u *Manager) ReadOutputByOutputID(outputID iotago.OutputID) (*Output, error) {

	u.ReadLockLedger()
	defer u.ReadUnlockLedger()

	return u.ReadOutputByOutputIDWithoutLocking(outputID)
}
