package utxo

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go/v2"
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

// ClearLedger removes all entries from the UTXO ledger (spent, unspent, diff, balances, receipts, treasury).
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
	if err = u.utxoStorage.DeletePrefix([]byte{UTXOStoreKeyPrefixUnspent}); err != nil {
		return err
	}
	if err = u.utxoStorage.DeletePrefix([]byte{UTXOStoreKeyPrefixSpent}); err != nil {
		return err
	}
	if err = u.utxoStorage.DeletePrefix([]byte{UTXOStoreKeyPrefixMilestoneDiffs}); err != nil {
		return err
	}
	if err = u.utxoStorage.DeletePrefix([]byte{UTXOStoreKeyPrefixBalances}); err != nil {
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

func (u *Manager) PruneMilestoneIndexWithoutLocking(msIndex milestone.Index, pruneReceipts bool, receiptMigratedAtIndex ...uint32) error {

	diff, err := u.MilestoneDiffWithoutLocking(msIndex)
	if err != nil {
		return err
	}

	mutations := u.utxoStorage.Batched()

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
			placeHolder := &ReceiptTuple{Receipt: &iotago.Receipt{MigratedAt: receiptMigratedAtIndex[0]}, MilestoneIndex: msIndex}
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
		if errors.Is(err, kvstore.ErrKeyNotFound) {
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

// TreasuryMutationTuple holds data about a mutation happening to the treasury.
type TreasuryMutationTuple struct {
	// The treasury transaction causes this mutation.
	NewOutput *TreasuryOutput
	// The previous treasury output which funded the new transaction.
	SpentOutput *TreasuryOutput
}

func (u *Manager) ApplyConfirmationWithoutLocking(msIndex milestone.Index, newOutputs Outputs, newSpents Spents, tm *TreasuryMutationTuple, rt *ReceiptTuple) error {

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

	if err := u.applyNewBalancesWithoutLocking(newOutputs, newSpents, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	return mutations.Commit()
}

func (u *Manager) ApplyConfirmation(msIndex milestone.Index, newOutputs Outputs, newSpents Spents, tm *TreasuryMutationTuple, rt *ReceiptTuple) error {
	u.WriteLockLedger()
	defer u.WriteUnlockLedger()

	return u.ApplyConfirmationWithoutLocking(msIndex, newOutputs, newSpents, tm, rt)
}

func (u *Manager) RollbackConfirmationWithoutLocking(msIndex milestone.Index, newOutputs Outputs, newSpents Spents, tm *TreasuryMutationTuple, rt *ReceiptTuple) error {

	mutations := u.utxoStorage.Batched()

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

	if err := u.rollbackBalancesWithoutLocking(newOutputs, newSpents, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	return mutations.Commit()
}

func (u *Manager) RollbackConfirmation(msIndex milestone.Index, newOutputs Outputs, newSpents Spents, tm *TreasuryMutationTuple, rt *ReceiptTuple) error {
	u.WriteLockLedger()
	defer u.WriteUnlockLedger()

	return u.RollbackConfirmationWithoutLocking(msIndex, newOutputs, newSpents, tm, rt)
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

	treasuryOutput, err := u.UnspentTreasuryOutputWithoutLocking()
	if err != nil {
		return err
	}
	total += treasuryOutput.Amount

	if total != iotago.TokenSupply {
		return ErrOutputsSumNotEqualTotalSupply
	}

	return u.checkBalancesLedger(treasuryOutput.Amount)
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

	if err := u.storeBalanceForUnspentOutput(unspentOutput, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	return mutations.Commit()
}

type UTXOIterateOptions struct {
	address          iotago.Address
	readLockLedger   bool
	maxResultCount   int
	filterOutputType *iotago.OutputType
}

type UTXOIterateOption func(*UTXOIterateOptions)

func FilterAddress(address iotago.Address) UTXOIterateOption {
	return func(args *UTXOIterateOptions) {
		args.address = address
	}
}

func ReadLockLedger(lockLedger bool) UTXOIterateOption {
	return func(args *UTXOIterateOptions) {
		args.readLockLedger = lockLedger
	}
}

func MaxResultCount(count int) UTXOIterateOption {
	return func(args *UTXOIterateOptions) {
		args.maxResultCount = count
	}
}

func FilterOutputType(outputType iotago.OutputType) UTXOIterateOption {
	return func(args *UTXOIterateOptions) {
		args.filterOutputType = &outputType
	}
}

func iterateOptions(optionalOptions []UTXOIterateOption) *UTXOIterateOptions {
	result := &UTXOIterateOptions{
		address:          nil,
		readLockLedger:   true,
		maxResultCount:   0,
		filterOutputType: nil,
	}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}
	return result
}

func (u *Manager) SpentOutputs(options ...UTXOIterateOption) (Spents, error) {

	var spents []*Spent

	consumerFunc := func(spent *Spent) bool {
		spents = append(spents, spent)
		return true
	}

	if err := u.ForEachSpentOutput(consumerFunc, options...); err != nil {
		return nil, err
	}

	return spents, nil
}

func (u *Manager) UnspentOutputs(options ...UTXOIterateOption) ([]*Output, error) {

	var outputs []*Output
	consumerFunc := func(output *Output) bool {
		outputs = append(outputs, output)
		return true
	}

	if err := u.ForEachUnspentOutput(consumerFunc, options...); err != nil {
		return nil, err
	}

	return outputs, nil
}

func (u *Manager) ComputeBalance(options ...UTXOIterateOption) (balance uint64, count int, err error) {

	balance = 0
	count = 0
	consumerFunc := func(output *Output) bool {
		balance += output.amount
		count++
		return true
	}

	if err := u.ForEachUnspentOutput(consumerFunc, options...); err != nil {
		return 0, 0, err
	}

	return balance, count, nil
}
