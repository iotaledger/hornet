package v1

import (
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/core/cli"
	"github.com/gohornet/hornet/core/database"
	tanglecore "github.com/gohornet/hornet/core/tangle"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/restapi/common"
	"github.com/gohornet/hornet/plugins/urts"
)

func info() (*infoResponse, error) {

	// latest milestone index
	latestMilestoneID := hornet.GetNullMessageID().Hex()
	latestMilestoneIndex := database.Tangle().GetLatestMilestoneIndex()

	// latest milestone message ID
	cachedLatestMilestone := database.Tangle().GetCachedMilestoneOrNil(latestMilestoneIndex)
	if cachedLatestMilestone != nil {
		latestMilestoneID = hex.EncodeToString(cachedLatestMilestone.GetMilestone().MilestoneID[:])
		cachedLatestMilestone.Release(true)
	}

	// solid milestone index
	solidMilestoneID := hornet.GetNullMessageID().Hex()
	solidMilestoneIndex := database.Tangle().GetSolidMilestoneIndex()

	// solid milestone message ID
	cachedSolidMilestone := database.Tangle().GetCachedMilestoneOrNil(solidMilestoneIndex)
	if cachedSolidMilestone != nil {
		solidMilestoneID = hex.EncodeToString(cachedSolidMilestone.GetMilestone().MilestoneID[:])
		cachedSolidMilestone.Release(true)
	}

	// pruning index
	var pruningIndex milestone.Index
	snapshotInfo := database.Tangle().GetSnapshotInfo()
	if snapshotInfo != nil {
		pruningIndex = snapshotInfo.PruningIndex
	}

	return &infoResponse{
		Name:                 cli.AppName,
		Version:              cli.AppVersion,
		IsHealthy:            tanglecore.IsNodeHealthy(),
		NetworkID:            snapshotInfo.NetworkID,
		LatestMilestoneID:    latestMilestoneID,
		LatestMilestoneIndex: latestMilestoneIndex,
		SolidMilestoneID:     solidMilestoneID,
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

	cachedMilestone := database.Tangle().GetCachedMilestoneOrNil(milestone.Index(msIndex)) // milestone +1
	if cachedMilestone == nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "milestone not found: %d", msIndex)
	}
	defer cachedMilestone.Release(true)

	return &milestoneResponse{
		Index:       uint32(cachedMilestone.GetMilestone().Index),
		MilestoneID: hex.EncodeToString(cachedMilestone.GetMilestone().MilestoneID[:]),
		Time:        cachedMilestone.GetMilestone().Timestamp.Unix(),
	}, nil

}
