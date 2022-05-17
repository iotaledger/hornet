package v2

import (
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/tipselect"
)

//nolint:unparam // even if the error is never used, the structure of all routes should be the same
func info() (*infoResponse, error) {

	var messagesPerSecond, referencedMessagesPerSecond, referencedRate float64
	lastConfirmedMilestoneMetric := deps.Tangle.LastConfirmedMilestoneMetric()
	if lastConfirmedMilestoneMetric != nil {
		messagesPerSecond = lastConfirmedMilestoneMetric.MPS
		referencedMessagesPerSecond = lastConfirmedMilestoneMetric.RMPS
		referencedRate = lastConfirmedMilestoneMetric.ReferencedRate
	}

	// latest milestone
	var latestMilestoneIndex = deps.SyncManager.LatestMilestoneIndex()
	var latestMilestoneTimestamp uint32 = 0
	var latestMilestoneIDHex string
	cachedMilestoneLatest := deps.Storage.CachedMilestoneByIndexOrNil(latestMilestoneIndex) // milestone +1
	if cachedMilestoneLatest != nil {
		latestMilestoneTimestamp = cachedMilestoneLatest.Milestone().TimestampUnix()
		latestMilestoneIDHex = cachedMilestoneLatest.Milestone().MilestoneIDHex()
		cachedMilestoneLatest.Release(true) // milestone -1
	}

	// confirmed milestone index
	var confirmedMilestoneIndex = deps.SyncManager.ConfirmedMilestoneIndex()
	var confirmedMilestoneTimestamp uint32 = 0
	var confirmedMilestoneIDHex string
	cachedMilestoneConfirmed := deps.Storage.CachedMilestoneByIndexOrNil(confirmedMilestoneIndex) // milestone +1
	if cachedMilestoneConfirmed != nil {
		confirmedMilestoneTimestamp = cachedMilestoneConfirmed.Milestone().TimestampUnix()
		confirmedMilestoneIDHex = cachedMilestoneConfirmed.Milestone().MilestoneIDHex()
		cachedMilestoneConfirmed.Release(true) // milestone -1
	}

	// pruning index
	var pruningIndex milestone.Index
	snapshotInfo := deps.Storage.SnapshotInfo()
	if snapshotInfo != nil {
		pruningIndex = snapshotInfo.PruningIndex
	}

	return &infoResponse{
		Name:    deps.AppInfo.Name,
		Version: deps.AppInfo.Version,
		Status: nodeStatus{
			IsHealthy: deps.Tangle.IsNodeHealthy(),
			LatestMilestone: milestoneInfoResponse{
				Index:       latestMilestoneIndex,
				Timestamp:   latestMilestoneTimestamp,
				MilestoneID: latestMilestoneIDHex,
			},
			ConfirmedMilestone: milestoneInfoResponse{
				Index:       confirmedMilestoneIndex,
				Timestamp:   confirmedMilestoneTimestamp,
				MilestoneID: confirmedMilestoneIDHex,
			},
			PruningIndex: pruningIndex,
		},
		Protocol:  deps.ProtocolParameters,
		BaseToken: deps.BaseToken,
		Metrics: nodeMetrics{
			MessagesPerSecond:           messagesPerSecond,
			ReferencedMessagesPerSecond: referencedMessagesPerSecond,
			ReferencedRate:              referencedRate,
		},
		Features: features,
		Plugins:  deps.RestPluginManager.Plugins(),
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

	var tips hornet.BlockIDs
	var err error

	if !spammerTips {
		tips, err = deps.TipSelector.SelectNonLazyTips()
	} else {
		_, tips, err = deps.TipSelector.SelectSpammerTips()
	}

	if err != nil {
		if errors.Is(err, common.ErrNodeNotSynced) || errors.Is(err, tipselect.ErrNoTipsAvailable) {
			return nil, errors.WithMessage(echo.ErrServiceUnavailable, err.Error())
		}
		return nil, err
	}

	return &tipsResponse{Tips: tips.ToHex()}, nil
}
