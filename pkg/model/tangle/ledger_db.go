package tangle

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/store"
)

const (
	snapshotMilestoneIndexKey = "snapshotMilestoneIndex"
	ledgerMilestoneIndexKey   = "ledgerMilestoneIndex"
)

var (
	ledgerStore                kvstore.KVStore
	ledgerStoreTransactionLock sync.RWMutex

	ledgerMilestoneIndex milestone.Index

	snapshotBalancePrefix = []byte("sb")      // balance at snapshot milestone index
	ledgerBalancePrefix   = []byte("balance") // balance at solid milestone index
	diffPrefix            = []byte("diff")    // balance diffs at milestone index
)

func ReadLockLedger() {
	ledgerStoreTransactionLock.RLock()
}

func ReadUnlockLedger() {
	ledgerStoreTransactionLock.RUnlock()
}

func WriteLockLedger() {
	ledgerStoreTransactionLock.Lock()
}

func WriteUnlockLedger() {
	ledgerStoreTransactionLock.Unlock()
}

func configureLedgerStore() {
	ledgerStore = store.StoreWithPrefix(StorePrefixLedgerState)

	if err := readLedgerMilestoneIndexFromDatabase(); err != nil {
		panic(err)
	}
}

func databaseKeyForSnapshotAddressBalance(address trinary.Hash) []byte {
	return append(snapshotBalancePrefix, trinary.MustTrytesToBytes(address)[:49]...)
}

func databaseKeyForLedgerAddressBalance(address trinary.Hash) []byte {
	return append(ledgerBalancePrefix, trinary.MustTrytesToBytes(address)[:49]...)
}

func databaseKeyPrefixForLedgerDiff(milestoneIndex milestone.Index) []byte {
	return append(diffPrefix, databaseKeyForMilestoneIndex(milestoneIndex)...)
}

func databaseKeyForLedgerDiffAndAddress(milestoneIndex milestone.Index, address trinary.Hash) []byte {
	return append(databaseKeyPrefixForLedgerDiff(milestoneIndex), trinary.MustTrytesToBytes(address)[:49]...)
}

func bytesFromBalance(balance uint64) []byte {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, balance)
	return bytes
}

func balanceFromBytes(bytes []byte) uint64 {
	return binary.LittleEndian.Uint64(bytes)
}

func bytesFromDiff(diff int64) []byte {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, uint64(diff))
	return bytes
}

func diffFromBytes(bytes []byte) int64 {
	return int64(balanceFromBytes(bytes))
}

func readLedgerMilestoneIndexFromDatabase() error {

	ReadLockLedger()
	defer ReadUnlockLedger()

	value, err := ledgerStore.Get([]byte(ledgerMilestoneIndexKey))
	if err != nil {
		if err == kvstore.ErrKeyNotFound {
			return nil
		}
		return errors.Wrap(NewDatabaseError(err), "failed to load ledger milestone index")
	}
	ledgerMilestoneIndex = milestoneIndexFromBytes(value)

	// set the solid milestone index based on the ledger milestone
	SetSolidMilestoneIndex(ledgerMilestoneIndex, false)

	return nil
}

func GetBalanceForAddressWithoutLocking(address trinary.Hash) (uint64, milestone.Index, error) {

	value, err := ledgerStore.Get(databaseKeyForLedgerAddressBalance(address))
	if err != nil {
		if err == kvstore.ErrKeyNotFound {
			return 0, ledgerMilestoneIndex, nil
		} else {
			return 0, ledgerMilestoneIndex, errors.Wrap(NewDatabaseError(err), "failed to retrieve balance")
		}
	}

	return balanceFromBytes(value), ledgerMilestoneIndex, err
}

func GetBalanceForAddress(address trinary.Hash) (uint64, milestone.Index, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return GetBalanceForAddressWithoutLocking(address)
}

func DeleteLedgerDiffForMilestone(index milestone.Index) error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	if err := ledgerStore.DeletePrefix(databaseKeyPrefixForLedgerDiff(index)); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete ledger diff")
	}

	return nil
}

// GetLedgerDiffForMilestoneWithoutLocking returns the ledger changes of that specific milestone.
// ReadLockLedger must be held while entering this function.
func GetLedgerDiffForMilestoneWithoutLocking(index milestone.Index, abortSignal <-chan struct{}) (map[trinary.Hash]int64, error) {

	diff := make(map[trinary.Hash]int64)

	keyPrefix := databaseKeyPrefixForLedgerDiff(index)

	err := ledgerStore.Iterate([]kvstore.KeyPrefix{keyPrefix}, func(key kvstore.Key, value kvstore.Value) bool {
		select {
		case <-abortSignal:
			return false
		default:
		}
		// Remove prefix from key
		addressBytes := key[len(keyPrefix):]
		address := trinary.MustBytesToTrytes(addressBytes, 81)
		diff[address] = diffFromBytes(value)
		return true
	})

	if err != nil {
		return nil, err
	}

	var diffSum int64
	for _, change := range diff {
		diffSum += change
	}

	if diffSum != 0 {
		panic(fmt.Sprintf("GetLedgerDiffForMilestone(): Ledger diff for milestone %d does not sum up to zero", index))
	}

	return diff, nil
}

