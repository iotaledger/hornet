package utxo

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/iotaledger/hive.go/byteutils"
	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/hornet"
)

type Output struct {
	kvStorable

	TransactionID *iotago.SignedTransactionPayloadHash
	OutputIndex   uint16

	MessageID hornet.Hash
	Address   *iotago.Ed25519Address

	Amount uint64
}

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

	var address *iotago.Ed25519Address
	switch a := deposit.Address.(type) {
	case *iotago.Ed25519Address:
		address = a
	default:
		return nil, errors.New("unsupported deposit address")
	}

	return &Output{
		TransactionID: txID,
		OutputIndex:   index,
		MessageID:     messageID,
		Address:       address,
		Amount:        deposit.Amount,
	}, nil
}

func (o *Output) UTXOKey() (key []byte) {
	bytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(bytes, o.OutputIndex)
	return byteutils.ConcatBytes(o.Address[:], o.TransactionID[:], bytes)
}

func (o *Output) kvStorableKey() (key []byte) {
	bytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(bytes, o.OutputIndex)
	return byteutils.ConcatBytes(o.TransactionID[:], bytes)
}

func (o *Output) kvStorableValue() (value []byte) {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, o.Amount)
	return byteutils.ConcatBytes(o.MessageID, o.Address[:], bytes)
}

func (o *Output) kvStorableLoad(key []byte, value []byte) error {

	expectedKeyLength := iotago.SignedTransactionPayloadHashLength + 2

	if len(key) < expectedKeyLength {
		return fmt.Errorf("not enough bytes in key to unmarshal object, expected: %d, got: %d", expectedKeyLength, len(key))
	}

	expectedValueLength := iotago.MessageHashLength + iotago.Ed25519AddressBytesLength + 8

	if len(value) < expectedValueLength {
		return fmt.Errorf("not enough bytes in value to unmarshal object, expected: %d, got: %d", expectedValueLength, len(value))
	}

	copy(o.TransactionID[:], key[:iotago.SignedTransactionPayloadHashLength])
	o.OutputIndex = binary.LittleEndian.Uint16(key[int(iotago.SignedTransactionPayloadHashLength):2])

	copy(o.MessageID, value[:iotago.MessageHashLength])
	copy(o.Address[:], value[iotago.MessageHashLength:iotago.MessageHashLength+iotago.Ed25519AddressBytesLength])
	o.Amount = binary.LittleEndian.Uint64(value[iotago.MessageHashLength+iotago.Ed25519AddressBytesLength : iotago.MessageHashLength+iotago.Ed25519AddressBytesLength+8])

	return nil
}

func (o *Output) IsUnspentWithoutLocking() (bool, error) {
	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixUnspent}, o.kvStorableKey())
	return utxoStorage.Has(key)
}
