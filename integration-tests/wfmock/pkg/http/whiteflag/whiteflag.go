package whiteflag

import (
	"fmt"
	"net/http"

	httpapi "github.com/gohornet/hornet/integration-tests/wfmock/pkg/http"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/labstack/echo/v4"
	"github.com/mitchellh/mapstructure"
)

// GetWhiteFlagConfirmationCommand defines the command for the getWhiteFlagConfirmation API call.
const GetWhiteFlagConfirmationCommand = "getWhiteFlagConfirmation"

// GetWhiteFlagConfirmationRequest represents the payload to the GetWhiteFlagConfirmation API call.
type GetWhiteFlagConfirmationRequest struct {
	Command        string `mapstructure:"command"`
	MilestoneIndex uint32 `mapstructure:"milestoneIndex"`
}

// GetWhiteFlagConfirmationResponse defines the response of a getWhiteFlagConfirmation API call.
type GetWhiteFlagConfirmationResponse struct {
	MilestoneBundle []trinary.Trytes   `json:"milestoneBundle"`
	IncludedBundles [][]trinary.Trytes `json:"includedBundles"`
}

func getWhiteFlagConfirmation(i interface{}, c echo.Context) error {
	request := &GetWhiteFlagConfirmationRequest{}
	if err := mapstructure.Decode(i, request); err != nil {
		e := httpapi.ErrorReturn{
			Error: fmt.Sprintf("invalid request: %s", err),
		}
		return c.JSON(http.StatusBadRequest, e)
	}

	if request.MilestoneIndex == 0 || request.MilestoneIndex >= uint32(len(data.milestones)) {
		e := httpapi.ErrorReturn{
			Error: fmt.Sprintf("milestone not found for wf-confirmation at %d", request.MilestoneIndex),
		}
		return c.JSON(http.StatusBadRequest, e)
	}

	confirmation := data.milestones[request.MilestoneIndex]
	res := GetWhiteFlagConfirmationResponse{
		MilestoneBundle: confirmation.milestoneBundle,
		IncludedBundles: confirmation.includedMigrationBundles,
	}
	return c.JSON(http.StatusOK, res)
}
