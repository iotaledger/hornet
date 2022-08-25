package utxo

import (
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/marshalutil"
	iotago "github.com/iotaledger/iota.go/v3"
)

// OutputIDConsumer is a function that consumes an output ID.
// Returning false from this function indicates to abort the iteration.
type OutputIDConsumer func(outputID iotago.OutputID) bool

// OutputConsumer is a function that consumes an output.
// Returning false from this function indicates to abort the iteration.
type OutputConsumer func(output *Output) bool

type LookupKey []byte

func lookupKeyUnspentOutput(outputID iotago.OutputID) LookupKey {
	ms := marshalutil.New(35)
	ms.WriteByte(UTXOStoreKeyPrefixOutputUnspent) // 1 byte
	ms.WriteBytes(outputID[:])                    // 34 bytes

	return ms.Bytes()
}

func (o *Output) UnspentLookupKey() LookupKey {
	return lookupKeyUnspentOutput(o.outputID)
}

func outputIDFromDatabaseKey(key LookupKey) (iotago.OutputID, error) {
	ms := marshalutil.New([]byte(key))

	// prefix
	if _, err := ms.ReadByte(); err != nil {
		return iotago.OutputID{}, err
	}

	return ParseOutputID(ms)
}

func markAsUnspent(output *Output, mutations kvstore.BatchedMutations) error {
	return mutations.Set(output.UnspentLookupKey(), []byte{})
}

func markAsSpent(output *Output, mutations kvstore.BatchedMutations) error {
	return deleteOutputLookups(output, mutations)
}

func deleteOutputLookups(output *Output, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(output.UnspentLookupKey())
}

func (u *Manager) IsOutputIDUnspentWithoutLocking(outputID iotago.OutputID) (bool, error) {
	return u.utxoStorage.Has(lookupKeyUnspentOutput(outputID))
}

func (u *Manager) IsOutputUnspentWithoutLocking(output *Output) (bool, error) {
	return u.utxoStorage.Has(output.UnspentLookupKey())
}

func storeSpentAndMarkOutputAsSpent(spent *Spent, mutations kvstore.BatchedMutations) error {
	if err := storeSpent(spent, mutations); err != nil {
		return err
	}

	return markAsSpent(spent.output, mutations)
}

func deleteSpentAndMarkOutputAsUnspent(spent *Spent, mutations kvstore.BatchedMutations) error {
	if err := deleteSpent(spent, mutations); err != nil {
		return err
	}

	return markAsUnspent(spent.output, mutations)
}
