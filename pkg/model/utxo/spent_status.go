package utxo

import (
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	iotago "github.com/iotaledger/iota.go/v3"
)

type OutputConsumer func(output *Output) bool

type lookupKey []byte

func lookupKeyUnspentOutput(outputID *iotago.OutputID) lookupKey {
	ms := marshalutil.New(35)
	ms.WriteByte(UTXOStoreKeyPrefixOutputUnspent) // 1 byte
	ms.WriteBytes(outputID[:])                    // 34 bytes
	return ms.Bytes()
}

func (o *Output) unspentLookupKey() lookupKey {
	return lookupKeyUnspentOutput(o.outputID)
}

func outputIDFromDatabaseKey(key lookupKey) (*iotago.OutputID, error) {
	ms := marshalutil.New([]byte(key))
	_, err := ms.ReadByte() // prefix
	if err != nil {
		return nil, err
	}

	return ParseOutputID(ms)
}

func markAsUnspent(output *Output, mutations kvstore.BatchedMutations) error {
	return mutations.Set(output.unspentLookupKey(), []byte{})
}

func markAsSpent(output *Output, mutations kvstore.BatchedMutations) error {
	return deleteOutputLookups(output, mutations)
}

func deleteOutputLookups(output *Output, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(output.unspentLookupKey())
}

func (u *Manager) IsOutputUnspentWithoutLocking(output *Output) (bool, error) {
	return u.utxoStorage.Has(output.unspentLookupKey())
}

func (u *Manager) IsOutputUnspent(outputID *iotago.OutputID) (bool, error) {
	u.ReadLockLedger()
	defer u.ReadUnlockLedger()

	output, err := u.ReadOutputByOutputIDWithoutLocking(outputID)
	if err != nil {
		return false, err
	}

	return u.IsOutputUnspentWithoutLocking(output)
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
