package v1

import (
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/restapi"
)

func ParseMilestoneIndexParam(c echo.Context) (milestone.Index, error) {
	milestoneIndex := strings.ToLower(c.Param(ParameterMilestoneIndex))
	if milestoneIndex == "" {
		return 0, errors.WithMessagef(restapi.ErrInvalidParameter, "parameter \"%s\" not specified", ParameterMilestoneIndex)
	}

	msIndex, err := strconv.ParseUint(milestoneIndex, 10, 64)
	if err != nil {
		return 0, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid milestone index: %s, error: %s", milestoneIndex, err)
	}

	return milestone.Index(msIndex), nil
}

func milestoneByIndex(c echo.Context) (*milestoneResponse, error) {

	msIndex, err := ParseMilestoneIndexParam(c)
	if err != nil {
		return nil, err
	}

	cachedMilestone := deps.Storage.GetCachedMilestoneOrNil(msIndex) // milestone +1
	if cachedMilestone == nil {
		return nil, errors.WithMessagef(restapi.ErrNotFound, "milestone not found: %d", msIndex)
	}
	defer cachedMilestone.Release(true)

	return &milestoneResponse{
		Index:     uint32(cachedMilestone.GetMilestone().Index),
		MessageID: cachedMilestone.GetMilestone().MessageID.ToHex(),
		Time:      cachedMilestone.GetMilestone().Timestamp.Unix(),
	}, nil
}

func milestoneUTXOChangesByIndex(c echo.Context) (*milestoneUTXOChangesResponse, error) {

	msIndex, err := ParseMilestoneIndexParam(c)
	if err != nil {
		return nil, err
	}

	diff, err := deps.UTXO.GetMilestoneDiffWithoutLocking(msIndex)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "can't load milestone diff for index: %d, error: %s", msIndex, err)
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
