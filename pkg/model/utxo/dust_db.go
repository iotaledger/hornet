package utxo

import (
	"encoding/hex"
	"fmt"

	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go"
)

var (
	// ErrInvalidDustForAddress is returned when the dust for an address is invalid.
	ErrInvalidDustForAddress = errors.New("invalid dust for address")
)

func dustFromBytes(value []byte) (dustAllowanceBalance uint64, outputCount int64, err error) {
	marshalUtil := marshalutil.New(value)

	if dustAllowanceBalance, err = marshalUtil.ReadUint64(); err != nil {
		return
	}

	if outputCount, err = marshalUtil.ReadInt64(); err != nil {
		return
	}

	return
}

func bytesFromDust(dustAllowanceBalance uint64, outputCount int64) []byte {
	marshalUtil := marshalutil.New(16)
	marshalUtil.WriteUint64(dustAllowanceBalance)
	marshalUtil.WriteInt64(outputCount)
	return marshalUtil.Bytes()
}

func (u *Manager) ReadDustForAddress(address iotago.Address, applyDiff *DustAllowanceDiff) (dustAllowanceBalance uint64, dustOutputCount int64, err error) {

	addressKey, err := address.Serialize(iotago.DeSeriModeNoValidation)
	if err != nil {
		return 0, 0, err
	}

	balance, count, err := u.readDustForAddress(addressKey)
	if err != nil {
		return 0, 0, err
	}

	diffBalance, diffCount, err := applyDiff.DiffForAddress(address)
	if err != nil {
		return 0, 0, err
	}

	newBalance := int64(balance) + diffBalance
	if newBalance < 0 {
		return 0, 0, fmt.Errorf("%w: negative dust balance on address %s", iotago.ErrInvalidDustAllowance, address.String())
	}
	balance = uint64(newBalance)

	count += diffCount

	return balance, count, nil
}

func (u *Manager) readDustForAddress(addressKey []byte) (dustAllowanceBalance uint64, dustOutputCount int64, err error) {

	value, err := u.dustStorage.Get(addressKey)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			// No dust information found in the database for this address
			return 0, 0, nil
		}
		return 0, 0, err
	}

	if value != nil {
		return dustFromBytes(value)
	}

	return 0, 0, nil
}

func (u *Manager) storeDustForAddress(addressKey []byte, dustAllowanceBalance uint64, dustOutputCount int64, mutations kvstore.BatchedMutations) error {

	if dustOutputCount == 0 {
		if dustAllowanceBalance != 0 {
			// Balance cannot be non-zero if there are no outputs
			return fmt.Errorf("%w: %s dustAllowanceBalance %d, dustOutputCount %d", ErrInvalidDustForAddress, hex.EncodeToString(addressKey), dustAllowanceBalance, dustOutputCount)
		}

		// Remove from database
		return mutations.Delete(addressKey)
	}

	return mutations.Set(addressKey, bytesFromDust(dustAllowanceBalance, dustOutputCount))
}

// This applies the diff to the current database
func (u *Manager) applyDustAllowanceDiff(allowance *DustAllowanceDiff) error {

	mutations := u.dustStorage.Batched()
	for addressMapKey, diff := range allowance.allowance {
		if err := u.applyDustDiffForAddress([]byte(addressMapKey), diff.allowanceBalanceDiff, diff.outputCount, mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}
	return mutations.Commit()
}

// This applies the diff to the current address by first reading the current value and adding the diff on it
func (u *Manager) applyDustDiffForAddress(addressKey []byte, dustAllowanceBalanceDiff int64, dustOutputCountDiff int64, mutations kvstore.BatchedMutations) error {

	dustAllowanceBalance, dustOutputCount, err := u.readDustForAddress(addressKey)
	if err != nil {
		return err
	}

	newDustAllowanceBalance := int64(dustAllowanceBalance) + dustAllowanceBalanceDiff
	newDustOutputCount := dustOutputCount + dustOutputCountDiff

	if newDustOutputCount < 0 || newDustAllowanceBalance < 0 {
		// Count or balance cannot be negative
		return fmt.Errorf("%w: %s dustAllowanceBalance %d, dustOutputCount %d", ErrInvalidDustForAddress, hex.EncodeToString(addressKey), dustAllowanceBalance, dustOutputCount)
	}

	return u.storeDustForAddress(addressKey, uint64(newDustAllowanceBalance), newDustOutputCount, mutations)
}

func (u *Manager) applyNewDustWithoutLocking(newOutputs Outputs, newSpents Spents) error {

	allowance := NewDustAllowanceDiff()
	if err := allowance.Add(newOutputs, newSpents); err != nil {
		return err
	}
	return u.applyDustAllowanceDiff(allowance)
}

func (u *Manager) rollbackDustWithoutLocking(newOutputs Outputs, newSpents Spents) error {
	allowance := NewDustAllowanceDiff()
	if err := allowance.Remove(newOutputs, newSpents); err != nil {
		return err
	}
	return u.applyDustAllowanceDiff(allowance)
}

func (u *Manager) storeDustForUnspentOutput(unspentOutput *Output) error {
	allowance := NewDustAllowanceDiff()
	if err := allowance.Add([]*Output{unspentOutput}, []*Spent{}); err != nil {
		return err
	}
	return u.applyDustAllowanceDiff(allowance)
}
