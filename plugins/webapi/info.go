package webapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

func init() {
	addEndpoint("getNodeAPIConfiguration", getNodeAPIConfiguration, implementedAPIcalls)
}

func getNodeAPIConfiguration(_ interface{}, c *gin.Context, _ <-chan struct{}) {

	result := GetNodeAPIConfigurationReturn{
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

	c.JSON(http.StatusOK, result)
}
