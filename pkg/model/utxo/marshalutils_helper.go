package utxo

import (
	"github.com/iotaledger/hive.go/core/marshalutil"
	iotago "github.com/iotaledger/iota.go/v3"
)

func ParseOutputID(ms *marshalutil.MarshalUtil) (iotago.OutputID, error) {
	o := iotago.OutputID{}
	bytes, err := ms.ReadBytes(iotago.OutputIDLength)
	if err != nil {
		return o, err
	}
	copy(o[:], bytes)

	return o, nil
}

func parseTransactionID(ms *marshalutil.MarshalUtil) (iotago.TransactionID, error) {
	t := iotago.TransactionID{}
	bytes, err := ms.ReadBytes(iotago.TransactionIDLength)
	if err != nil {
		return t, err
	}
	copy(t[:], bytes)

	return t, nil
}

func ParseBlockID(ms *marshalutil.MarshalUtil) (iotago.BlockID, error) {
	bytes, err := ms.ReadBytes(iotago.BlockIDLength)
	if err != nil {
		return iotago.EmptyBlockID(), err
	}
	blockID := iotago.BlockID{}
	copy(blockID[:], bytes)

	return blockID, nil
}
