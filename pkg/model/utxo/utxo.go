package utxo

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/kvstore"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	// ErrOutputsSumNotEqualTotalSupply is returned if the sum of the output deposits is not equal the total supply of tokens.
	ErrOutputsSumNotEqualTotalSupply = errors.New("accumulated output balance is not equal to total supply")
)

type Manager struct {
	utxoStorage kvstore.KVStore
	utxoLock    sync.RWMutex
}

func New(store kvstore.KVStore) *Manager {
	return &Manager{
		utxoStorage: store,
	}
}

// KVStore returns the underlying KVStore.
func (u *Manager) KVStore() kvstore.KVStore {
	return u.utxoStorage
}

// ClearLedger removes all entries from the UTXO ledger (spent, unspent, diff, receipts, treasury).
func (u *Manager) ClearLedger(pruneReceipts bool) (err error) {
	u.WriteLockLedger()
	defer u.WriteUnlockLedger()

	defer func() {
		if errFlush := u.utxoStorage.Flush(); err == nil && errFlush != nil {
			err = errFlush
		}
	}()

	if pruneReceipts {
		// if we also prune the receipts, we can just clear everything
		return u.utxoStorage.Clear()
	}

	if err = u.utxoStorage.DeletePrefix([]byte{UTXOStoreKeyPrefixLedgerMilestoneIndex}); err != nil {
		return err
	}
	if err = u.utxoStorage.DeletePrefix([]byte{UTXOStoreKeyPrefixOutput}); err != nil {
		return err
	}
	if err = u.utxoStorage.DeletePrefix([]byte{UTXOStoreKeyPrefixOutputSpent}); err != nil {
		return err
	}
	if err = u.utxoStorage.DeletePrefix([]byte{UTXOStoreKeyPrefixOutputUnspent}); err != nil {
		return err
	}

	if err = u.utxoStorage.DeletePrefix([]byte{UTXOStoreKeyPrefixMilestoneDiffs}); err != nil {
		return err
	}
	if err = u.utxoStorage.DeletePrefix([]byte{UTXOStoreKeyPrefixTreasuryOutput}); err != nil {
		return err
	}

	return nil
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

func (u *Manager) PruneMilestoneIndexWithoutLocking(msIndex iotago.MilestoneIndex, pruneReceipts bool, receiptMigratedAtIndex ...iotago.MilestoneIndex) error {

	diff, err := u.MilestoneDiffWithoutLocking(msIndex)
	if err != nil {
		return err
	}

	mutations, err := u.utxoStorage.Batched()
	if err != nil {
		return err
	}

	for _, spent := range diff.Spents {
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

	if len(receiptMigratedAtIndex) > 0 {
		if pruneReceipts {
			placeHolder := &ReceiptTuple{Receipt: &iotago.ReceiptMilestoneOpt{MigratedAt: receiptMigratedAtIndex[0]}, MilestoneIndex: msIndex}
			if err := deleteReceipt(placeHolder, mutations); err != nil {
				mutations.Cancel()

				return err
			}
		}

		// only ever delete spent treasury outputs, since the unspent treasury output must exist
		// even after a milestone's lifetime
		if err := deleteTreasuryOutput(diff.SpentTreasuryOutput, mutations); err != nil {
			return err
		}
	}

	return mutations.Commit()
}

func storeLedgerIndex(msIndex iotago.MilestoneIndex, mutations kvstore.BatchedMutations) error {

	value := make([]byte, 4)
	binary.LittleEndian.PutUint32(value, msIndex)

	return mutations.Set([]byte{UTXOStoreKeyPrefixLedgerMilestoneIndex}, value)
}

func (u *Manager) StoreLedgerIndex(msIndex iotago.MilestoneIndex) error {
	u.WriteLockLedger()
	defer u.WriteUnlockLedger()

	value := make([]byte, 4)
	binary.LittleEndian.PutUint32(value, msIndex)

	return u.utxoStorage.Set([]byte{UTXOStoreKeyPrefixLedgerMilestoneIndex}, value)
}

func (u *Manager) ReadLedgerIndexWithoutLocking() (iotago.MilestoneIndex, error) {
	value, err := u.utxoStorage.Get([]byte{UTXOStoreKeyPrefixLedgerMilestoneIndex})
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			// there is no ledger milestone yet => return 0
			return 0, nil
		}

		return 0, fmt.Errorf("failed to load ledger milestone index: %w", err)
	}

	return binary.LittleEndian.Uint32(value), nil
}

func (u *Manager) ReadLedgerIndex() (iotago.MilestoneIndex, error) {
	u.ReadLockLedger()
	defer u.ReadUnlockLedger()

	return u.ReadLedgerIndexWithoutLocking()
}

// TreasuryMutationTuple holds data about a mutation happening to the treasury.
type TreasuryMutationTuple struct {
	// The treasury transaction causes this mutation.
	NewOutput *TreasuryOutput
	// The previous treasury output which funded the new transaction.
	SpentOutput *TreasuryOutput
}

