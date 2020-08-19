package tangle

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/iota.go/consts"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

const (
	ledgerMilestoneIndexKey = "ledgerMilestoneIndex"
)

var (
	ledgerStore           kvstore.KVStore
	ledgerBalanceStore    kvstore.KVStore
	ledgerDiffStore       kvstore.KVStore
	ledgerTransactionLock sync.RWMutex

	ledgerMilestoneIndex milestone.Index
)

func ReadLockLedger() {
	ledgerTransactionLock.RLock()
}

func ReadUnlockLedger() {
	ledgerTransactionLock.RUnlock()
}

func WriteLockLedger() {
	ledgerTransactionLock.Lock()
}

func WriteUnlockLedger() {
	ledgerTransactionLock.Unlock()
}

func configureLedgerStore(store kvstore.KVStore) {
	ledgerStore = store.WithRealm([]byte{StorePrefixLedgerState})
	ledgerBalanceStore = store.WithRealm([]byte{StorePrefixLedgerBalance})
	ledgerDiffStore = store.WithRealm([]byte{StorePrefixLedgerDiff})

	if err := readLedgerMilestoneIndexFromDatabase(); err != nil {
		panic(err)
	}
}

func databaseKeyForAddress(address hornet.Hash) []byte {
	return address[:49]
}

func databaseKeyForLedgerDiffAndAddress(milestoneIndex milestone.Index, address hornet.Hash) []byte {
	return append(databaseKeyForMilestoneIndex(milestoneIndex), address[:49]...)
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
		if err != kvstore.ErrKeyNotFound {
			return errors.Wrap(NewDatabaseError(err), "failed to load ledger milestone index")
		}
		return nil
	}
	ledgerMilestoneIndex = milestoneIndexFromBytes(value)

	// set the solid milestone index based on the ledger milestone
	SetSolidMilestoneIndex(ledgerMilestoneIndex, false)

	return nil
}

func GetBalanceForAddressWithoutLocking(address hornet.Hash) (uint64, milestone.Index, error) {

	value, err := ledgerBalanceStore.Get(databaseKeyForAddress(address))
	if err != nil {
		if err != kvstore.ErrKeyNotFound {
			return 0, ledgerMilestoneIndex, errors.Wrap(NewDatabaseError(err), "failed to retrieve balance")
		}
		return 0, ledgerMilestoneIndex, nil
	}

	return balanceFromBytes(value), ledgerMilestoneIndex, err
}

func GetBalanceForAddress(address hornet.Hash) (uint64, milestone.Index, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return GetBalanceForAddressWithoutLocking(address)
}

func DeleteLedgerDiffForMilestone(index milestone.Index) error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	if err := ledgerDiffStore.DeletePrefix(databaseKeyForMilestoneIndex(index)); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete ledger diff")
	}

	return nil
}

// GetLedgerDiffForMilestoneWithoutLocking returns the ledger changes of that specific milestone.
// ReadLockLedger must be held while entering this function.
func GetLedgerDiffForMilestoneWithoutLocking(index milestone.Index, abortSignal <-chan struct{}) (map[string]int64, error) {

	diff := make(map[string]int64)

	keyPrefix := databaseKeyForMilestoneIndex(index)

	aborted := false
	err := ledgerDiffStore.Iterate(keyPrefix, func(key kvstore.Key, value kvstore.Value) bool {
		select {
		case <-abortSignal:
			aborted = true
			return false
		default:
		}
		// Remove prefix from key
		diff[string(key[len(keyPrefix):len(keyPrefix)+49])] = diffFromBytes(value)
		return true
	})

	if err != nil {
		return nil, err
	}

	if aborted {
		return nil, ErrOperationAborted
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

// LedgerDiffHashConsumer consumes the given ledger diff addresses during looping through all ledger diffs in the persistence layer.
type LedgerDiffHashConsumer func(msIndex milestone.Index, address hornet.Hash) bool

// ForEachLedgerDiffHash loops over all ledger diffs.
func ForEachLedgerDiffHash(consumer LedgerDiffHashConsumer, skipCache bool) {
	ledgerDiffStore.IterateKeys([]byte{}, func(key kvstore.Key) bool {
		return consumer(milestone.Index(binary.LittleEndian.Uint32(key[:4])), key[4:53])
	})
}

func GetLedgerDiffForMilestone(index milestone.Index, abortSignal <-chan struct{}) (map[string]int64, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return GetLedgerDiffForMilestoneWithoutLocking(index, abortSignal)
}

func GetLedgerStateForMilestoneWithoutLocking(targetIndex milestone.Index, abortSignal <-chan struct{}) (map[string]uint64, milestone.Index, error) {

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
		if err == ErrOperationAborted {
			return nil, 0, err
		}
		return nil, 0, fmt.Errorf("GetLedgerStateForLSMI failed! %v", err)
	}

	if ledgerMilestone != solidMilestoneIndex {
		return nil, 0, fmt.Errorf("LedgerMilestone wrong! %d/%d", ledgerMilestone, solidMilestoneIndex)
	}

	// Calculate balances for targetIndex
	for milestoneIndex := solidMilestoneIndex; milestoneIndex > targetIndex; milestoneIndex-- {
		diff, err := GetLedgerDiffForMilestoneWithoutLocking(milestoneIndex, abortSignal)
		if err != nil {
			if err == ErrOperationAborted {
				return nil, 0, err
			}
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
				return nil, 0, fmt.Errorf("Ledger diff for milestone %d creates negative balance for address %s: current %d, diff %d", milestoneIndex, hornet.Hash(address).Trytes(), balances[address], change)
			} else if newBalance == 0 {
				delete(balances, address)
			} else {
				balances[address] = uint64(newBalance)
			}
		}
	}
	return balances, targetIndex, nil
}

