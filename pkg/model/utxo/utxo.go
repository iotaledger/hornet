package utxo

import (
	"encoding/binary"
	"errors"
	"sync"

	"github.com/iotaledger/hive.go/kvstore"

	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go"
)

var (
	utxoStorage kvstore.KVStore
	utxoLock    sync.RWMutex

	// Returned if the size of the given address is incorrect.
	ErrInvalidAddressSize = errors.New("invalid address size")

	// Returned if the sum of the output deposits is not equal the total supply of tokens.
	ErrOutputsSumNotEqualTotalSupply = errors.New("accumulated output balance is not equal to total supply")
)

func ConfigureStorages(store kvstore.KVStore) {
	utxoStorage = store.WithRealm([]byte{StorePrefixUTXO})
}

func ReadLockLedger() {
	utxoLock.RLock()
}

func ReadUnlockLedger() {
	utxoLock.RUnlock()
}

func WriteLockLedger() {
	utxoLock.Lock()
}

func WriteUnlockLedger() {
	utxoLock.Unlock()
}

func PruneMilestoneIndex(msIndex milestone.Index) error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	_, spents, err := GetMilestoneDiffsWithoutLocking(msIndex)
	if err != nil {
		return err
	}

	mutations := utxoStorage.Batched()

	for _, spent := range spents {
		err = deleteOutput(spent.output, mutations)
		if err != nil {
			mutations.Cancel()
			return err
		}

		err = deleteSpent(spent, mutations)
		if err != nil {
			mutations.Cancel()
			return err
		}
	}

	err = deleteDiff(msIndex, mutations)
	if err != nil {
		mutations.Cancel()
		return err
	}

	return mutations.Commit()
}

func storeLedgerIndex(msIndex milestone.Index, mutations kvstore.BatchedMutations) error {

	value := make([]byte, 4)
	binary.LittleEndian.PutUint32(value, uint32(msIndex))

	return mutations.Set([]byte{UTXOStoreKeyPrefixLedgerMilestoneIndex}, value)
}

func StoreLedgerIndex(msIndex milestone.Index) error {
	WriteLockLedger()
	defer WriteUnlockLedger()

	value := make([]byte, 4)
	binary.LittleEndian.PutUint32(value, uint32(msIndex))

	return utxoStorage.Set([]byte{UTXOStoreKeyPrefixLedgerMilestoneIndex}, value)
}

func ReadLedgerIndexWithoutLocking() (milestone.Index, error) {
	value, err := utxoStorage.Get([]byte{UTXOStoreKeyPrefixLedgerMilestoneIndex})
	if err != nil {
		return 0, err
	}

	return milestone.Index(binary.LittleEndian.Uint32(value)), nil
}

func ReadLedgerIndex() (milestone.Index, error) {
	ReadLockLedger()
	defer ReadUnlockLedger()

	return ReadLedgerIndexWithoutLocking()
}

func ApplyConfirmationWithoutLocking(msIndex milestone.Index, newOutputs Outputs, newSpents Spents) error {

	mutations := utxoStorage.Batched()

	for _, output := range newOutputs {
		if err := storeOutput(output, mutations); err != nil {
			mutations.Cancel()
			return err
		}
		if err := markAsUnspent(output, mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}

	for _, spent := range newSpents {
		if err := storeSpentAndRemoveUnspent(spent, mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}

	if err := storeDiff(msIndex, newOutputs, newSpents, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	if err := storeLedgerIndex(msIndex, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	return mutations.Commit()
}

func ApplyConfirmation(msIndex milestone.Index, newOutputs Outputs, newSpents Spents) error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	return ApplyConfirmationWithoutLocking(msIndex, newOutputs, newSpents)
}

func RollbackConfirmationWithoutLocking(msIndex milestone.Index, newOutputs Outputs, newSpents Spents) error {

	mutations := utxoStorage.Batched()

	// we have to delete the newOutputs of this milestone
	for _, output := range newOutputs {
		if err := deleteOutput(output, mutations); err != nil {
			mutations.Cancel()
			return err
		}
		if err := deleteFromUnspent(output, mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}

	// we have to store the spents as output and mark them as unspent
	for _, spent := range newSpents {
		if err := storeOutput(spent.output, mutations); err != nil {
			mutations.Cancel()
			return err
		}

		if err := deleteSpentAndMarkUnspent(spent, mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}

	if err := deleteDiff(msIndex, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	if err := storeLedgerIndex(msIndex-1, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	return mutations.Commit()
}

func RollbackConfirmation(msIndex milestone.Index, newOutputs Outputs, newSpents Spents) error {
	WriteLockLedger()
	defer WriteUnlockLedger()

	return RollbackConfirmationWithoutLocking(msIndex, newOutputs, newSpents)
}

func CheckLedgerState() error {

	var total uint64 = 0

	consumerFunc := func(output *Output) bool {
		total += output.amount
		return true
	}

	if err := ForEachUnspentOutput(consumerFunc); err != nil {
		return err
	}

	if total != iotago.TokenSupply {
		return ErrOutputsSumNotEqualTotalSupply
	}

	return nil
}

func AddUnspentOutput(unspentOutput *Output) error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	mutations := utxoStorage.Batched()

	if err := storeOutput(unspentOutput, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	if err := markAsUnspent(unspentOutput, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	return mutations.Commit()
}
