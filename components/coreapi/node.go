package coreapi

import (
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/tipselect"
	iotago "github.com/iotaledger/iota.go/v3"
)

//nolint:unparam // even if the error is never used, the structure of all routes should be the same
func info() (*infoResponse, error) {

	var blocksPerSecond, referencedBlocksPerSecond, referencedRate float64
	lastConfirmedMilestoneMetric := deps.Tangle.LastConfirmedMilestoneMetric()
	if lastConfirmedMilestoneMetric != nil {
		blocksPerSecond = lastConfirmedMilestoneMetric.BPS
		referencedBlocksPerSecond = lastConfirmedMilestoneMetric.RBPS
		referencedRate = lastConfirmedMilestoneMetric.ReferencedRate
	}

	syncState := deps.SyncManager.SyncState()

	// latest milestone
	var latestMilestoneIndex = syncState.LatestMilestoneIndex
	var latestMilestoneTimestamp uint32
	var latestMilestoneIDHex string
	cachedMilestoneLatest := deps.Storage.CachedMilestoneByIndexOrNil(latestMilestoneIndex) // milestone +1
	if cachedMilestoneLatest != nil {
		latestMilestoneTimestamp = cachedMilestoneLatest.Milestone().TimestampUnix()
		latestMilestoneIDHex = cachedMilestoneLatest.Milestone().MilestoneIDHex()
		cachedMilestoneLatest.Release(true) // milestone -1
	}

	// confirmed milestone index
	var confirmedMilestoneIndex = syncState.ConfirmedMilestoneIndex
	var confirmedMilestoneTimestamp uint32
	var confirmedMilestoneIDHex string
	cachedMilestoneConfirmed := deps.Storage.CachedMilestoneByIndexOrNil(confirmedMilestoneIndex) // milestone +1
	if cachedMilestoneConfirmed != nil {
		confirmedMilestoneTimestamp = cachedMilestoneConfirmed.Milestone().TimestampUnix()
		confirmedMilestoneIDHex = cachedMilestoneConfirmed.Milestone().MilestoneIDHex()
		cachedMilestoneConfirmed.Release(true) // milestone -1
	}

	// pruning index
	var pruningIndex iotago.MilestoneIndex
	snapshotInfo := deps.Storage.SnapshotInfo()
	if snapshotInfo != nil {
		pruningIndex = snapshotInfo.PruningIndex()
	}

	return &infoResponse{
		Name:    deps.AppInfo.Name,
		Version: deps.AppInfo.Version,
		Status: nodeStatus{
			IsHealthy: deps.Tangle.IsNodeHealthy(syncState),
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
		SupportedProtocolVersions: deps.ProtocolManager.SupportedVersions(),
		ProtocolParameters:        deps.ProtocolManager.Current(),
		PendingProtocolParameters: deps.ProtocolManager.Pending(),
		BaseToken:                 deps.BaseToken,
		Metrics: nodeMetrics{
			BlocksPerSecond:           blocksPerSecond,
			ReferencedBlocksPerSecond: referencedBlocksPerSecond,
			ReferencedRate:            referencedRate,
		},
		Features: features,
	}, nil
}

func tips(c echo.Context) (*tipsResponse, error) {
	allowSemiLazy := false
	for query := range c.QueryParams() {
		if strings.ToLower(query) == "allowsemilazy" {
			allowSemiLazy = true

			break
		}
	}

	var tips iotago.BlockIDs
	var err error

	if !allowSemiLazy {
		tips, err = deps.TipSelector.SelectNonLazyTips()
	} else {
		tips, err = deps.TipSelector.SelectTipsWithSemiLazyAllowed()
	}

	if err != nil {
		if errors.Is(err, common.ErrNodeNotSynced) || errors.Is(err, tipselect.ErrNoTipsAvailable) {
			return nil, errors.WithMessage(echo.ErrServiceUnavailable, err.Error())
		}

		return nil, err
	}

	return &tipsResponse{Tips: tips.ToHex()}, nil
}