func GetLedgerStateForMilestone(targetIndex milestone.Index, abortSignal <-chan struct{}) (map[string]uint64, milestone.Index, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return GetLedgerStateForMilestoneWithoutLocking(targetIndex, abortSignal)
}

// ApplyLedgerDiffWithoutLocking applies the changes to the ledger.
// WriteLockLedger must be held while entering this function.
func ApplyLedgerDiffWithoutLocking(diff map[string]int64, index milestone.Index) error {

	balanceBatch := ledgerBalanceStore.Batched()
	diffBatch := ledgerDiffStore.Batched()

	var diffSum int64

	for address, change := range diff {

		balance, _, err := GetBalanceForAddressWithoutLocking(hornet.Hash(address))
		if err != nil {
			panic(fmt.Sprintf("GetBalanceForAddressWithoutLocking() returned error for address %s: %v", address, err))
		}

		newBalance := int64(balance) + change

		if newBalance < 0 {
			panic(fmt.Sprintf("Ledger diff for milestone %d creates negative balance for address %s: current %d, diff %d", index, hornet.Hash(address).Trytes(), balance, change))
		} else if newBalance > 0 {
			// Save balance
			balanceBatch.Set(databaseKeyForAddress(hornet.Hash(address)), bytesFromBalance(uint64(newBalance)))
		} else {
			// Balance is zero, so we can remove this address from the ledger
			balanceBatch.Delete(databaseKeyForAddress(hornet.Hash(address)))
		}

		//Save diff
		diffBatch.Set(databaseKeyForLedgerDiffAndAddress(index, hornet.Hash(address)), bytesFromDiff(change))

		diffSum += change
	}

	if diffSum != 0 {
		panic(fmt.Sprintf("Ledger diff for milestone %d does not sum up to zero", index))
	}

	if err := diffBatch.Commit(); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store ledger diff")
	}

	if err := balanceBatch.Commit(); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store ledger balance")
	}

	if err := ledgerStore.Set([]byte(ledgerMilestoneIndexKey), bytesFromMilestoneIndex(index)); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store ledger index")
	}

	ledgerMilestoneIndex = index
	return nil
}

func StoreLedgerBalancesInDatabase(balances map[string]uint64, index milestone.Index) error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	// Delete all ledger balances
	if err := ledgerBalanceStore.Clear(); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete ledger balances")
	}

	balanceBatch := ledgerBalanceStore.Batched()

	for address, balance := range balances {
		if balance == 0 {
			balanceBatch.Delete(databaseKeyForAddress(hornet.Hash(address)))
		} else {
			balanceBatch.Set(databaseKeyForAddress(hornet.Hash(address)), bytesFromBalance(balance))
		}
	}

	if err := balanceBatch.Commit(); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store ledger state")
	}

	if err := ledgerStore.Set([]byte(ledgerMilestoneIndexKey), bytesFromMilestoneIndex(index)); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store ledger index")
	}

	ledgerMilestoneIndex = index
	return nil
}

// GetLedgerStateForLSMIWithoutLocking returns all balances for the current solid milestone.
// ReadLockLedger must be held while entering this function.
func GetLedgerStateForLSMIWithoutLocking(abortSignal <-chan struct{}) (map[string]uint64, milestone.Index, error) {

	balances := make(map[string]uint64)

	aborted := false
	err := ledgerBalanceStore.Iterate(kvstore.EmptyPrefix, func(key kvstore.Key, value kvstore.Value) bool {
		select {
		case <-abortSignal:
			aborted = true
			return false
		default:
		}

		balances[string(key[:49])] = balanceFromBytes(value)
		return true
	})
	if err != nil {
		return nil, ledgerMilestoneIndex, err
	}

	if aborted {
		return nil, ledgerMilestoneIndex, ErrOperationAborted
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
func GetLedgerStateForLSMI(abortSignal <-chan struct{}) (map[string]uint64, milestone.Index, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return GetLedgerStateForLSMIWithoutLocking(abortSignal)
}