func GetLedgerDiffForMilestone(index milestone.Index, abortSignal <-chan struct{}) (map[trinary.Hash]int64, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return GetLedgerDiffForMilestoneWithoutLocking(index, abortSignal)
}

func GetLedgerStateForMilestoneWithoutLocking(targetIndex milestone.Index, abortSignal <-chan struct{}) (map[trinary.Hash]uint64, milestone.Index, error) {

	solidMilestoneIndex := GetSolidMilestoneIndex()
	if targetIndex == 0 {
		targetIndex = solidMilestoneIndex
	}

	if targetIndex > solidMilestoneIndex {
		return nil, 0, fmt.Errorf("target index is too new. maximum: %d, actual: %d", solidMilestoneIndex, targetIndex)
	}

	if targetIndex <= snapshot.PruningIndex {
		return nil, 0, fmt.Errorf("target index is too old. minimum: %d, actual: %d", snapshot.PruningIndex+1, targetIndex)
	}

	balances, ledgerMilestone, err := GetLedgerStateForLSMIWithoutLocking(abortSignal)
	if err != nil {
		return nil, 0, fmt.Errorf("GetLedgerStateForLSMI failed! %v", err)
	}

	if ledgerMilestone != solidMilestoneIndex {
		return nil, 0, fmt.Errorf("LedgerMilestone wrong! %d/%d", ledgerMilestone, solidMilestoneIndex)
	}

	// Calculate balances for targetIndex
	for milestoneIndex := solidMilestoneIndex; milestoneIndex > targetIndex; milestoneIndex-- {
		diff, err := GetLedgerDiffForMilestoneWithoutLocking(milestoneIndex, abortSignal)
		if err != nil {
			return nil, 0, fmt.Errorf("GetLedgerDiffForMilestone: %v", err)
		}

		for address, change := range diff {
			select {
			case <-abortSignal:
				return nil, 0, ErrOperationAborted
			default:
			}

			newBalance := int64(balances[address]) - change

			if newBalance < 0 {
				return nil, 0, fmt.Errorf("Ledger diff for milestone %d creates negative balance for address %s: current %d, diff %d", milestoneIndex, address, balances[address], change)
			} else if newBalance == 0 {
				delete(balances, address)
			} else {
				balances[address] = uint64(newBalance)
			}
		}
	}
	return balances, targetIndex, nil
}

func GetLedgerStateForMilestone(targetIndex milestone.Index, abortSignal <-chan struct{}) (map[trinary.Hash]uint64, milestone.Index, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return GetLedgerStateForMilestoneWithoutLocking(targetIndex, abortSignal)
}

// ApplyLedgerDiffWithoutLocking applies the changes to the ledger.
// WriteLockLedger must be held while entering this function.
func ApplyLedgerDiffWithoutLocking(diff map[trinary.Hash]int64, index milestone.Index) error {

	batch := ledgerStore.Batched()

	var diffSum int64

	for address, change := range diff {

		balance, _, err := GetBalanceForAddressWithoutLocking(address)
		if err != nil {
			panic(fmt.Sprintf("GetBalanceForAddressWithoutLocking() returned error for address %s: %v", address, err))
		}

		newBalance := int64(balance) + change

		if newBalance < 0 {
			panic(fmt.Sprintf("Ledger diff for milestone %d creates negative balance for address %s: current %d, diff %d", index, address, balance, change))
		} else if newBalance > 0 {
			// Save balance
			batch.Set(databaseKeyForLedgerAddressBalance(address), bytesFromBalance(uint64(newBalance)))
		} else {
			// Balance is zero, so we can remove this address from the ledger
			batch.Delete(databaseKeyForLedgerAddressBalance(address))
		}

		//Save diff
		batch.Set(databaseKeyForLedgerDiffAndAddress(index, address), bytesFromDiff(change))

		diffSum += change
	}

	if diffSum != 0 {
		panic(fmt.Sprintf("Ledger diff for milestone %d does not sum up to zero", index))
	}

	batch.Set([]byte(ledgerMilestoneIndexKey), bytesFromMilestoneIndex(index))

	// Now batch insert/delete all entries
	if err := batch.Commit(); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store ledger diff")
	}

	ledgerMilestoneIndex = index
	return nil
}

