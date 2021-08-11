package utxo

import (
	"encoding/binary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	iotago "github.com/iotaledger/iota.go/v2"
)

// ReceiptTuple contains a receipt and the index of the milestone
// which contained the receipt.
type ReceiptTuple struct {
	// The actual receipt.
	Receipt *iotago.Receipt `json:"receipt"`
	// The index of the milestone which included the receipt.
	MilestoneIndex milestone.Index `json:"milestoneIndex"`
}

func (rt *ReceiptTuple) kvStorableKey() (key []byte) {
	return marshalutil.New(9).
		WriteByte(UTXOStoreKeyPrefixReceipts).
		WriteUint32(rt.Receipt.MigratedAt).
		WriteUint32(uint32(rt.MilestoneIndex)).
		Bytes()
}

func (rt *ReceiptTuple) kvStorableValue() (value []byte) {
	receiptBytes, err := rt.Receipt.Serialize(iotago.DeSeriModeNoValidation)
	if err != nil {
		panic(err)
	}
	return receiptBytes
}

func (rt *ReceiptTuple) kvStorableLoad(_ *Manager, key []byte, value []byte) error {
	keyExt := marshalutil.New(key)

	// skip prefix and migrated at index
	if _, err := keyExt.ReadByte(); err != nil {
		return err
	}

	if _, err := keyExt.ReadUint32(); err != nil {
		return err
	}

	// read out index of the milestone which contained this receipt
	msIndex, err := keyExt.ReadUint32()
	if err != nil {
		return err
	}

	r := &iotago.Receipt{}
	if _, err := r.Deserialize(value, iotago.DeSeriModeNoValidation); err != nil {
		return err
	}

	rt.Receipt = r
	rt.MilestoneIndex = milestone.Index(msIndex)

	return nil
}

// adds a receipt store instruction to the given mutations.
func storeReceipt(rt *ReceiptTuple, mutations kvstore.BatchedMutations) error {
	return mutations.Set(rt.kvStorableKey(), rt.kvStorableValue())
}

// adds a receipt delete instruction to the given mutations.
func deleteReceipt(rt *ReceiptTuple, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(rt.kvStorableKey())
}

// SearchHighestReceiptMigratedAtIndex searches the highest migratedAt of all stored receipts.
func (u *Manager) SearchHighestReceiptMigratedAtIndex(options ...UTXOIterateOption) (uint32, error) {
	var highestMigratedAtIndex uint32
	if err := u.ForEachReceiptTuple(func(rt *ReceiptTuple) bool {
		if rt.Receipt.MigratedAt > highestMigratedAtIndex {
			highestMigratedAtIndex = rt.Receipt.MigratedAt
		}
		return true
	}, options...); err != nil {
		return 0, err
	}
	return highestMigratedAtIndex, nil
}

// ReceiptTupleConsumer is a function that consumes a receipt tuple.
type ReceiptTupleConsumer func(rt *ReceiptTuple) bool

// ForEachReceiptTuple iterates over all stored receipt tuples.
func (u *Manager) ForEachReceiptTuple(consumer ReceiptTupleConsumer, options ...UTXOIterateOption) error {
	opt := iterateOptions(options)

	if opt.readLockLedger {
		u.ReadLockLedger()
		defer u.ReadUnlockLedger()
	}

	var innerErr error
	var i int
	if err := u.utxoStorage.Iterate([]byte{UTXOStoreKeyPrefixReceipts}, func(key kvstore.Key, value kvstore.Value) bool {

		if (opt.maxResultCount > 0) && (i >= opt.maxResultCount) {
			return false
		}

		i++

		rt := &ReceiptTuple{}
		if err := rt.kvStorableLoad(u, key, value); err != nil {
			innerErr = err
			return false
		}

		return consumer(rt)
	}); err != nil {
		return err
	}

	return innerErr
}

// ForEachReceiptTupleMigratedAt iterates over all stored receipt tuples for a given migrated at index.
func (u *Manager) ForEachReceiptTupleMigratedAt(migratedAtIndex milestone.Index, consumer ReceiptTupleConsumer, options ...UTXOIterateOption) error {
	opt := iterateOptions(options)

	if opt.readLockLedger {
		u.ReadLockLedger()
		defer u.ReadUnlockLedger()
	}

	prefix := make([]byte, 5)
	prefix[0] = UTXOStoreKeyPrefixReceipts
	binary.LittleEndian.PutUint32(prefix[1:], uint32(migratedAtIndex))

	var innerErr error
	var i int
	if err := u.utxoStorage.Iterate(prefix, func(key kvstore.Key, value kvstore.Value) bool {

		if (opt.maxResultCount > 0) && (i >= opt.maxResultCount) {
			return false
		}

		i++

		rt := &ReceiptTuple{}
		if err := rt.kvStorableLoad(u, key, value); err != nil {
			innerErr = err
			return false
		}

		return consumer(rt)
	}); err != nil {
		return err
	}

	return innerErr
}

// ReceiptToOutputs extracts the migrated funds to outputs.
func ReceiptToOutputs(r *iotago.Receipt, msgID hornet.MessageID, msID *iotago.MilestoneID) ([]*Output, error) {
	outputs := make([]*Output, len(r.Funds))
	for outputIndex, migFundsEntry := range r.Funds {
		entry := migFundsEntry.(*iotago.MigratedFundsEntry)
		utxoID := OutputIDForMigratedFunds(*msID, uint16(outputIndex))
		// we use the milestone hash as the "origin message"
		outputs[outputIndex] = CreateOutput(&utxoID, msgID, iotago.OutputSigLockedSingleOutput, entry.Address.(iotago.Address), entry.Deposit)
	}
	return outputs, nil
}

// OutputIDForMigratedFunds returns the UTXO ID for a migrated funds entry given the milestone containing the receipt
// and the index of the entry.
func OutputIDForMigratedFunds(milestoneHash iotago.MilestoneID, outputIndex uint16) iotago.UTXOInputID {
	var utxoID iotago.UTXOInputID
	copy(utxoID[:], milestoneHash[:])
	binary.LittleEndian.PutUint16(utxoID[len(utxoID)-2:], outputIndex)
	return utxoID
}

// ReceiptToTreasuryMutation converts a receipt to a treasury mutation tuple.
func ReceiptToTreasuryMutation(r *iotago.Receipt, unspentTreasuryOutput *TreasuryOutput, newMsID *iotago.MilestoneID) (*TreasuryMutationTuple, error) {
	newOutput := &TreasuryOutput{
		Amount: r.Transaction.(*iotago.TreasuryTransaction).Output.(*iotago.TreasuryOutput).Amount,
		Spent:  false,
	}
	copy(newOutput.MilestoneID[:], newMsID[:])

	return &TreasuryMutationTuple{
		NewOutput:   newOutput,
		SpentOutput: unspentTreasuryOutput,
	}, nil
}
