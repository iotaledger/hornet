package utxo

import (
	"encoding/hex"
	"fmt"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/milestone"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	iotago "github.com/iotaledger/iota.go/v2"
)

var (
	// ErrInvalidBalancesTotalSupply is returned when the sum of all balances does not match total supply.
	ErrInvalidBalancesTotalSupply = errors.New("invalid balances total supply")

	// ErrInvalidBalanceOnAddress is returned when the balance on an address is invalid.
	ErrInvalidBalanceOnAddress = errors.New("invalid balance on address")

	// ErrInvalidDustForAddress is returned when the dust for an address is invalid.
	ErrInvalidDustForAddress = errors.New("invalid dust for address")
)

func balanceFromBytes(value []byte) (balance uint64, dustAllowanceBalance uint64, outputCount int64, err error) {
	marshalUtil := marshalutil.New(value)

	if balance, err = marshalUtil.ReadUint64(); err != nil {
		return
	}

	if dustAllowanceBalance, err = marshalUtil.ReadUint64(); err != nil {
		return
	}

	if outputCount, err = marshalUtil.ReadInt64(); err != nil {
		return
	}

	return
}

func bytesFromBalance(balance uint64, dustAllowanceBalance uint64, outputCount int64) []byte {
	marshalUtil := marshalutil.New(24)
	marshalUtil.WriteUint64(balance)              // 8 bytes
	marshalUtil.WriteUint64(dustAllowanceBalance) // 8 bytes
	marshalUtil.WriteInt64(outputCount)           // 8 bytes
	return marshalUtil.Bytes()
}

func (u *Manager) checkBalancesLedger(treasury uint64) error {

	var balanceSum uint64
	var innerErr error

	key := []byte{UTXOStoreKeyPrefixBalances}

	if err := u.utxoStorage.IterateKeys(key, func(key kvstore.Key) bool {

		value, err := u.utxoStorage.Get(key)
		if err != nil {
			innerErr = err
			return false
		}

		balance, _, _, err := balanceFromBytes(value)
		if err != nil {
			innerErr = err
			return false
		}

		balanceSum += balance

		return true
	}); err != nil {
		return err
	}

	if innerErr != nil {
		return innerErr
	}

	balanceSum += treasury

	if balanceSum != iotago.TokenSupply {
		return ErrInvalidBalancesTotalSupply
	}

	return nil
}

func (u *Manager) AddressBalance(address iotago.Address) (balance uint64, dustAllowed bool, ledgerIndex milestone.Index, err error) {

	u.ReadLockLedger()
	defer u.ReadUnlockLedger()

	ledgerIndex, err = u.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return 0, false, 0, err
	}

	balance, dustAllowed, err = u.AddressBalanceWithoutLocking(address)
	if err != nil {
		return 0, false, 0, err
	}

	return balance, dustAllowed, ledgerIndex, err
}

func (u *Manager) AddressBalanceWithoutLocking(address iotago.Address) (balance uint64, dustAllowed bool, err error) {

	addressKey, err := address.Serialize(iotago.DeSeriModeNoValidation)
	if err != nil {
		return 0, false, err
	}

	b, dustAllowance, dustOutputCount, err := u.readBalanceForAddress(addressKey)
	if err != nil {
		return 0, false, err
	}

	// There is no built-in min function for int64, so inline one here
	min := func(x, y int64) int64 {
		if x > y {
			return y
		}
		return x
	}

	dustAllowed = min(int64(dustAllowance)/iotago.DustAllowanceDivisor, iotago.MaxDustOutputsOnAddress) > dustOutputCount

	return b, dustAllowed, nil
}

func (u *Manager) ReadDustForAddress(address iotago.Address, applyDiff *BalanceDiff) (dustAllowanceBalance uint64, dustOutputCount int64, err error) {

	addressKey, err := address.Serialize(iotago.DeSeriModeNoValidation)
	if err != nil {
		return 0, 0, err
	}

	_, dustAllowance, count, err := u.readBalanceForAddress(addressKey)
	if err != nil {
		return 0, 0, err
	}

	_, diffDustAllowance, diffCount, err := applyDiff.DiffForAddress(address)
	if err != nil {
		return 0, 0, err
	}

	newDustAllowance := int64(dustAllowance) + diffDustAllowance
	if newDustAllowance < 0 {
		return 0, 0, fmt.Errorf("%w: negative dust balance on address %s", iotago.ErrInvalidDustAllowance, address.String())
	}
	dustAllowance = uint64(newDustAllowance)

	count += diffCount

	return dustAllowance, count, nil
}

