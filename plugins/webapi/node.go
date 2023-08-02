package webapi

import (
	"time"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/hornet/pkg/config"
	"github.com/iotaledger/hornet/pkg/metrics"
	"github.com/iotaledger/hornet/pkg/model/tangle"
	"github.com/iotaledger/hornet/plugins/cli"
	"github.com/iotaledger/hornet/plugins/gossip"
	"github.com/iotaledger/hornet/plugins/peering"

	tangleplugin "github.com/iotaledger/hornet/plugins/tangle"

	"github.com/iotaledger/iota.go/consts"
)

func (s *WebAPIServer) rpcGetNodeInfo(_ echo.Context) (any, error) {
	// Basic info data
	result := &GetNodeInfoResponse{
		AppName:                       cli.AppName,
		AppVersion:                    cli.AppVersion,
		LatestMilestone:               consts.NullHashTrytes,
		LatestSolidSubtangleMilestone: consts.NullHashTrytes,
	}

	// Latest milestone index
	lmi := tangle.GetLatestMilestoneIndex()
	// Latest milestone hash
	cachedLatestMs := tangle.GetMilestoneOrNil(lmi) // bundle +1
	if cachedLatestMs != nil {
		result.LatestMilestone = cachedLatestMs.GetBundle().GetMilestoneHash().Trytes()
		cachedLatestMs.Release(true) // bundle -1
	}
	result.LatestMilestoneIndex = lmi

	// Solid milestone index
	smi := tangle.GetSolidMilestoneIndex()
	// Solid milestone hash
	cachedSolidMs := tangle.GetMilestoneOrNil(smi) // bundle +1
	if cachedSolidMs != nil {
		result.LatestSolidSubtangleMilestone = cachedSolidMs.GetBundle().GetMilestoneHash().Trytes()
		cachedSolidMs.Release(true) // bundle -1
	}
	result.LatestSolidSubtangleMilestoneIndex = smi

	result.IsSynced = tangle.IsNodeSyncedWithThreshold()
	result.Health = tangleplugin.IsNodeHealthy()

	// Milestone start index
	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		result.MilestoneStartIndex = snapshotInfo.PruningIndex
		result.LastSnapshottedMilestoneIndex = snapshotInfo.SnapshotIndex
	}

	// Number of peers
	result.Neighbors = uint(peering.Manager().ConnectedPeerCount())

	// System time
	result.Time = time.Now().UnixMilli()

	// Tips
	result.Tips = metrics.SharedServerMetrics.TipsNonLazy.Load() + metrics.SharedServerMetrics.TipsSemiLazy.Load()

	// TX to request
	queued, pending, _ := gossip.RequestQueue().Size()
	result.TransactionsToRequest = queued + pending

	// Features
	// Workaround until https://github.com/golang/go/issues/27589 is fixed
	if len(s.features) != 0 {
		result.Features = s.features
	} else {
		result.Features = []string{}
	}

	// Coo addr
	result.CoordinatorAddress = config.NodeConfig.GetString(config.CfgCoordinatorAddress)

	return &result, nil
}

func (s *WebAPIServer) rpcGetNodeAPIConfiguration(_ echo.Context) (any, error) {
	result := &GetNodeAPIConfigurationResponse{
		MaxFindTransactions: config.NodeConfig.GetInt(config.CfgWebAPILimitsMaxFindTransactions),
		MaxRequestsList:     config.NodeConfig.GetInt(config.CfgWebAPILimitsMaxRequestsList),
		MaxGetTrytes:        config.NodeConfig.GetInt(config.CfgWebAPILimitsMaxGetTrytes),
		MaxBodyLength:       config.NodeConfig.GetInt(config.CfgWebAPILimitsMaxBodyLengthBytes),
	}

	// Milestone start index
	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		result.MilestoneStartIndex = snapshotInfo.PruningIndex
	}

	return result, nil
}

//nolint:unparam // even if the error is never used, the structure of all routes should be the same
func (s *WebAPIServer) info() (*infoResponse, error) {
	// Basic info data
	result := &infoResponse{
		AppName:                       cli.AppName,
		AppVersion:                    cli.AppVersion,
		LatestMilestone:               consts.NullHashTrytes,
		LatestSolidSubtangleMilestone: consts.NullHashTrytes,
	}

	// Latest milestone index
	lmi := tangle.GetLatestMilestoneIndex()
	// Latest milestone hash
	cachedLatestMs := tangle.GetMilestoneOrNil(lmi) // bundle +1
	if cachedLatestMs != nil {
		result.LatestMilestone = cachedLatestMs.GetBundle().GetMilestoneHash().Trytes()
		cachedLatestMs.Release(true) // bundle -1
	}
	result.LatestMilestoneIndex = lmi

	// Solid milestone index
	smi := tangle.GetSolidMilestoneIndex()
	// Solid milestone hash
	cachedSolidMs := tangle.GetMilestoneOrNil(smi) // bundle +1
	if cachedSolidMs != nil {
		result.LatestSolidSubtangleMilestone = cachedSolidMs.GetBundle().GetMilestoneHash().Trytes()
		cachedSolidMs.Release(true) // bundle -1
	}
	result.LatestSolidSubtangleMilestoneIndex = smi

	result.IsSynced = tangle.IsNodeSyncedWithThreshold()
	result.Health = tangleplugin.IsNodeHealthy()

	// Milestone start index
	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		result.MilestoneStartIndex = snapshotInfo.PruningIndex
		result.LastSnapshottedMilestoneIndex = snapshotInfo.SnapshotIndex
	}

	// Number of peers
	result.Neighbors = uint(peering.Manager().ConnectedPeerCount())

	// System time
	result.Time = time.Now().UnixMilli()

	// Tips
	result.Tips = metrics.SharedServerMetrics.TipsNonLazy.Load() + metrics.SharedServerMetrics.TipsSemiLazy.Load()

	// TX to request
	queued, pending, _ := gossip.RequestQueue().Size()
	result.TransactionsToRequest = queued + pending

	// Features
	// Workaround until https://github.com/golang/go/issues/27589 is fixed
	if len(s.features) != 0 {
		result.Features = s.features
	} else {
		result.Features = []string{}
	}

	// Coo addr
	result.CoordinatorAddress = config.NodeConfig.GetString(config.CfgCoordinatorAddress)

	return result, nil
}
