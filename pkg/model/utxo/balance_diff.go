package utxo

import (
	iotago "github.com/iotaledger/iota.go/v2"
)

type singleBalanceDiff struct {
	balanceDiff              int64
	dustAllowanceBalanceDiff int64
	dustOutputCountDiff      int64
}

type BalanceDiff struct {
	balances map[string]*singleBalanceDiff
}

func NewBalanceDiff() *BalanceDiff {
	return &BalanceDiff{
		balances: make(map[string]*singleBalanceDiff),
	}
}

func (d *BalanceDiff) addressKeyForAddress(address iotago.Address) (string, error) {
	bytes, err := address.Serialize(iotago.DeSeriModeNoValidation)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (d *BalanceDiff) addressKeyForOutput(output *Output) (string, error) {
	bytes, err := output.Address().Serialize(iotago.DeSeriModeNoValidation)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (d *BalanceDiff) DiffForAddress(address iotago.Address) (balanceDiff int64, dustAllowanceBalance int64, dustOutputCount int64, err error) {

	addressKey, err := d.addressKeyForAddress(address)
	if err != nil {
		return 0, 0, 0, err
	}

	if diff, found := d.balances[addressKey]; found {
		return diff.balanceDiff, diff.dustAllowanceBalanceDiff, diff.dustOutputCountDiff, nil
	}
	return 0, 0, 0, nil
}

func (d *BalanceDiff) singleDiffForOutput(output *Output) (*singleBalanceDiff, error) {

	addressKey, err := d.addressKeyForOutput(output)
	if err != nil {
		return nil, err
	}

	if diff, found := d.balances[addressKey]; found {
		return diff, nil
	}

	diff := &singleBalanceDiff{}
	d.balances[addressKey] = diff
	return diff, nil
}

func (d *BalanceDiff) Add(newOutputs Outputs, newSpents Spents) error {

	for _, output := range newOutputs {

		diff, err := d.singleDiffForOutput(output)
		if err != nil {
			return err
		}

		// Increase balance
		diff.balanceDiff += int64(output.Amount())

		switch output.outputType {
		case iotago.OutputSigLockedDustAllowanceOutput:

			// Add new dust allowance
			diff.dustAllowanceBalanceDiff += int64(output.Amount())

		case iotago.OutputSigLockedSingleOutput:

			if output.Amount() < iotago.OutputSigLockedDustAllowanceOutputMinDeposit {
				// Add new dust
				diff.dustOutputCountDiff += 1
			}
		}
	}

	for _, spent := range newSpents {

		output := spent.Output()

		diff, err := d.singleDiffForOutput(output)
		if err != nil {
			return err
		}

		// Decrease balance
		diff.balanceDiff -= int64(output.Amount())

		switch output.outputType {
		case iotago.OutputSigLockedDustAllowanceOutput:

			// Remove spent dust allowance
			diff.dustAllowanceBalanceDiff -= int64(output.Amount())

		case iotago.OutputSigLockedSingleOutput:

			if output.Amount() < iotago.OutputSigLockedDustAllowanceOutputMinDeposit {
				// Remove spent dust
				diff.dustOutputCountDiff -= 1
			}
		}
	}

	return nil
}

func (d *BalanceDiff) Remove(newOutputs Outputs, newSpents Spents) error {

	// we have to delete the newOutputs
	for _, output := range newOutputs {

		diff, err := d.singleDiffForOutput(output)
		if err != nil {
			return err
		}

		// Remove unspent balance
		diff.balanceDiff -= int64(output.Amount())

		switch output.outputType {
		case iotago.OutputSigLockedDustAllowanceOutput:

			// Remove unspent dust allowance
			diff.dustAllowanceBalanceDiff -= int64(output.Amount())

		case iotago.OutputSigLockedSingleOutput:

			if output.Amount() < iotago.OutputSigLockedDustAllowanceOutputMinDeposit {
				// Remove unspent dust
				diff.dustOutputCountDiff -= 1
			}
		}
	}

	// we have to re-add the spents as output and mark them as unspent
	for _, spent := range newSpents {

		output := spent.Output()

		diff, err := d.singleDiffForOutput(output)
		if err != nil {
			return err
		}

		// Re-add previously-spent balance
		diff.balanceDiff += int64(output.Amount())

		switch output.outputType {
		case iotago.OutputSigLockedDustAllowanceOutput:

			// Re-Add previously-spent dust allowance
			diff.dustAllowanceBalanceDiff += int64(output.Amount())

		case iotago.OutputSigLockedSingleOutput:

			if output.Amount() < iotago.OutputSigLockedDustAllowanceOutputMinDeposit {
				// Re-Add previously-spent dust
				diff.dustOutputCountDiff += 1
			}
		}
	}
	return nil
}
