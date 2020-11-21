package v1

import (
	"strconv"
	"strings"

	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/tipselect"
)

func info() (*infoResponse, error) {

	// latest milestone index
	latestMilestoneIndex := deps.Storage.GetLatestMilestoneIndex()

	// latest milestone message ID
	cachedLatestMilestone := deps.Storage.GetCachedMilestoneOrNil(latestMilestoneIndex)
	if cachedLatestMilestone != nil {
		cachedLatestMilestone.Release(true)
	}

	// solid milestone index
	solidMilestoneIndex := deps.Storage.GetSolidMilestoneIndex()

	// solid milestone message ID
	cachedSolidMilestone := deps.Storage.GetCachedMilestoneOrNil(solidMilestoneIndex)
	if cachedSolidMilestone != nil {
		cachedSolidMilestone.Release(true)
	}

	// pruning index
	var pruningIndex milestone.Index
	snapshotInfo := deps.Storage.GetSnapshotInfo()
	if snapshotInfo != nil {
		pruningIndex = snapshotInfo.PruningIndex
	}

	return &infoResponse{
		Name:                 deps.AppInfo.Name,
		Version:              deps.AppInfo.Version,
		IsHealthy:            deps.Tangle.IsNodeHealthy(),
		NetworkID:            deps.NodeConfig.String(protocfg.CfgProtocolNetworkIDName),
		LatestMilestoneIndex: latestMilestoneIndex,
		SolidMilestoneIndex:  solidMilestoneIndex,
		PruningIndex:         pruningIndex,
		Features:             features,
	}, nil
}

func tips(c echo.Context) (*tipsResponse, error) {
	spammerTips := false
	for query := range c.QueryParams() {
		if strings.ToLower(query) == "spammertips" {
			spammerTips = true
			break
		}
	}

	var tips hornet.MessageIDs
	var err error

	if !spammerTips {
		tips, err = deps.TipSelector.SelectNonLazyTips()
	} else {
		_, tips, err = deps.TipSelector.SelectSpammerTips()
	}

	if err != nil {
		if err == common.ErrNodeNotSynced || err == tipselect.ErrNoTipsAvailable {
			return nil, errors.WithMessage(restapi.ErrServiceUnavailable, err.Error())
		}
		return nil, errors.WithMessage(restapi.ErrInternalError, err.Error())
	}

	return &tipsResponse{Tip1: tips[0].Hex(), Tip2: tips[1].Hex()}, nil
}

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
