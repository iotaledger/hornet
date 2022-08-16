//nolint:forcetypeassert,varnamelen,revive,exhaustruct,golint,stylecheck // we don't care about these linters in test cases
package storage_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/tpkg"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestProtocolStorage_Get(t *testing.T) {
	protoStorage := storage.NewProtocolStorage(mapdb.NewMapDB())

	protoParams := addRandProtocolUpgrade(t, protoStorage, 0)
	newProtoParams := addRandProtocolUpgrade(t, protoStorage, 5)

	// Get the protocol parameters for a specific milestone
	protoParams_idx_4, err := protoStorage.ProtocolParameters(4)
	require.NoError(t, err)
	require.Equal(t, protoParams, protoParams_idx_4)

	protoParams_idx_5, err := protoStorage.ProtocolParameters(5)
	require.NoError(t, err)
	require.Equal(t, newProtoParams, protoParams_idx_5)

	protoParams_idx_6, err := protoStorage.ProtocolParameters(6)
	require.NoError(t, err)
	require.Equal(t, newProtoParams, protoParams_idx_6)

	// check adding of protocol parameters
	checkProtoParamsMsOptionCount(t, protoStorage, 2)

	_ = addRandProtocolUpgrade(t, protoStorage, 10)
	checkProtoParamsMsOptionCount(t, protoStorage, 3)

	_ = addRandProtocolUpgrade(t, protoStorage, 15)
	checkProtoParamsMsOptionCount(t, protoStorage, 4)
}

func TestProtocolStorage_Pruning(t *testing.T) {
	protoStorage := storage.NewProtocolStorage(mapdb.NewMapDB())

	addRandProtocolUpgrade(t, protoStorage, 0)
	addRandProtocolUpgrade(t, protoStorage, 5)
	addRandProtocolUpgrade(t, protoStorage, 10)
	addRandProtocolUpgrade(t, protoStorage, 15)
	checkProtoParamsMsOptionCount(t, protoStorage, 4)
	checkProtoParamsMsOptionIndexes(t, protoStorage, map[iotago.MilestoneIndex]struct{}{
		0:  {},
		5:  {},
		10: {},
		15: {},
	})

	// check pruning of the protocol storage
	err := protoStorage.PruneProtocolParameterMilestoneOptions(6)
	require.NoError(t, err)
	checkProtoParamsMsOptionCount(t, protoStorage, 3) // if we prune milestone 6, only the one at milestone 0 is deleted
	checkProtoParamsMsOptionIndexes(t, protoStorage, map[iotago.MilestoneIndex]struct{}{
		5:  {},
		10: {},
		15: {},
	})

	err = protoStorage.PruneProtocolParameterMilestoneOptions(10)
	require.NoError(t, err)
	checkProtoParamsMsOptionCount(t, protoStorage, 2)
	checkProtoParamsMsOptionIndexes(t, protoStorage, map[iotago.MilestoneIndex]struct{}{
		10: {},
		15: {},
	})

	err = protoStorage.PruneProtocolParameterMilestoneOptions(100)
	require.NoError(t, err)
	checkProtoParamsMsOptionCount(t, protoStorage, 1) // if we prune a much higher index, only the last valid should remain
	checkProtoParamsMsOptionIndexes(t, protoStorage, map[iotago.MilestoneIndex]struct{}{
		15: {},
	})
}

