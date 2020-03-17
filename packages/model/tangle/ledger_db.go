package tangle

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/typeutils"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/compressed"
	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/milestone_index"
)

const (
	snapshotMilestoneIndexKey = "snapshotMilestoneIndex"
	ledgerMilestoneIndexKey   = "ledgerMilestoneIndex"
)

var (
	ledgerDatabase                database.Database
	ledgerDatabaseTransactionLock sync.RWMutex

	ledgerMilestoneIndex milestone_index.MilestoneIndex

	snapshotBalancePrefix = []byte("sb")      // balance at snapshot milestone index
	ledgerBalancePrefix   = []byte("balance") // balance at solid milestone index
	diffPrefix            = []byte("diff")    // balance diffs at milestone index
)

func ReadLockLedger() {
	ledgerDatabaseTransactionLock.RLock()
}

func ReadUnlockLedger() {
	ledgerDatabaseTransactionLock.RUnlock()
}

func WriteLockLedger() {
	ledgerDatabaseTransactionLock.Lock()
}

func WriteUnlockLedger() {
	ledgerDatabaseTransactionLock.Unlock()
}

func configureLedgerDatabase() {
	if db, err := database.Get(DBPrefixLedgerState, database.GetHornetBadgerInstance()); err != nil {
		panic(err)
	} else {
		ledgerDatabase = db
	}

	loadLSMIAsLSM := config.NodeConfig.GetBool(config.CfgCompassLoadLSMIAsLMI)
	err := readLedgerMilestoneIndexFromDatabase(loadLSMIAsLSM)
	if err != nil {
		panic(err)
	}
}

func databaseKeyForSnapshotAddressBalance(address trinary.Hash) []byte {
	return append(snapshotBalancePrefix, trinary.MustTrytesToBytes(address)[:49]...)
}

func databaseKeyForLedgerAddressBalance(address trinary.Hash) []byte {
	return append(ledgerBalancePrefix, trinary.MustTrytesToBytes(address)[:49]...)
}

func databaseKeyPrefixForLedgerDiff(milestoneIndex milestone_index.MilestoneIndex) []byte {
	return append(diffPrefix, databaseKeyForMilestoneIndex(milestoneIndex)...)
}

func databaseKeyForLedgerDiffAndAddress(milestoneIndex milestone_index.MilestoneIndex, address trinary.Hash) []byte {
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

func entryForSnapshotMilestoneIndex(index milestone_index.MilestoneIndex) database.Entry {
	return database.Entry{
		Key:   typeutils.StringToBytes(snapshotMilestoneIndexKey),
		Value: bytesFromMilestoneIndex(index),
	}
}

func entryForLedgerMilestoneIndex(index milestone_index.MilestoneIndex) database.Entry {
	return database.Entry{
		Key:   typeutils.StringToBytes(ledgerMilestoneIndexKey),
		Value: bytesFromMilestoneIndex(index),
	}
}

func readLedgerMilestoneIndexFromDatabase(setLSMIAsLMI bool) error {

	ReadLockLedger()
	defer ReadUnlockLedger()

	entry, err := ledgerDatabase.Get(typeutils.StringToBytes(ledgerMilestoneIndexKey))
	if err != nil {
		if err == database.ErrKeyNotFound {
			return nil
		}
		return errors.Wrap(NewDatabaseError(err), "failed to retrieve ledger milestone index")
	}
	ledgerMilestoneIndex = milestoneIndexFromBytes(entry.Value)

	// Set the solid milestone index based on the ledger milestone
	setSolidMilestoneIndex(ledgerMilestoneIndex)
	if setLSMIAsLMI && ledgerMilestoneIndex != 0 {
		cachedSolidMs := GetMilestoneOrNil(ledgerMilestoneIndex) // bundle +1
		if cachedSolidMs != nil {
			err = SetLatestMilestone(cachedSolidMs.Retain()) // bundle pass +1
			cachedSolidMs.Release()                          // bundle -1
			if err != nil {
				return errors.Wrap(NewDatabaseError(err), "failed to set the latest milestone")
			}
		}
	}

	return nil
}

func GetBalanceForAddressWithoutLocking(address trinary.Hash) (uint64, milestone_index.MilestoneIndex, error) {

	entry, err := ledgerDatabase.Get(databaseKeyForLedgerAddressBalance(address))
	if err != nil {
		if err == database.ErrKeyNotFound {
			return 0, ledgerMilestoneIndex, nil
		} else {
			return 0, ledgerMilestoneIndex, errors.Wrap(NewDatabaseError(err), "failed to retrieve balance")
		}
	}

	return balanceFromBytes(entry.Value), ledgerMilestoneIndex, err
}

func GetBalanceForAddress(address trinary.Hash) (uint64, milestone_index.MilestoneIndex, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return GetBalanceForAddressWithoutLocking(address)
}

func DeleteLedgerDiffForMilestone(index milestone_index.MilestoneIndex) error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	var deletions []database.Key

	err := ledgerDatabase.StreamForEachPrefixKeyOnly(databaseKeyPrefixForLedgerDiff(index), func(entry database.KeyOnlyEntry) error {
		deletions = append(deletions, entry.Key)
		return nil
	})

	if err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete ledger diff")
	}

	// Now batch delete all entries
	if err := ledgerDatabase.Apply([]database.Entry{}, deletions); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete ledger diff")
	}

	return nil
}

