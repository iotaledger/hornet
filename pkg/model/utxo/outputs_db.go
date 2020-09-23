package utxo

import (
	"encoding/binary"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go"
)

var outputsStorage kvstore.KVStore

func configureOutputsStorage(store kvstore.KVStore) {
	outputsStorage = store.WithRealm([]byte{StorePrefixOutputs})
}

func ReadOutputForTransaction(transactionID iotago.SignedTransactionPayloadHash, index uint16) (*Output, error) {
	bytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(bytes, index)
	key := byteutils.ConcatBytes(transactionID[:], bytes)
	value, err := outputsStorage.Get(key)
	if err != nil {
		return nil, err
	}

	var output *Output
	output.FromStorage(key, value)
	return output, nil
}