func (u *Manager) ApplyConfirmationWithoutLocking(msIndex iotago.MilestoneIndex, newOutputs Outputs, newSpents Spents, tm *TreasuryMutationTuple, rt *ReceiptTuple) error {

	mutations, err := u.utxoStorage.Batched()
	if err != nil {
		return err
	}

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
		if err := storeSpentAndMarkOutputAsSpent(spent, mutations); err != nil {
			mutations.Cancel()

			return err
		}
	}

	msDiff := &MilestoneDiff{
		Index:   msIndex,
		Outputs: newOutputs,
		Spents:  newSpents,
	}

	if rt != nil {
		if err := storeReceipt(rt, mutations); err != nil {
			mutations.Cancel()

			return err
		}
	}

	if tm != nil {
		if err := storeTreasuryOutput(tm.NewOutput, mutations); err != nil {
			mutations.Cancel()

			return err
		}

		msDiff.TreasuryOutput = tm.NewOutput

		// this simply re-keys the output
		if err := markTreasuryOutputAsSpent(tm.SpentOutput, mutations); err != nil {
			mutations.Cancel()

			return err
		}
		msDiff.SpentTreasuryOutput = tm.SpentOutput
	}

	if err := storeDiff(msDiff, mutations); err != nil {
		mutations.Cancel()

		return err
	}

	if err := storeLedgerIndex(msIndex, mutations); err != nil {
		mutations.Cancel()

		return err
	}

	return mutations.Commit()
}

func (u *Manager) ApplyConfirmation(msIndex iotago.MilestoneIndex, newOutputs Outputs, newSpents Spents, tm *TreasuryMutationTuple, rt *ReceiptTuple) error {
	u.WriteLockLedger()
	defer u.WriteUnlockLedger()

	return u.ApplyConfirmationWithoutLocking(msIndex, newOutputs, newSpents, tm, rt)
}

func (u *Manager) RollbackConfirmationWithoutLocking(msIndex iotago.MilestoneIndex, newOutputs Outputs, newSpents Spents, tm *TreasuryMutationTuple, rt *ReceiptTuple) error {

	mutations, err := u.utxoStorage.Batched()
	if err != nil {
		return err
	}

	// we have to store the spents as output and mark them as unspent
	for _, spent := range newSpents {
		if err := storeOutput(spent.output, mutations); err != nil {
			mutations.Cancel()

			return err
		}

		if err := deleteSpentAndMarkOutputAsUnspent(spent, mutations); err != nil {
			mutations.Cancel()

			return err
		}
	}

	// we have to delete the newOutputs of this milestone
	for _, output := range newOutputs {
		if err := deleteOutput(output, mutations); err != nil {
			mutations.Cancel()

			return err
		}
		if err := deleteOutputLookups(output, mutations); err != nil {
			mutations.Cancel()

			return err
		}
	}

	if rt != nil {
		if err := deleteReceipt(rt, mutations); err != nil {
			mutations.Cancel()

			return err
		}
	}

	if tm != nil {
		if err := deleteTreasuryOutput(tm.NewOutput, mutations); err != nil {
			mutations.Cancel()

			return err
		}

		if err := markTreasuryOutputAsUnspent(tm.SpentOutput, mutations); err != nil {
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

func (u *Manager) RollbackConfirmation(msIndex iotago.MilestoneIndex, newOutputs Outputs, newSpents Spents, tm *TreasuryMutationTuple, rt *ReceiptTuple) error {
	u.WriteLockLedger()
	defer u.WriteUnlockLedger()

	return u.RollbackConfirmationWithoutLocking(msIndex, newOutputs, newSpents, tm, rt)
}

func (u *Manager) CheckLedgerState(tokenSupply uint64) error {

	total, _, err := u.ComputeLedgerBalance()
	if err != nil {
		return err
	}

	treasuryOutput, err := u.UnspentTreasuryOutputWithoutLocking()
	if err != nil {
		return fmt.Errorf("unable to get unspent treasury output: %w", err)
	}
	total += treasuryOutput.Amount

	if total != tokenSupply {
		return ErrOutputsSumNotEqualTotalSupply
	}

	return nil
}

func (u *Manager) AddUnspentOutput(unspentOutput *Output) error {

	u.WriteLockLedger()
	defer u.WriteUnlockLedger()

	mutations, err := u.utxoStorage.Batched()
	if err != nil {
		return err
	}

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

func (u *Manager) LedgerStateSHA256Sum() ([]byte, error) {
	u.ReadLockLedger()
	defer u.ReadUnlockLedger()

	ledgerStateHash := sha256.New()

	ledgerIndex, err := u.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, err
	}
	if err := binary.Write(ledgerStateHash, binary.LittleEndian, ledgerIndex); err != nil {
		return nil, err
	}

	// get all UTXOs and sort them by outputID
	outputIDs, err := u.UnspentOutputsIDs(ReadLockLedger(false))
	if err != nil {
		return nil, err
	}

	for _, outputID := range outputIDs.RemoveDupsAndSort() {
		output, err := u.ReadOutputByOutputID(outputID)
		if err != nil {
			return nil, err
		}

		if _, err := ledgerStateHash.Write(output.outputID[:]); err != nil {
			return nil, err
		}

		if _, err := ledgerStateHash.Write(output.KVStorableValue()); err != nil {
			return nil, err
		}
	}

	// calculate sha256 hash
	return ledgerStateHash.Sum(nil), nil
}
