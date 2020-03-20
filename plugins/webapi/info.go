package webapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gohornet/hornet/plugins/gossip"

	"github.com/iotaledger/iota.go/consts"

	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/plugins/cli"
	"github.com/gohornet/hornet/plugins/peering"
)

func init() {
	addEndpoint("getNodeInfo", getNodeInfo, implementedAPIcalls)
	addEndpoint("getNodeAPIConfiguration", getNodeAPIConfiguration, implementedAPIcalls)
}

func getNodeInfo(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	// Basic info data
	info := GetNodeInfoReturn{
		AppName:    cli.AppName,
		AppVersion: cli.AppVersion,
	}

	// Node Alias
	if config.NodeConfig.GetBool(config.CfgNodeShowAliasInGetNodeInfo) {
		info.NodeAlias = config.NodeConfig.GetString(config.CfgNodeAlias)
	}
	
	// Number of peers
	info.Neighbors = uint(peering.Manager().PeerCount())

	// Latest milestone index
	lmi := tangle.GetLatestMilestoneIndex()
	info.LatestMilestoneIndex = uint32(lmi)
	info.LatestMilestone = consts.NullHashTrytes

	// Latest milestone hash
	cachedLatestMs := tangle.GetMilestoneOrNil(lmi) // bundle +1
	if cachedLatestMs != nil {
		cachedMsTailTx := cachedLatestMs.GetBundle().GetTail() // tx +1
		info.LatestMilestone = cachedMsTailTx.GetTransaction().GetHash()
		cachedMsTailTx.Release(true) // tx -1
		cachedLatestMs.Release(true) // bundle -1
	}

	// Solid milestone index
	smi := tangle.GetSolidMilestoneIndex()
	info.LatestSolidSubtangleMilestoneIndex = uint32(smi)
	info.LatestSolidSubtangleMilestone = consts.NullHashTrytes
	info.IsSynced = tangle.IsNodeSyncedWithThreshold()

	// Solid milestone hash
	cachedSolidMs := tangle.GetMilestoneOrNil(smi) // bundle +1
	if cachedSolidMs != nil {
		cachedMsTailTx := cachedSolidMs.GetBundle().GetTail() // tx +1
		info.LatestSolidSubtangleMilestone = cachedMsTailTx.GetTransaction().GetHash()
		cachedMsTailTx.Release(true) // tx -1
		cachedSolidMs.Release(true)  // bundle -1
	}

	// Milestone start index
	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		info.MilestoneStartIndex = uint32(snapshotInfo.PruningIndex)
		info.LastSnapshottedMilestoneIndex = uint32(snapshotInfo.PruningIndex)
	}

	// System time
	info.Time = time.Now().Unix() * 1000

	// Features
	// Workaround until https://github.com/golang/go/issues/27589 is fixed
	if len(features) != 0 {
		info.Features = features
	} else {
		info.Features = []string{}
	}

	// TX to request
	queued, pending := gossip.RequestQueue().Size()
	info.TransactionsToRequest = queued + pending

	// Coo addr
	info.CoordinatorAddress = config.NodeConfig.GetString(config.CfgMilestoneCoordinator)

	// Return node info
	c.JSON(http.StatusOK, info)
}

func getNodeAPIConfiguration(_ interface{}, c *gin.Context, _ <-chan struct{}) {

	config := GetNodeAPIConfigurationReturn{
		MaxFindTransactions: config.NodeConfig.GetInt(config.CfgWebAPILimitsMaxFindTransactions),
		MaxRequestsList:     config.NodeConfig.GetInt(config.CfgWebAPILimitsMaxRequestsList),
		MaxGetTrytes:        config.NodeConfig.GetInt(config.CfgWebAPILimitsMaxGetTrytes),
		MaxBodyLength:       config.NodeConfig.GetInt(config.CfgWebAPILimitsMaxBodyLengthBytes),
	}

	// Milestone start index
	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		config.MilestoneStartIndex = uint32(snapshotInfo.PruningIndex)
	}

	c.JSON(http.StatusOK, config)
}
