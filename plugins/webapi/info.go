package webapi

import (
	"github.com/iotaledger/iota.go/consts"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/iotaledger/hive.go/parameter"

	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/plugins/cli"
	"github.com/gohornet/hornet/plugins/gossip"
)

func init() {
	addEndpoint("getNodeInfo", getNodeInfo, implementedAPIcalls)
	addEndpoint("getNodeAPIConfiguration", getNodeAPIConfiguration, implementedAPIcalls)
}

func getNodeInfo(i interface{}, c *gin.Context) {
	e := ErrorReturn{}
	// Basic info data
	info := GetNodeInfoReturn{
		AppName:    cli.AppName,
		AppVersion: cli.AppVersion,
	}

	// Number of neighbors
	info.Neighbors = uint(len(gossip.GetNeighbors()))

	// Latest milestone index
	lmi := tangle.GetLatestMilestoneIndex()
	info.LatestMilestoneIndex = uint32(lmi)

	// Latest milestone hash
	lsmBndl, err := tangle.GetMilestone(lmi)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if lsmBndl != nil && lsmBndl.GetTail() != nil {
		tail := lsmBndl.GetTail()
		info.LatestMilestone = tail.GetTransaction().GetHash()
		tail.Release()
	} else {
		info.LatestMilestone = consts.NullHashTrytes
	}

	// Solid milestone index
	smi := tangle.GetSolidMilestoneIndex()
	info.LatestSolidSubtangleMilestoneIndex = uint32(smi)
	info.IsSynced = tangle.IsNodeSynced()

	// Solid milestone hash
	smBndl, err := tangle.GetMilestone(smi)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if smBndl != nil && smBndl.GetTail() != nil {
		tail := smBndl.GetTail()
		info.LatestSolidSubtangleMilestone = tail.GetTransaction().GetHash()
		tail.Release()
	} else {
		info.LatestSolidSubtangleMilestone = consts.NullHashTrytes
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
	info.Features = features

	// TX to request
	_, info.TransactionsToRequest = gossip.RequestQueue.CurrentMilestoneIndexAndSize()

	// Coo addr
	info.CoordinatorAddress = parameter.NodeConfig.GetString("milestones.coordinator")

	// Return node info
	c.JSON(http.StatusOK, info)
}

func getNodeAPIConfiguration(i interface{}, c *gin.Context) {

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