// GetLedgerDiffForMilestoneWithoutLocking returns the ledger changes of that specific milestone.
// ReadLockLedger must be held while entering this function.
func GetLedgerDiffForMilestoneWithoutLocking(index milestone_index.MilestoneIndex, abortSignal <-chan struct{}) (map[trinary.Hash]int64, error) {

	diff := make(map[trinary.Hash]int64)

	err := ledgerDatabase.StreamForEachPrefix(databaseKeyPrefixForLedgerDiff(index), func(entry database.Entry) error {
		select {
		case <-abortSignal:
			return ErrOperationAborted
		default:
		}

		address := trinary.MustBytesToTrytes(entry.Key, 81)
		diff[address] = diffFromBytes(entry.Value)
		return nil
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

func GetLedgerDiffForMilestone(index milestone_index.MilestoneIndex, abortSignal <-chan struct{}) (map[trinary.Hash]int64, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return GetLedgerDiffForMilestoneWithoutLocking(index, abortSignal)
}

// ApplyLedgerDiffWithoutLocking applies the changes to the ledger.
// WriteLockLedger must be held while entering this function.
func ApplyLedgerDiffWithoutLocking(diff map[trinary.Hash]int64, index milestone_index.MilestoneIndex) error {

	var diffEntries []database.Entry
	var balanceChanges []database.Entry
	var emptyAddresses []database.Key

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
			balanceChanges = append(balanceChanges, database.Entry{
				Key:   databaseKeyForLedgerAddressBalance(address),
				Value: bytesFromBalance(uint64(newBalance)),
			})
		} else {
			// Balance is zero, so we can remove this address from the ledger
			emptyAddresses = append(emptyAddresses, databaseKeyForLedgerAddressBalance(address))
		}

		diffEntries = append(diffEntries, database.Entry{
			Key:   databaseKeyForLedgerDiffAndAddress(index, address),
			Value: bytesFromDiff(change),
		})

		diffSum += change
	}

	if diffSum != 0 {
		panic(fmt.Sprintf("Ledger diff for milestone %d does not sum up to zero", index))
	}

	entries := balanceChanges
	entries = append(entries, diffEntries...)
	entries = append(entries, entryForLedgerMilestoneIndex(index))
	deletions := emptyAddresses

	// Now batch insert/delete all entries
	if err := ledgerDatabase.Apply(entries, deletions); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store ledger diff")
	}

	ledgerMilestoneIndex = index
	return nil
}

// StoreSnapshotBalancesInDatabase deletes all old entries and stores the ledger state of the snapshot index
func StoreSnapshotBalancesInDatabase(balances map[trinary.Hash]uint64, index milestone_index.MilestoneIndex) error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	var entries []database.Entry
	var deletions []database.Key

	// Delete all old entries
	err := ledgerDatabase.StreamForEachPrefixKeyOnly(snapshotBalancePrefix, func(entry database.KeyOnlyEntry) error {
		deletions = append(deletions, entry.Key)
		return nil
	})
	if err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete snapshot balances")
	}
	deletions = append(deletions, typeutils.StringToBytes(snapshotMilestoneIndexKey))

	for address, balance := range balances {
		key := databaseKeyForSnapshotAddressBalance(address)
		if balance != 0 {
			entries = append(entries, database.Entry{
				Key:   key,
				Value: bytesFromBalance(balance),
			})
		}
	}

	entries = append(entries, entryForSnapshotMilestoneIndex(index))

	// Now batch insert/delete all entries
	if err := ledgerDatabase.Apply(entries, deletions); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store snapshot ledger state")
	}

	return nil
}

