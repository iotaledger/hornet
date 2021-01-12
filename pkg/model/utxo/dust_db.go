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

func (u *Manager) applyDustDiff(dustDiff map[string]*DustDiff) error {

	mutations := u.dustStorage.Batched()
	for addr, diff := range dustDiff {
		if err := u.applyDustDiffForAddress(addr, diff.DustAllowanceBalanceDiff, diff.DustOutputCount, mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}
	return mutations.Commit()
}

func (u *Manager) applyDustDiffForAddress(addressString string, dustAllowanceBalanceDiff int64, dustOutputCountDiff int64, mutations kvstore.BatchedMutations) error {

	addressBytes := []byte(addressString)

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

func (u *Manager) applyNewDustWithoutLocking(newOutputs Outputs, newSpents Spents) error {
	dustDiff := make(map[string]*DustDiff)

	for _, output := range newOutputs {
		// Add new dust
		switch output.outputType {
		case iotago.OutputSigLockedDustAllowanceOutput:
			address := output.Address().String()
			diff, found := dustDiff[address]
			if found {
				diff.DustAllowanceBalanceDiff += int64(output.Amount())
			} else {
				dustDiff[address] = NewDustDiff(int64(output.Amount()), 0)
			}
		case iotago.OutputSigLockedSingleOutput:
			if output.Amount() < iotago.OutputSigLockedDustAllowanceOutputMinDeposit {
				address := output.Address().String()
				diff, found := dustDiff[address]
				if found {
					diff.DustOutputCount += 1
				} else {
					dustDiff[address] = NewDustDiff(0, 1)
				}
			}
		}
	}

	for _, spent := range newSpents {

		// Remove spent dust
		output := spent.Output()
		switch output.outputType {
		case iotago.OutputSigLockedDustAllowanceOutput:
			address := output.Address().String()
			diff, found := dustDiff[address]
			if found {
				diff.DustAllowanceBalanceDiff -= int64(output.Amount())
			} else {
				dustDiff[address] = NewDustDiff(-int64(output.Amount()), 0)
			}
		case iotago.OutputSigLockedSingleOutput:
			if output.Amount() < iotago.OutputSigLockedDustAllowanceOutputMinDeposit {
				address := output.Address().String()
				diff, found := dustDiff[address]
				if found {
					diff.DustOutputCount -= 1
				} else {
					dustDiff[address] = NewDustDiff(0, -1)
				}
			}
		}
	}

	return u.applyDustDiff(dustDiff)
}

func (u *Manager) rollbackDustWithoutLocking(newOutputs Outputs, newSpents Spents) error {

	dustDiff := make(map[string]*DustDiff)

	// we have to delete the newOutputs of this milestone
	for _, output := range newOutputs {

		// Remove unspent dust
		switch output.outputType {
		case iotago.OutputSigLockedDustAllowanceOutput:
			address := output.Address().String()
			diff, found := dustDiff[address]
			if found {
				diff.DustAllowanceBalanceDiff -= int64(output.Amount())
			} else {
				dustDiff[address] = NewDustDiff(-int64(output.Amount()), 0)
			}
		case iotago.OutputSigLockedSingleOutput:
			if output.Amount() < iotago.OutputSigLockedDustAllowanceOutputMinDeposit {
				address := output.Address().String()
				diff, found := dustDiff[address]
				if found {
					diff.DustOutputCount -= 1
				} else {
					dustDiff[address] = NewDustDiff(0, -1)
				}
			}
		}
	}

	// we have to store the spents as output and mark them as unspent
	for _, spent := range newSpents {

		// Re-Add previously-spent dust
		output := spent.Output()
		switch output.outputType {
		case iotago.OutputSigLockedDustAllowanceOutput:
			address := output.Address().String()
			diff, found := dustDiff[address]
			if found {
				diff.DustAllowanceBalanceDiff += int64(output.Amount())
			} else {
				dustDiff[address] = NewDustDiff(int64(output.Amount()), 0)
			}
		case iotago.OutputSigLockedSingleOutput:
			if output.Amount() < iotago.OutputSigLockedDustAllowanceOutputMinDeposit {
				address := output.Address().String()
				diff, found := dustDiff[address]
				if found {
					diff.DustOutputCount += 1
				} else {
					dustDiff[address] = NewDustDiff(0, 1)
				}
			}
		}
	}

	return u.applyDustDiff(dustDiff)
}

func (u *Manager) storeDustForUnspentOutput(unspentOutput *Output) error {

	dustMutations := u.dustStorage.Batched()

	switch unspentOutput.outputType {
	case iotago.OutputSigLockedDustAllowanceOutput:
		if err := u.applyDustDiffForAddress(unspentOutput.Address().String(), int64(unspentOutput.Amount()), 0, dustMutations); err != nil {
			dustMutations.Cancel()
			return nil
		}
	case iotago.OutputSigLockedSingleOutput:
		if unspentOutput.Amount() < iotago.OutputSigLockedDustAllowanceOutputMinDeposit {
			if err := u.applyDustDiffForAddress(unspentOutput.Address().String(), 0, 1, dustMutations); err != nil {
				dustMutations.Cancel()
				return nil
			}
		}
	}

	return dustMutations.Commit()
}
