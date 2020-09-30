package utxo

import (
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

	_, spents, err := getMilestoneDiffs(msIndex)
	if err != nil {
		return err
	}

	mutation := utxoStorage.Batched()

	for _, spent := range spents {
		err = deleteOutput(&Output{OutputID: spent.OutputID}, mutation)
		if err != nil {
			mutation.Cancel()
			return err
		}

		err = deleteSpent(spent, mutation)
		if err != nil {
			mutation.Cancel()
			return err
		}
	}

	err = deleteMilestoneDiffs(msIndex, mutation)
	if err != nil {
		mutation.Cancel()
		return err
	}

	return mutation.Commit()
}

func ApplyConfirmation(msIndex milestone.Index, newOutputs Outputs, newSpents Spents) error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	mutation := utxoStorage.Batched()

	for _, output := range newOutputs {
		if err := storeOutput(output, mutation); err != nil {
			mutation.Cancel()
			return err
		}
		if err := markAsUnspent(output, mutation); err != nil {
			mutation.Cancel()
			return err
		}
	}

	for _, spent := range newSpents {
		if err := storeSpentAndRemoveUnspent(spent, mutation); err != nil {
			mutation.Cancel()
			return err
		}
	}

	if err := storeDiff(msIndex, newOutputs, newSpents, mutation); err != nil {
		mutation.Cancel()
		return err
	}

	return mutation.Commit()
}

func CheckLedgerState() error {

	var total uint64 = 0

	consumerFunc := func(output *Output) bool {
		total += output.Amount
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