func (u *Manager) readBalanceForAddress(addressKey []byte) (balance uint64, dustAllowanceBalance uint64, dustOutputCount int64, err error) {

	dbKey := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixBalances}, addressKey)

	value, err := u.utxoStorage.Get(dbKey)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			// No dust information found in the database for this address
			return 0, 0, 0, nil
		}
		return 0, 0, 0, err
	}

	return balanceFromBytes(value)
}

func (u *Manager) storeBalanceForAddress(addressKey []byte, balance uint64, dustAllowanceBalance uint64, dustOutputCount int64, mutations kvstore.BatchedMutations) error {

	dbKey := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixBalances}, addressKey)

	if balance == 0 && dustOutputCount == 0 && dustAllowanceBalance == 0 {
		// Remove from database
		return mutations.Delete(dbKey)
	}

	return mutations.Set(dbKey, bytesFromBalance(balance, dustAllowanceBalance, dustOutputCount))
}

// This applies the diff to the current database.
func (u *Manager) applyBalanceDiff(allowance *BalanceDiff, mutations kvstore.BatchedMutations) error {

	for addressMapKey, diff := range allowance.balances {
		if err := u.applyBalanceDiffForAddress([]byte(addressMapKey), diff.balanceDiff, diff.dustAllowanceBalanceDiff, diff.dustOutputCountDiff, mutations); err != nil {
			return err
		}
	}
	return nil
}

// This applies the diff to the current address by first reading the current value and adding the diff on it.
func (u *Manager) applyBalanceDiffForAddress(addressKey []byte, balanceDiff int64, dustAllowanceBalanceDiff int64, dustOutputCountDiff int64, mutations kvstore.BatchedMutations) error {

	balance, dustAllowanceBalance, dustOutputCount, err := u.readBalanceForAddress(addressKey)
	if err != nil {
		return err
	}

	newBalance := int64(balance) + balanceDiff
	newDustAllowanceBalance := int64(dustAllowanceBalance) + dustAllowanceBalanceDiff
	newDustOutputCount := dustOutputCount + dustOutputCountDiff

	if newBalance < 0 {
		// Balance cannot be negative
		return fmt.Errorf("%w: %s balance %d", ErrInvalidBalanceOnAddress, hex.EncodeToString(addressKey), balance)
	}

	if newDustOutputCount < 0 || newDustAllowanceBalance < 0 {
		// Count or dust allowance cannot be negative
		return fmt.Errorf("%w: %s dustAllowanceBalance %d, dustOutputCount %d", ErrInvalidDustForAddress, hex.EncodeToString(addressKey), dustAllowanceBalance, dustOutputCount)
	}

	return u.storeBalanceForAddress(addressKey, uint64(newBalance), uint64(newDustAllowanceBalance), newDustOutputCount, mutations)
}

func (u *Manager) applyNewBalancesWithoutLocking(newOutputs Outputs, newSpents Spents, mutations kvstore.BatchedMutations) error {

	balances := NewBalanceDiff()
	if err := balances.Add(newOutputs, newSpents); err != nil {
		return err
	}
	return u.applyBalanceDiff(balances, mutations)
}

func (u *Manager) rollbackBalancesWithoutLocking(newOutputs Outputs, newSpents Spents, mutations kvstore.BatchedMutations) error {
	balances := NewBalanceDiff()
	if err := balances.Remove(newOutputs, newSpents); err != nil {
		return err
	}
	return u.applyBalanceDiff(balances, mutations)
}

func (u *Manager) storeBalanceForUnspentOutput(unspentOutput *Output, mutations kvstore.BatchedMutations) error {
	balances := NewBalanceDiff()
	if err := balances.Add([]*Output{unspentOutput}, []*Spent{}); err != nil {
		return err
	}
	return u.applyBalanceDiff(balances, mutations)
}
