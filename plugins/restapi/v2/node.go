package v2

import (
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/tipselect"
	iotago "github.com/iotaledger/iota.go/v3"
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

	// latest milestone index
	latestMilestoneIndex := deps.SyncManager.LatestMilestoneIndex()

	// latest milestone timestamp
	var latestMilestoneTimestamp uint32 = 0
	cachedMilestoneLatest := deps.Storage.CachedMilestoneOrNil(latestMilestoneIndex) // milestone +1
	if cachedMilestoneLatest != nil {
		latestMilestoneTimestamp = uint32(cachedMilestoneLatest.Milestone().Timestamp.Unix())
		cachedMilestoneLatest.Release(true) // milestone -1
	}

	// confirmed milestone index
	confirmedMilestoneIndex := deps.SyncManager.ConfirmedMilestoneIndex()

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
			IsHealthy:                deps.Tangle.IsNodeHealthy(),
			LatestMilestoneTimestamp: latestMilestoneTimestamp,
			LatestMilestoneIndex:     latestMilestoneIndex,
			ConfirmedMilestoneIndex:  confirmedMilestoneIndex,
			PruningIndex:             pruningIndex,
		},
		Protocol: protocolParameters{
			NetworkName:     deps.NetworkIDName,
			ProtocolVersion: iotago.ProtocolVersion,
			Bech32HRP:       string(deps.Bech32HRP),
			MinPoWScore:     deps.MinPoWScore,
			RentStructure:   deps.DeserializationParameters.RentStructure,
		},
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

	var tips hornet.MessageIDs
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
