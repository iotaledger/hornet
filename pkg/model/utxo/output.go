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

	OutputID iotago.UTXOInputID

	MessageID hornet.Hash

	Type    iotago.OutputType
	Address iotago.Ed25519Address
	Amount  uint64
}

type Outputs []*Output

func NewOutput(messageID hornet.Hash, transaction *iotago.SignedTransactionPayload, index uint16) (*Output, error) {

	var deposit *iotago.SigLockedSingleDeposit
	switch unsignedTx := transaction.Transaction.(type) {
	case *iotago.UnsignedTransaction:
		if len(unsignedTx.Outputs) < int(index) {
			return nil, errors.New("deposit not found")
		}
		output := unsignedTx.Outputs[int(index)]
		switch d := output.(type) {
		case *iotago.SigLockedSingleDeposit:
			deposit = d
		default:
			return nil, errors.New("unsuported output type")
		}
	default:
		return nil, errors.New("unsupported transaction type")
	}

	txID, err := transaction.Hash()
	if err != nil {
		return nil, err
	}

	var address iotago.Ed25519Address
	switch a := deposit.Address.(type) {
	case *iotago.Ed25519Address:
		address = *a
	default:
		return nil, errors.New("unsupported deposit address")
	}

	var outputID iotago.UTXOInputID
	copy(outputID[:iotago.TransactionIDLength], txID[:])
	bytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(bytes, index)
	copy(outputID[iotago.TransactionIDLength:iotago.TransactionIDLength+2], bytes)

	return &Output{
		OutputID:  outputID,
		MessageID: messageID,
		Type:      iotago.OutputSigLockedSingleDeposit,
		Address:   address,
		Amount:    deposit.Amount,
	}, nil
}

func (o *Output) UTXOKey() (key []byte) {
	return byteutils.ConcatBytes(o.Address[:], o.OutputID[:])
}

func (o *Output) kvStorableKey() (key []byte) {
	return o.OutputID[:]
}

func (o *Output) kvStorableValue() (value []byte) {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, o.Amount)
	return byteutils.ConcatBytes(o.MessageID, []byte{o.Type}, o.Address[:], bytes)
}

func (o *Output) kvStorableLoad(key []byte, value []byte) error {

	expectedKeyLength := iotago.SignedTransactionPayloadHashLength + 2

	if len(key) < expectedKeyLength {
		return fmt.Errorf("not enough bytes in key to unmarshal object, expected: %d, got: %d", expectedKeyLength, len(key))
	}

	expectedValueLength := iotago.MessageHashLength + 1 + iotago.Ed25519AddressBytesLength + 8

	if len(value) < expectedValueLength {
		return fmt.Errorf("not enough bytes in value to unmarshal object, expected: %d, got: %d", expectedValueLength, len(value))
	}

	copy(o.OutputID[:], key[:iotago.TransactionIDLength+2])

	copy(o.MessageID, value[:iotago.MessageHashLength])
	o.Type = value[iotago.MessageHashLength]
	copy(o.Address[:], value[iotago.MessageHashLength+1:iotago.MessageHashLength+iotago.Ed25519AddressBytesLength])
	o.Amount = binary.LittleEndian.Uint64(value[iotago.MessageHashLength+1+iotago.Ed25519AddressBytesLength : iotago.MessageHashLength+iotago.Ed25519AddressBytesLength+8])

	return nil
}

func (o *Output) IsUnspentWithoutLocking() (bool, error) {
	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixUnspent}, o.kvStorableKey())
	return utxoStorage.Has(key)
}

func storeOutput(output *Output, mutations kvstore.BatchedMutations) error {
	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, output.kvStorableKey())
	return mutations.Set(key, output.kvStorableValue())
}

func deleteOutput(output *Output, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, output.kvStorableKey()))
}

func ReadOutputForTransactionWithoutLocking(utxoInputId iotago.UTXOInputID) (*Output, error) {

	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, utxoInputId[:])
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
