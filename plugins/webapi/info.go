package webapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/iotaledger/iota.go/consts"

	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/parameter"
	"github.com/gohornet/hornet/plugins/cli"
	"github.com/gohornet/hornet/plugins/gossip"
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

	// Number of neighbors
	info.Neighbors = uint(gossip.GetNeighborsCount())

	// Latest milestone index
	lmi := tangle.GetLatestMilestoneIndex()
	info.LatestMilestoneIndex = uint32(lmi)
	info.LatestMilestone = consts.NullHashTrytes

	// Latest milestone hash
	lsmBndl := tangle.GetMilestone(lmi)
	if lsmBndl != nil {
		tail := lsmBndl.GetTail() //+1
		if tail != nil {
			info.LatestMilestone = tail.GetTransaction().GetHash()
		}
		tail.Release() //-1
	}

	// Solid milestone index
	smi := tangle.GetSolidMilestoneIndex()
	info.LatestSolidSubtangleMilestoneIndex = uint32(smi)
	info.LatestSolidSubtangleMilestone = consts.NullHashTrytes
	info.IsSynced = tangle.IsNodeSynced()

	// Solid milestone hash
	smBndl := tangle.GetMilestone(smi)
	if smBndl != nil {
		tail := smBndl.GetTail() //+1
		if tail != nil {
			info.LatestSolidSubtangleMilestone = tail.GetTransaction().GetHash()
		}
		tail.Release() //-1
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
	_, info.TransactionsToRequest = gossip.RequestQueue.CurrentMilestoneIndexAndSize()

	// Coo addr
	info.CoordinatorAddress = parameter.NodeConfig.GetString("milestones.coordinator")

	// Return node info
	c.JSON(http.StatusOK, info)
}

func getNodeAPIConfiguration(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {

	config := GetNodeAPIConfigurationReturn{
		MaxFindTransactions: parameter.NodeConfig.GetInt("api.maxFindTransactions"),
		MaxRequestsList:     parameter.NodeConfig.GetInt("api.maxRequestsList"),
		MaxGetTrytes:        parameter.NodeConfig.GetInt("api.maxGetTrytes"),
		MaxBodyLength:       parameter.NodeConfig.GetInt("api.maxBodyLength"),
	}

	// Milestone start index
	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		config.MilestoneStartIndex = uint32(snapshotInfo.PruningIndex)
	}

	c.JSON(http.StatusOK, config)
}
