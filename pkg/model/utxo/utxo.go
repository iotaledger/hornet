package utxo

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go"
)

var (
	// Returned if the size of the given address is incorrect.
	ErrInvalidAddressSize = errors.New("invalid address size")

	// Returned if the sum of the output deposits is not equal the total supply of tokens.
	ErrOutputsSumNotEqualTotalSupply = errors.New("accumulated output balance is not equal to total supply")
)

type Manager struct {
	utxoStorage kvstore.KVStore
	utxoLock    sync.RWMutex
}

func New(store kvstore.KVStore) *Manager {
	return &Manager{
		utxoStorage: store.WithRealm([]byte{common.StorePrefixUTXO}),
	}
}

func (u *Manager) ReadLockLedger() {
	u.utxoLock.RLock()
}

func (u *Manager) ReadUnlockLedger() {
	u.utxoLock.RUnlock()
}

func (u *Manager) WriteLockLedger() {
	u.utxoLock.Lock()
}

func (u *Manager) WriteUnlockLedger() {
	u.utxoLock.Unlock()
}

func (u *Manager) PruneMilestoneIndex(msIndex milestone.Index) error {

	u.WriteLockLedger()
	defer u.WriteUnlockLedger()

	_, spents, err := u.GetMilestoneDiffsWithoutLocking(msIndex)
	if err != nil {
		return err
	}

	mutations := u.utxoStorage.Batched()

	for _, spent := range spents {
		if err := deleteOutput(spent.output, mutations); err != nil {
			mutations.Cancel()
			return err
		}

		if err := deleteSpent(spent, mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}

	if err := deleteDiff(msIndex, mutations); err != nil {
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

func (u *Manager) StoreLedgerIndex(msIndex milestone.Index) error {
	u.WriteLockLedger()
	defer u.WriteUnlockLedger()

	value := make([]byte, 4)
	binary.LittleEndian.PutUint32(value, uint32(msIndex))

	return u.utxoStorage.Set([]byte{UTXOStoreKeyPrefixLedgerMilestoneIndex}, value)
}

func (u *Manager) ReadLedgerIndexWithoutLocking() (milestone.Index, error) {
	value, err := u.utxoStorage.Get([]byte{UTXOStoreKeyPrefixLedgerMilestoneIndex})
	if err != nil {
		if err == kvstore.ErrKeyNotFound {
			// there is no ledger milestone yet => return 0
			return 0, nil
		}
		return 0, fmt.Errorf("failed to load ledger milestone index: %w", err)
	}

	return milestone.Index(binary.LittleEndian.Uint32(value)), nil
}

func (u *Manager) ReadLedgerIndex() (milestone.Index, error) {
	u.ReadLockLedger()
	defer u.ReadUnlockLedger()

	return u.ReadLedgerIndexWithoutLocking()
}

func (u *Manager) ApplyConfirmationWithoutLocking(msIndex milestone.Index, newOutputs Outputs, newSpents Spents) error {

	mutations := u.utxoStorage.Batched()

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

func (u *Manager) ApplyConfirmation(msIndex milestone.Index, newOutputs Outputs, newSpents Spents) error {

	u.WriteLockLedger()
	defer u.WriteUnlockLedger()

	return u.ApplyConfirmationWithoutLocking(msIndex, newOutputs, newSpents)
}

func (u *Manager) RollbackConfirmationWithoutLocking(msIndex milestone.Index, newOutputs Outputs, newSpents Spents) error {

	mutations := u.utxoStorage.Batched()

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

func (u *Manager) RollbackConfirmation(msIndex milestone.Index, newOutputs Outputs, newSpents Spents) error {
	u.WriteLockLedger()
	defer u.WriteUnlockLedger()

	return u.RollbackConfirmationWithoutLocking(msIndex, newOutputs, newSpents)
}

func (u *Manager) CheckLedgerState() error {

	var total uint64 = 0

	consumerFunc := func(output *Output) bool {
		total += output.amount
		return true
	}

	if err := u.ForEachUnspentOutput(consumerFunc); err != nil {
		return err
	}

	if total != iotago.TokenSupply {
		return ErrOutputsSumNotEqualTotalSupply
	}

	return nil
}

func (u *Manager) AddUnspentOutput(unspentOutput *Output) error {

	u.WriteLockLedger()
	defer u.WriteUnlockLedger()

	mutations := u.utxoStorage.Batched()

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
