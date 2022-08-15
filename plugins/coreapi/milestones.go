package coreapi

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/restapi"
	"github.com/iotaledger/inx-app/httpserver"
	iotago "github.com/iotaledger/iota.go/v3"
)

func storageMilestoneByIndex(c echo.Context) (*storage.Milestone, error) {

	msIndex, err := httpserver.ParseMilestoneIndexParam(c, restapi.ParameterMilestoneIndex)
	if err != nil {
		return nil, err
	}

	cachedMilestone := deps.Storage.CachedMilestoneByIndexOrNil(msIndex) // milestone +1
	if cachedMilestone == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "milestone index not found: %d", msIndex)
	}
	defer cachedMilestone.Release(true) // milestone -1

	return cachedMilestone.Milestone(), nil
}

func storageMilestoneByID(c echo.Context) (*storage.Milestone, error) {

	milestoneID, err := httpserver.ParseMilestoneIDParam(c, restapi.ParameterMilestoneID)
	if err != nil {
		return nil, err
	}

	cachedMilestone := deps.Storage.CachedMilestoneOrNil(*milestoneID) // milestone +1
	if cachedMilestone == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "milestone not found: %s", iotago.EncodeHex((*milestoneID)[:]))
	}
	defer cachedMilestone.Release(true) // milestone -1

	return cachedMilestone.Milestone(), nil
}

func milestoneByIndex(c echo.Context) (*iotago.Milestone, error) {
	ms, err := storageMilestoneByIndex(c)
	if err != nil {
		return nil, err
	}

	return ms.Milestone(), nil
}

func milestoneByID(c echo.Context) (*iotago.Milestone, error) {
	ms, err := storageMilestoneByID(c)
	if err != nil {
		return nil, err
	}

	return ms.Milestone(), nil
}

func milestoneBytesByIndex(c echo.Context) ([]byte, error) {
	ms, err := storageMilestoneByIndex(c)
	if err != nil {
		return nil, err
	}

	return ms.Data(), nil
}

func milestoneBytesByID(c echo.Context) ([]byte, error) {
	ms, err := storageMilestoneByID(c)
	if err != nil {
		return nil, err
	}

	return ms.Data(), nil
}

func milestoneUTXOChanges(msIndex iotago.MilestoneIndex) (*milestoneUTXOChangesResponse, error) {
	diff, err := deps.UTXOManager.MilestoneDiffWithoutLocking(msIndex)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "can't load milestone diff for index: %d, error: %s", msIndex, err)
		}

		return nil, errors.WithMessagef(echo.ErrInternalServerError, "can't load milestone diff for index: %d, error: %s", msIndex, err)
	}

	createdOutputs := make([]string, len(diff.Outputs))
	consumedOutputs := make([]string, len(diff.Spents))

	for i, output := range diff.Outputs {
		createdOutputs[i] = output.OutputID().ToHex()
	}

	for i, output := range diff.Spents {
		consumedOutputs[i] = output.OutputID().ToHex()
	}

	return &milestoneUTXOChangesResponse{
		Index:           msIndex,
		CreatedOutputs:  createdOutputs,
		ConsumedOutputs: consumedOutputs,
	}, nil
}

func milestoneUTXOChangesByIndex(c echo.Context) (*milestoneUTXOChangesResponse, error) {
	msIndex, err := httpserver.ParseMilestoneIndexParam(c, restapi.ParameterMilestoneIndex)
	if err != nil {
		return nil, err
	}

	return milestoneUTXOChanges(msIndex)
}

func milestoneUTXOChangesByID(c echo.Context) (*milestoneUTXOChangesResponse, error) {
	ms, err := storageMilestoneByID(c)
	if err != nil {
		return nil, err
	}

	return milestoneUTXOChanges(ms.Index())
}
