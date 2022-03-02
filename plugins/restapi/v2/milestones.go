package v2

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/restapi"

	"github.com/iotaledger/hive.go/kvstore"
)

func milestoneByIndex(c echo.Context) (*milestoneResponse, error) {

	msIndex, err := restapi.ParseMilestoneIndexParam(c, restapi.ParameterMilestoneIndex)
	if err != nil {
		return nil, err
	}

	cachedMilestone := deps.Storage.CachedMilestoneOrNil(msIndex) // milestone +1
	if cachedMilestone == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "milestone not found: %d", msIndex)
	}
	defer cachedMilestone.Release(true) // milestone -1

	return &milestoneResponse{
		Index:     uint32(cachedMilestone.Milestone().Index),
		MessageID: cachedMilestone.Milestone().MessageID.ToHex(),
		Time:      uint32(cachedMilestone.Milestone().Timestamp.Unix()),
	}, nil
}

func milestoneUTXOChangesByIndex(c echo.Context) (*milestoneUTXOChangesResponse, error) {

	msIndex, err := restapi.ParseMilestoneIndexParam(c, restapi.ParameterMilestoneIndex)
	if err != nil {
		return nil, err
	}

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
		Index:           uint32(msIndex),
		CreatedOutputs:  createdOutputs,
		ConsumedOutputs: consumedOutputs,
	}, nil
}
