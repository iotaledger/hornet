package storage_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/tpkg"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestProtocolStorage(t *testing.T) {
	protoStorage := storage.NewProtocolStorage(mapdb.NewMapDB())

	addRandProtocolUpgrade := func(activationIndex iotago.MilestoneIndex) *iotago.ProtocolParameters {
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

	protoParams := addRandProtocolUpgrade(0)
	newProtoParams := addRandProtocolUpgrade(5)

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
	checkProtoParamsMsOptionCount := func(expected int) {
		collectedProtoParamsMsOptions := []*iotago.ProtocolParamsMilestoneOpt{}

		// loop over all existing protocol parameter milestone options
		err = protoStorage.ForEachProtocolParameterMilestoneOption(func(protoParamsMsOption *iotago.ProtocolParamsMilestoneOpt) bool {
			collectedProtoParamsMsOptions = append(collectedProtoParamsMsOptions, protoParamsMsOption)
			return true
		})
		require.NoError(t, err)
		require.Equal(t, expected, len(collectedProtoParamsMsOptions))
	}
	checkProtoParamsMsOptionCount(2)

	_ = addRandProtocolUpgrade(10)
	checkProtoParamsMsOptionCount(3)

	_ = addRandProtocolUpgrade(15)
	checkProtoParamsMsOptionCount(4)

	// check pruning of the protocol storage
	err = protoStorage.PruneProtocolParameterMilestoneOptions(6)
	require.NoError(t, err)
	checkProtoParamsMsOptionCount(3) // if we prune milestone 6, only the one at milestone 0 is deleted

	err = protoStorage.PruneProtocolParameterMilestoneOptions(10)
	require.NoError(t, err)
	checkProtoParamsMsOptionCount(2)

	err = protoStorage.PruneProtocolParameterMilestoneOptions(100)
	require.NoError(t, err)
	checkProtoParamsMsOptionCount(1) // if we prune a much higher index, only the last valid should remain
}
