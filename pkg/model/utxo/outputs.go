package utxo

import (
	"encoding/binary"
	"errors"

	"github.com/iotaledger/hive.go/byteutils"
	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/hornet"
)

type Output struct {
	TransactionID *iotago.SignedTransactionPayloadHash
	Index         uint16

	MessageID hornet.Hash
	Address   *iotago.Ed25519Address

	Amount uint64
}

func NewOutput(messageID hornet.Hash, transaction iotago.SignedTransactionPayload, index uint16) (*Output, error) {

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
		Index:         index,
		MessageID:     messageID,
		Address:       address,
		Amount:        deposit.Amount,
	}, nil
}

func (o *Output) StorageKey() (key []byte) {
	bytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(bytes, o.Index)
	return byteutils.ConcatBytes(o.TransactionID[:], bytes)
}

func (o *Output) StorageValue() (value []byte) {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, o.Amount)
	return byteutils.ConcatBytes(o.MessageID, o.Address[:], bytes)
}

func (o *Output) FromStorage(key []byte, value []byte) {

	copy(o.TransactionID[:], key[:iotago.SignedTransactionPayloadHashLength])
	o.Index = binary.LittleEndian.Uint16(key[int(iotago.SignedTransactionPayloadHashLength):2])

	copy(o.MessageID, value[:iotago.MessageHashLength])
	copy(o.Address[:], value[iotago.MessageHashLength:iotago.MessageHashLength+iotago.Ed25519AddressBytesLength])
	o.Amount = binary.LittleEndian.Uint64(value[iotago.MessageHashLength+iotago.Ed25519AddressBytesLength : iotago.MessageHashLength+iotago.Ed25519AddressBytesLength+8])
}
