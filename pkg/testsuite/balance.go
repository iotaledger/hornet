package testsuite

import (
	"github.com/gohornet/hornet/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
)

func ouputOwnerAddress(output *utxo.Output) iotago.Address {
	switch o := output.Output().(type) {
	case *iotago.AliasOutput:
		return o.UnlockConditions().MustSet().GovernorAddress().Address
	case *iotago.FoundryOutput:
		return o.UnlockConditions().MustSet().ImmutableAlias().Address
	default:
		return o.UnlockConditions().MustSet().Address().Address
	}
}

func outputHasSpendingConstraint(output *utxo.Output) bool {
	conditions := output.Output().UnlockConditions().MustSet()
	return conditions.HasDustDepositReturnCondition() || conditions.HasExpirationCondition() || conditions.HasTimelockCondition()
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
