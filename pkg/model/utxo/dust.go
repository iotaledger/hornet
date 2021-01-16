package utxo

import iotago "github.com/iotaledger/iota.go"

type dustDiff struct {
	allowanceBalanceDiff int64
	outputCount          int64
}

func newDustDiff(dustAllowanceBalance int64, dustOutputCount int64) *dustDiff {
	return &dustDiff{
		allowanceBalanceDiff: dustAllowanceBalance,
		outputCount:          dustOutputCount,
	}
}

type DustAllowanceDiff struct {
	allowance map[string]*dustDiff
}

func NewDustAllowanceDiff() *DustAllowanceDiff {
	return &DustAllowanceDiff{
		allowance: make(map[string]*dustDiff),
	}
}

func (d *DustAllowanceDiff) addressKeyForAddress(address iotago.Address) (string, error) {
	bytes, err := address.Serialize(iotago.DeSeriModeNoValidation)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (d *DustAllowanceDiff) addressKeyForOutput(output *Output) (string, error) {
	bytes, err := output.Address().Serialize(iotago.DeSeriModeNoValidation)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (d *DustAllowanceDiff) DiffForAddress(address iotago.Address) (dustAllowanceBalance int64, dustOutputCount int64, err error) {

	addressKey, err := d.addressKeyForAddress(address)
	if err != nil {
		return 0, 0, err
	}

	if diff, found := d.allowance[addressKey]; found {
		return diff.allowanceBalanceDiff, diff.outputCount, nil
	}
	return 0, 0, nil
}

func (d *DustAllowanceDiff) Add(newOutputs Outputs, newSpents Spents) error {

	for _, output := range newOutputs {

		switch output.outputType {
		case iotago.OutputSigLockedDustAllowanceOutput:
			// Add new dust allowance
			addressKey, err := d.addressKeyForOutput(output)
			if err != nil {
				return err
			}
			if diff, found := d.allowance[addressKey]; found {
				diff.allowanceBalanceDiff += int64(output.Amount())
			} else {
				d.allowance[addressKey] = newDustDiff(int64(output.Amount()), 0)
			}
		case iotago.OutputSigLockedSingleOutput:
			if output.Amount() < iotago.OutputSigLockedDustAllowanceOutputMinDeposit {
				// Add new dust
				addressKey, err := d.addressKeyForOutput(output)
				if err != nil {
					return err
				}
				if diff, found := d.allowance[addressKey]; found {
					diff.outputCount += 1
				} else {
					d.allowance[addressKey] = newDustDiff(0, 1)
				}
			}
		}
	}

	for _, spent := range newSpents {

		output := spent.Output()
		switch output.outputType {
		case iotago.OutputSigLockedDustAllowanceOutput:
			// Remove spent dust allowance
			addressKey, err := d.addressKeyForOutput(output)
			if err != nil {
				return err
			}
			if diff, found := d.allowance[addressKey]; found {
				diff.allowanceBalanceDiff -= int64(output.Amount())
			} else {
				d.allowance[addressKey] = newDustDiff(-int64(output.Amount()), 0)
			}
		case iotago.OutputSigLockedSingleOutput:
			if output.Amount() < iotago.OutputSigLockedDustAllowanceOutputMinDeposit {
				// Remove spent dust
				addressKey, err := d.addressKeyForOutput(output)
				if err != nil {
					return err
				}
				if diff, found := d.allowance[addressKey]; found {
					diff.outputCount -= 1
				} else {
					d.allowance[addressKey] = newDustDiff(0, -1)
				}
			}
		}
	}
	return nil
}

func (d *DustAllowanceDiff) Remove(newOutputs Outputs, newSpents Spents) error {

	// we have to delete the newOutputs
	for _, output := range newOutputs {

		switch output.outputType {
		case iotago.OutputSigLockedDustAllowanceOutput:
			// Remove unspent dust allowance
			addressKey, err := d.addressKeyForOutput(output)
			if err != nil {
				return err
			}
			if diff, found := d.allowance[addressKey]; found {
				diff.allowanceBalanceDiff -= int64(output.Amount())
			} else {
				d.allowance[addressKey] = newDustDiff(-int64(output.Amount()), 0)
			}
		case iotago.OutputSigLockedSingleOutput:
			if output.Amount() < iotago.OutputSigLockedDustAllowanceOutputMinDeposit {
				// Remove unspent dust
				addressKey, err := d.addressKeyForOutput(output)
				if err != nil {
					return err
				}
				if diff, found := d.allowance[addressKey]; found {
					diff.outputCount -= 1
				} else {
					d.allowance[addressKey] = newDustDiff(0, -1)
				}
			}
		}
	}

	// we have to re-add the spents as output and mark them as unspent
	for _, spent := range newSpents {

		output := spent.Output()
		switch output.outputType {
		case iotago.OutputSigLockedDustAllowanceOutput:
			// Re-Add previously-spent dust allowance
			addressKey, err := d.addressKeyForOutput(output)
			if err != nil {
				return err
			}
			if diff, found := d.allowance[addressKey]; found {
				diff.allowanceBalanceDiff += int64(output.Amount())
			} else {
				d.allowance[addressKey] = newDustDiff(int64(output.Amount()), 0)
			}
		case iotago.OutputSigLockedSingleOutput:
			if output.Amount() < iotago.OutputSigLockedDustAllowanceOutputMinDeposit {
				// Re-Add previously-spent dust
				addressKey, err := d.addressKeyForOutput(output)
				if err != nil {
					return err
				}
				if diff, found := d.allowance[addressKey]; found {
					diff.outputCount += 1
				} else {
					d.allowance[addressKey] = newDustDiff(0, 1)
				}
			}
		}
	}
	return nil
}
