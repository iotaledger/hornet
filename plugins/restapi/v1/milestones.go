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
		MessageID: cachedMilestone.GetMilestone().MessageID.Hex(),
		Time:      cachedMilestone.GetMilestone().Timestamp.Unix(),
	}, nil
}

func milestoneUTXOChangesByIndex(c echo.Context) (*milestoneUTXOChangesResponse, error) {
	milestoneIndex := strings.ToLower(c.Param(ParameterMilestoneIndex))

	msIndex, err := strconv.ParseUint(milestoneIndex, 10, 64)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid milestone index: %s, error: %s", milestoneIndex, err)
	}

	newOutputs, newSpents, err := deps.UTXO.GetMilestoneDiffsWithoutLocking(milestone.Index(msIndex))
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "can't load milestone diff for index: %s, error: %s", milestoneIndex, err)
	}

	createdOutputs := []string{}
	consumedOutputs := []string{}

	for _, output := range newOutputs {
		createdOutputs = append(createdOutputs, output.OutputID().ToHex())
	}

	for _, output := range newSpents {
		consumedOutputs = append(consumedOutputs, output.OutputID().ToHex())
	}

	return &milestoneUTXOChangesResponse{
		Index:           uint32(msIndex),
		CreatedOutputs:  createdOutputs,
		ConsumedOutputs: consumedOutputs,
	}, nil
}
