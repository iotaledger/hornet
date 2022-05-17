package testsuite

import (
	"github.com/gohornet/hornet/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
)

func ouputOwnerAddress(output *utxo.Output) iotago.Address {
	switch o := output.Output().(type) {
	case *iotago.AliasOutput:
		return o.UnlockConditionsSet().GovernorAddress().Address
	case *iotago.FoundryOutput:
		return o.UnlockConditionsSet().ImmutableAlias().Address
	default:
		return o.UnlockConditionsSet().Address().Address
	}
}

func outputHasSpendingConstraint(output *utxo.Output) bool {
	conditions := output.Output().UnlockConditionsSet()
	return conditions.HasStorageDepositReturnCondition() || conditions.HasExpirationCondition() || conditions.HasTimelockCondition()
}

func (te *TestEnvironment) UnspentAddressOutputsWithoutConstraints(address iotago.Address, options ...utxo.UTXOIterateOption) (utxo.Outputs, error) {
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

func (te *TestEnvironment) ComputeAddressBalanceWithoutConstraints(address iotago.Address, options ...utxo.UTXOIterateOption) (balance uint64, count int, err error) {
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
