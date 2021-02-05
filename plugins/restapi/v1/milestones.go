package v1

import (
	"strconv"
	"strings"

	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

func milestoneByIndex(c echo.Context) (*milestoneResponse, error) {
	milestoneIndex := strings.ToLower(c.Param(ParameterMilestoneIndex))

	msIndex, err := strconv.ParseUint(milestoneIndex, 10, 64)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid milestone index: %s, error: %s", milestoneIndex, err)
	}

	cachedMilestone := deps.Storage.GetCachedMilestoneOrNil(milestone.Index(msIndex)) // milestone +1
	if cachedMilestone == nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "milestone not found: %d", msIndex)
	}
	defer cachedMilestone.Release(true)

	return &milestoneResponse{
		Index:     uint32(cachedMilestone.GetMilestone().Index),
		MessageID: cachedMilestone.GetMilestone().MessageID.ToHex(),
		Time:      cachedMilestone.GetMilestone().Timestamp.Unix(),
	}, nil
}

func milestoneUTXOChangesByIndex(c echo.Context) (*milestoneUTXOChangesResponse, error) {
	milestoneIndex := strings.ToLower(c.Param(ParameterMilestoneIndex))

	msIndex, err := strconv.ParseUint(milestoneIndex, 10, 64)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid milestone index: %s, error: %s", milestoneIndex, err)
	}

	diff, err := deps.UTXO.GetMilestoneDiffWithoutLocking(milestone.Index(msIndex))
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "can't load milestone diff for index: %s, error: %s", milestoneIndex, err)
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