func TestProtocolStorage_ForEachActiveProtocolParameterMilestoneOption(t *testing.T) {
	protoStorage := storage.NewProtocolStorage(mapdb.NewMapDB())

	addRandProtocolUpgrade(t, protoStorage, 0)
	addRandProtocolUpgrade(t, protoStorage, 5)
	addRandProtocolUpgrade(t, protoStorage, 10)
	addRandProtocolUpgrade(t, protoStorage, 15)
	addRandProtocolUpgrade(t, protoStorage, 50)
	checkProtoParamsMsOptionCount(t, protoStorage, 5)

	allowedTargetIndexes := map[iotago.MilestoneIndex]struct{}{
		10: {},
		15: {},
	}

	err := protoStorage.ForEachActiveProtocolParameterMilestoneOption(11, func(protoParamsMsOption *iotago.ProtocolParamsMilestoneOpt) bool {
		if _, exists := allowedTargetIndexes[protoParamsMsOption.TargetMilestoneIndex]; !exists {
			require.Fail(t, "unexpected target milestone index", protoParamsMsOption.TargetMilestoneIndex)
		}
		delete(allowedTargetIndexes, protoParamsMsOption.TargetMilestoneIndex)

		return true
	})

	require.NoError(t, err)
	require.Equal(t, 0, len(allowedTargetIndexes), "expected target milestone indexes not found: %v", allowedTargetIndexes)
}

func TestProtocolStorage_ActiveProtocolParameterMilestoneOptionsHash(t *testing.T) {
	protoStorage := storage.NewProtocolStorage(mapdb.NewMapDB())

	addRandProtocolUpgrade(t, protoStorage, 0)
	addRandProtocolUpgrade(t, protoStorage, 5)
	addRandProtocolUpgrade(t, protoStorage, 10)
	addRandProtocolUpgrade(t, protoStorage, 15)
	checkProtoParamsMsOptionCount(t, protoStorage, 4)
	checkProtoParamsMsOptionIndexes(t, protoStorage, map[iotago.MilestoneIndex]struct{}{
		0:  {},
		5:  {},
		10: {},
		15: {},
	})

	_, err := protoStorage.ActiveProtocolParameterMilestoneOptionsHash(5)
	require.NoError(t, err)
}

func addRandProtocolUpgrade(t *testing.T, protoStorage *storage.ProtocolStorage, activationIndex iotago.MilestoneIndex) *iotago.ProtocolParameters {
	protoParams := tpkg.RandProtocolParameters()

	protoParamsBytes, err := protoParams.Serialize(serializer.DeSeriModeNoValidation, nil)
	require.NoError(t, err)

	err = protoStorage.StoreProtocolParametersMilestoneOption(&iotago.ProtocolParamsMilestoneOpt{
		TargetMilestoneIndex: activationIndex,
		ProtocolVersion:      tpkg.RandByte(),
		Params:               protoParamsBytes,
	})
	require.NoError(t, err)

	return protoParams
}

func checkProtoParamsMsOptionCount(t *testing.T, protoStorage *storage.ProtocolStorage, expected int) {
	collectedProtoParamsMsOptions := []*iotago.ProtocolParamsMilestoneOpt{}

	// loop over all existing protocol parameters milestone options
	err := protoStorage.ForEachProtocolParameterMilestoneOption(func(protoParamsMsOption *iotago.ProtocolParamsMilestoneOpt) bool {
		collectedProtoParamsMsOptions = append(collectedProtoParamsMsOptions, protoParamsMsOption)

		return true
	})
	require.NoError(t, err)
	require.Equal(t, expected, len(collectedProtoParamsMsOptions))
}

func checkProtoParamsMsOptionIndexes(t *testing.T, protoStorage *storage.ProtocolStorage, allowedTargetIndexes map[iotago.MilestoneIndex]struct{}) {
	// loop over all existing protocol parameters milestone options
	err := protoStorage.ForEachProtocolParameterMilestoneOption(func(protoParamsMsOption *iotago.ProtocolParamsMilestoneOpt) bool {
		if _, exists := allowedTargetIndexes[protoParamsMsOption.TargetMilestoneIndex]; !exists {
			require.Fail(t, "unexpected target milestone index", protoParamsMsOption.TargetMilestoneIndex)
		}
		delete(allowedTargetIndexes, protoParamsMsOption.TargetMilestoneIndex)

		return true
	})
	require.NoError(t, err)
	require.Equal(t, 0, len(allowedTargetIndexes), "expected target milestone indexes not found: %v", allowedTargetIndexes)
}