// StoreSnapshotBalancesInDatabase deletes all old entries and stores the ledger state of the snapshot index
func StoreSnapshotBalancesInDatabase(balances map[trinary.Hash]uint64, index milestone.Index) error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	// Delete all old entries
	if err := ledgerStore.DeletePrefix(snapshotBalancePrefix); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete old snapshot balances")
	}

	// Delete index
	if err := ledgerStore.Delete([]byte(snapshotMilestoneIndexKey)); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete old snapshot index")
	}

	batch := ledgerStore.Batched()

	for address, balance := range balances {
		key := databaseKeyForSnapshotAddressBalance(address)
		if balance != 0 {
			batch.Set(key, bytesFromBalance(balance))
		}
	}

	batch.Set([]byte(snapshotMilestoneIndexKey), bytesFromMilestoneIndex(index))

	// Now batch insert all entries
	if err := batch.Commit(); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store snapshot ledger state")
	}

	return nil
}

// GetAllSnapshotBalancesWithoutLocking returns all balances for the snapshot milestone.
// ReadLockLedger must be held while entering this function.
func GetAllSnapshotBalancesWithoutLocking(abortSignal <-chan struct{}) (map[trinary.Hash]uint64, milestone.Index, error) {

	balances := make(map[trinary.Hash]uint64)

	value, err := ledgerStore.Get([]byte(snapshotMilestoneIndexKey))
	if err != nil {
		return nil, 0, errors.Wrap(NewDatabaseError(err), "failed to retrieve snapshot milestone index")
	}

	snapshotMilestoneIndex := milestoneIndexFromBytes(value)

	err = ledgerStore.Iterate([]kvstore.KeyPrefix{snapshotBalancePrefix}, func(key kvstore.Key, value kvstore.Value) bool {
		select {
		case <-abortSignal:
			return false
		default:
		}
		// Remove prefix from key
		addressBytes := key[len(snapshotBalancePrefix):]
		address := trinary.MustBytesToTrytes(addressBytes, 81)
		balances[address] = balanceFromBytes(value)
		return true
	})

	if err != nil {
		return nil, 0, err
	}

	var total uint64
	for _, value := range balances {
		total += value
	}

	if total != consts.TotalSupply {
		panic(fmt.Sprintf("GetAllSnapshotBalances() Total does not match supply: %d != %d", total, consts.TotalSupply))
	}

	return balances, snapshotMilestoneIndex, err
}

// GetAllSnapshotBalances returns all balances for the snapshot milestone.
func GetAllSnapshotBalances(abortSignal <-chan struct{}) (map[trinary.Hash]uint64, milestone.Index, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return GetAllSnapshotBalancesWithoutLocking(abortSignal)
}

func DeleteLedgerBalancesInDatabase() error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	// Delete ledger all balances
	if err := ledgerStore.DeletePrefix(ledgerBalancePrefix); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete ledger balances")
	}

	return nil
}

func StoreLedgerBalancesInDatabase(balances map[trinary.Hash]uint64, index milestone.Index) error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	batch := ledgerStore.Batched()

	for address, balance := range balances {
		key := databaseKeyForLedgerAddressBalance(address)
		if balance == 0 {
			batch.Delete(key)
		} else {
			batch.Set(key, bytesFromBalance(balance))
		}
	}

	batch.Set([]byte(ledgerMilestoneIndexKey), bytesFromMilestoneIndex(index))

	if err := batch.Commit(); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store ledger state")
	}

	ledgerMilestoneIndex = index
	return nil
}

// GetLedgerStateForLSMIWithoutLocking returns all balances for the current solid milestone.
// ReadLockLedger must be held while entering this function.
func GetLedgerStateForLSMIWithoutLocking(abortSignal <-chan struct{}) (map[trinary.Hash]uint64, milestone.Index, error) {

	balances := make(map[trinary.Hash]uint64)

	err := ledgerStore.Iterate([]kvstore.KeyPrefix{ledgerBalancePrefix}, func(key kvstore.Key, value kvstore.Value) bool {
		select {
		case <-abortSignal:
			return false
		default:
		}

		// Remove prefix from key
		addressBytes := key[len(ledgerBalancePrefix):]
		address := trinary.MustBytesToTrytes(addressBytes, 81)
		balances[address] = balanceFromBytes(value)
		return true
	})
	if err != nil {
		return nil, ledgerMilestoneIndex, err
	}

	var total uint64
	for _, value := range balances {
		total += value
	}

	if total != consts.TotalSupply {
		panic(fmt.Sprintf("total does not match supply: %d != %d", total, consts.TotalSupply))
	}

	return balances, ledgerMilestoneIndex, err
}

// GetLedgerStateForLSMI returns all balances for the current solid milestone.
func GetLedgerStateForLSMI(abortSignal <-chan struct{}) (map[trinary.Hash]uint64, milestone.Index, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return GetLedgerStateForLSMIWithoutLocking(abortSignal)
}
