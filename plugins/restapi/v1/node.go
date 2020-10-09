package v1

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"strconv"
	"strings"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/cli"
	"github.com/gohornet/hornet/plugins/restapi/common"
	tangleplugin "github.com/gohornet/hornet/plugins/tangle"
	"github.com/gohornet/hornet/plugins/urts"
)

func info() (*infoResponse, error) {

	// latest milestone index
	latestMilestoneMessageID := hornet.GetNullMessageID().Hex()
	latestMilestoneIndex := tangle.GetLatestMilestoneIndex()

	// latest milestone message ID
	cachedLatestMilestoneMsg := tangle.GetMilestoneCachedMessageOrNil(latestMilestoneIndex)
	if cachedLatestMilestoneMsg != nil {
		latestMilestoneMessageID = cachedLatestMilestoneMsg.GetMessage().GetMessageID().Hex()
		cachedLatestMilestoneMsg.Release(true)
	}

	// solid milestone index
	solidMilestoneMessageID := hornet.GetNullMessageID().Hex()
	solidMilestoneIndex := tangle.GetSolidMilestoneIndex()

	// solid milestone message ID
	cachedSolidMilestoneMsg := tangle.GetMilestoneCachedMessageOrNil(solidMilestoneIndex)
	if cachedSolidMilestoneMsg != nil {
		solidMilestoneMessageID = cachedSolidMilestoneMsg.GetMessage().GetMessageID().Hex()
		cachedSolidMilestoneMsg.Release(true)
	}

	// pruning index
	var pruningIndex milestone.Index
	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		pruningIndex = snapshotInfo.PruningIndex
	}

	return &infoResponse{
		Name:                          cli.AppName,
		Version:                       cli.AppVersion,
		IsHealthy:                     tangleplugin.IsNodeHealthy(),
		CoordinatorPublicKey:          config.NodeConfig.GetString(config.CfgCoordinatorPublicKey),
		LatestMilestoneMessageID:      latestMilestoneMessageID,
		LatestMilestoneIndex:          latestMilestoneIndex,
		LatestSolidMilestoneMessageID: solidMilestoneMessageID,
		LatestSolidMilestoneIndex:     solidMilestoneIndex,
		PruningIndex:                  pruningIndex,
		Features:                      features,
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
		tips, err = urts.TipSelector.SelectNonLazyTips()
	} else {
		_, tips, err = urts.TipSelector.SelectSpammerTips()
	}

	if err != nil {
		if err == tangle.ErrNodeNotSynced || err == tipselect.ErrNoTipsAvailable {
			return nil, errors.WithMessage(common.ErrServiceUnavailable, err.Error())
		}
		return nil, errors.WithMessage(common.ErrInternalError, err.Error())
	}

	return &tipsResponse{Tip1: tips[0].Hex(), Tip2: tips[1].Hex()}, nil
}

func milestoneByIndex(c echo.Context) (*milestoneResponse, error) {
	milestoneIndex := strings.ToLower(c.Param(ParameterMilestoneIndex))

	msIndex, err := strconv.ParseUint(milestoneIndex, 10, 64)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid milestone index: %s, error: %w", milestoneIndex, err)
	}

	cachedMilestone := tangle.GetCachedMilestoneOrNil(milestone.Index(msIndex)) // milestone +1
	if cachedMilestone == nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "milestone not found: %d", msIndex)
	}
	defer cachedMilestone.Release(true)

	return &milestoneResponse{
		Index:     uint32(cachedMilestone.GetMilestone().Index),
		MessageID: cachedMilestone.GetMilestone().MessageID.Hex(),
		Time:      cachedMilestone.GetMilestone().Timestamp.Unix(),
	}, nil

}
