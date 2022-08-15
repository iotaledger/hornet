package testsuite

import (
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
)

func ouputOwnerAddress(output *utxo.Output) iotago.Address {
	switch o := output.Output().(type) {
	case *iotago.AliasOutput:
		return o.UnlockConditionSet().GovernorAddress().Address
	case *iotago.FoundryOutput:
		return o.UnlockConditionSet().ImmutableAlias().Address
	default:
		return o.UnlockConditionSet().Address().Address
	}
}

func outputHasSpendingConstraint(output *utxo.Output) bool {
	conditions := output.Output().UnlockConditionSet()

	return conditions.HasStorageDepositReturnCondition() || conditions.HasExpirationCondition() || conditions.HasTimelockCondition()
}

func (te *TestEnvironment) UnspentAddressOutputsWithoutConstraints(address iotago.Address, options ...utxo.IterateOption) (utxo.Outputs, error) {
	outputs := utxo.Outputs{}
	consumerFunc := func(output *utxo.Output) bool {
		ownerAddress := ouputOwnerAddress(output)
		if ownerAddress != nil && address.Equal(ownerAddress) && !outputHasSpendingConstraint(output) {
			outputs = append(outputs, output)
		}

		return true
	}
	if err := te.UTXOManager().ForEachUnspentOutput(consumerFunc, options...); err != nil {
		return nil, err
	}

	return outputs, nil
}

func (te *TestEnvironment) ComputeAddressBalanceWithoutConstraints(address iotago.Address, options ...utxo.IterateOption) (balance uint64, count int, err error) {
	balance = 0
	count = 0

	consumerFunc := func(output *utxo.Output) bool {
		ownerAddress := ouputOwnerAddress(output)
		if ownerAddress != nil && address.Equal(ownerAddress) && !outputHasSpendingConstraint(output) {
			count++
			balance += output.Deposit()
		}

		return true
	}
	if err := te.UTXOManager().ForEachUnspentOutput(consumerFunc, options...); err != nil {
		return 0, 0, err
	}

	return balance, count, err
}
