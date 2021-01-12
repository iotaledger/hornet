package utxo

import (
	"encoding/hex"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go"
)

var (
	// ErrInvalidDustForAddress is returned when the dust for an address is invalid.
	ErrInvalidDustForAddress = errors.New("invalid dust for address")
)

func (u *Manager) ReadDustForAddress(address iotago.Address) (dustAllowanceBalance uint64, dustOutputCount int64, err error) {

	return u.readDustForAddress([]byte(address.String()))
}

func (u *Manager) readDustForAddress(addressBytes []byte) (dustAllowanceBalance uint64, dustOutputCount int64, err error) {

	value, err := u.dustStorage.Get(addressBytes)
	if err != nil {
		// No error should ever happen here
		return 0, 0, err
	}

	if value != nil {
		return dustFromBytes(value)
	}

	return 0, 0, nil
}

func (u *Manager) storeDustForAddress(addressBytes []byte, dustAllowanceBalance uint64, dustOutputCount int64, mutations kvstore.BatchedMutations) error {

	if dustOutputCount == 0 && dustAllowanceBalance != 0 {
		// Balance cannot be zero if there are no outputs
		return fmt.Errorf("%w: %s dustAllowanceBalance %d, dustOutputCount %d", ErrInvalidDustForAddress, hex.EncodeToString(addressBytes), dustAllowanceBalance, dustOutputCount)
	}

	if dustAllowanceBalance == 0 {
		// Remove from database
		return mutations.Delete(addressBytes)
	} else {
		return mutations.Set(addressBytes, bytesFromDust(dustAllowanceBalance, dustOutputCount))
	}

	return nil
}

func (u *Manager) applyDustDiff(dustDiff map[iotago.Address]*DustDiff) error {

	mutations := u.dustStorage.Batched()
	for addr, diff := range dustDiff {
		if err := u.applyDustDiffForAddress([]byte(addr.String()), diff.DustAllowanceBalanceDiff, diff.DustOutputCount, mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}
	return mutations.Commit()
}

func (u *Manager) applyDustDiffForAddress(addressBytes []byte, dustAllowanceBalanceDiff int64, dustOutputCountDiff int64, mutations kvstore.BatchedMutations) error {

	dustAllowanceBalance, dustOutputCount, err := u.readDustForAddress(addressBytes)
	if err != nil {
		return err
	}

	newDustAllowanceBalance := int64(dustAllowanceBalance) + dustAllowanceBalanceDiff
	newDustOutputCount := dustOutputCount + dustOutputCountDiff

	if newDustOutputCount < 0 || newDustAllowanceBalance < 0 {
		// Count or balance cannot be negative
		return fmt.Errorf("%w: %s dustAllowanceBalance %d, dustOutputCount %d", ErrInvalidDustForAddress, hex.EncodeToString(addressBytes), dustAllowanceBalance, dustOutputCount)
	}

	return u.storeDustForAddress(addressBytes, uint64(newDustAllowanceBalance), newDustOutputCount, mutations)
}