// GetAllSnapshotBalancesWithoutLocking returns all balances for the snapshot milestone.
// ReadLockLedger must be held while entering this function.
func GetAllSnapshotBalancesWithoutLocking(abortSignal <-chan struct{}) (map[trinary.Hash]uint64, milestone_index.MilestoneIndex, error) {

	balances := make(map[trinary.Hash]uint64)

	entry, err := ledgerDatabase.Get(typeutils.StringToBytes(snapshotMilestoneIndexKey))
	if err != nil {
		return nil, 0, errors.Wrap(NewDatabaseError(err), "failed to retrieve snapshot milestone index")
	}

	snapshotMilestoneIndex := milestoneIndexFromBytes(entry.Value)

	err = ledgerDatabase.StreamForEachPrefix(snapshotBalancePrefix, func(entry database.Entry) error {
		select {
		case <-abortSignal:
			return ErrOperationAborted
		default:
		}

		address := trinary.MustBytesToTrytes(entry.Key, 81)
		balances[address] = balanceFromBytes(entry.Value)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	var total uint64
	for _, value := range balances {
		total += value
	}

	if total != compressed.TOTAL_SUPPLY {
		panic(fmt.Sprintf("GetAllSnapshotBalances() Total does not match supply: %d != %d", total, compressed.TOTAL_SUPPLY))
	}

	return balances, snapshotMilestoneIndex, err
}

// GetAllSnapshotBalances returns all balances for the snapshot milestone.
func GetAllSnapshotBalances(abortSignal <-chan struct{}) (map[trinary.Hash]uint64, milestone_index.MilestoneIndex, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return GetAllSnapshotBalancesWithoutLocking(abortSignal)
}

func DeleteLedgerBalancesInDatabase() error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	var deletions []database.Key

	err := ledgerDatabase.StreamForEachPrefixKeyOnly(ledgerBalancePrefix, func(entry database.KeyOnlyEntry) error {
		deletions = append(deletions, entry.Key)
		return nil
	})

	if err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete ledger balances")
	}

	// Now batch delete all entries
	if err := ledgerDatabase.Apply([]database.Entry{}, deletions); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete ledger balances")
	}

	return nil
}

func StoreLedgerBalancesInDatabase(balances map[trinary.Hash]uint64, index milestone_index.MilestoneIndex) error {

	WriteLockLedger()
	defer WriteUnlockLedger()

	var entries []database.Entry
	var deletions []database.Key

	for address, balance := range balances {
		key := databaseKeyForLedgerAddressBalance(address)
		if balance == 0 {
			deletions = append(deletions, key)
		} else {
			entries = append(entries, database.Entry{
				Key:   key,
				Value: bytesFromBalance(balance),
			})
		}
	}

	entries = append(entries, entryForLedgerMilestoneIndex(index))

	// Now batch insert/delete all entries
	if err := ledgerDatabase.Apply(entries, deletions); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store ledger state")
	}

	ledgerMilestoneIndex = index
	return nil
}

// GetAllLedgerBalancesWithoutLocking returns all balances for the current solid milestone.
// ReadLockLedger must be held while entering this function.
func GetAllLedgerBalancesWithoutLocking(abortSignal <-chan struct{}) (map[trinary.Hash]uint64, milestone_index.MilestoneIndex, error) {

	balances := make(map[trinary.Hash]uint64)

	err := ledgerDatabase.StreamForEachPrefix(ledgerBalancePrefix, func(entry database.Entry) error {
		select {
		case <-abortSignal:
			return ErrOperationAborted
		default:
		}

		address := trinary.MustBytesToTrytes(entry.Key, 81)
		balances[address] = balanceFromBytes(entry.Value)
		return nil
	})

	if err != nil {
		return nil, ledgerMilestoneIndex, err
	}

	var total uint64
	for _, value := range balances {
		total += value
	}

	if total != compressed.TOTAL_SUPPLY {
		panic(fmt.Sprintf("GetAllLedgerBalances() Total does not match supply: %d != %d", total, compressed.TOTAL_SUPPLY))
	}

	return balances, ledgerMilestoneIndex, err
}

// GetAllLedgerBalances returns all balances for the current solid milestone.
func GetAllLedgerBalances(abortSignal <-chan struct{}) (map[trinary.Hash]uint64, milestone_index.MilestoneIndex, error) {

	ReadLockLedger()
	defer ReadUnlockLedger()

	return GetAllLedgerBalancesWithoutLocking(abortSignal)
}
