package whiteflag

import (
	"net/http"

	"github.com/iotaledger/iota.go/trinary"
	"github.com/labstack/echo/v4"
)

// GetNodeInfoCommand defines the command for the getNodeInfo API call.
const GetNodeInfoCommand = "getNodeInfo"

// GetNodeInfoResponse defines the response of a getNodeInfo API call.
type GetNodeInfoResponse struct {
	AppName                            string       `json:"appName"`
	AppVersion                         string       `json:"appVersion"`
	LatestMilestone                    trinary.Hash `json:"latestMilestone"`
	LatestMilestoneIndex               uint32       `json:"latestMilestoneIndex"`
	LatestSolidSubtangleMilestone      trinary.Hash `json:"latestSolidSubtangleMilestone"`
	LatestSolidSubtangleMilestoneIndex uint32       `json:"latestSolidSubtangleMilestoneIndex"`
	MilestoneStartIndex                uint32       `json:"milestoneStartIndex"`
	LastSnapshottedMilestoneIndex      uint32       `json:"lastSnapshottedMilestoneIndex"`
	Neighbors                          uint         `json:"neighbors"`
	PacketsQueueSize                   uint         `json:"packetsQueueSize"`
	Time                               int64        `json:"time"`
	Tips                               uint         `json:"tips"`
	TransactionsToRequest              uint         `json:"transactionsToRequest"`
	Features                           []string     `json:"features"`
	CoordinatorAddress                 trinary.Hash `json:"coordinatorAddress"`
	DBSizeInBytes                      uint         `json:"dbSizeInBytes"`
}

func getNodeInfo(_ interface{}, c echo.Context) error {
	result := GetNodeInfoResponse{
		AppName:                            "WhiteFlag Mock",
		LatestMilestoneIndex:               data.latestMilestoneIndex,
		LatestMilestone:                    data.latestMilestoneHash,
		LatestSolidSubtangleMilestoneIndex: data.latestMilestoneIndex,
		LatestSolidSubtangleMilestone:      data.latestMilestoneHash,
		CoordinatorAddress:                 data.coordinatorAddress,
	}
	return c.JSON(http.StatusOK, result)
}
