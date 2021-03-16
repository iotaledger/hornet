package v1

import (
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/restapi"
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

	// confirmed milestone index
	confirmedMilestoneIndex := deps.Storage.GetConfirmedMilestoneIndex()

	// confirmed milestone message ID
	cachedConfirmedMilestone := deps.Storage.GetCachedMilestoneOrNil(confirmedMilestoneIndex)
	if cachedConfirmedMilestone != nil {
		cachedConfirmedMilestone.Release(true)
	}

	// pruning index
	var pruningIndex milestone.Index
	snapshotInfo := deps.Storage.GetSnapshotInfo()
	if snapshotInfo != nil {
		pruningIndex = snapshotInfo.PruningIndex
	}

	return &infoResponse{
		Name:                    deps.AppInfo.Name,
		Version:                 deps.AppInfo.Version,
		IsHealthy:               deps.Tangle.IsNodeHealthy(),
		NetworkID:               deps.NodeConfig.String(protocfg.CfgProtocolNetworkIDName),
		Bech32HRP:               string(deps.Bech32HRP),
		MinPowScore:             deps.NodeConfig.Float64(protocfg.CfgProtocolMinPoWScore),
		LatestMilestoneIndex:    latestMilestoneIndex,
		ConfirmedMilestoneIndex: confirmedMilestoneIndex,
		PruningIndex:            pruningIndex,
		Features:                features,
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

	return &tipsResponse{Tips: tips.ToHex()}, nil
}
