package v2

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/tangle"
	iotago "github.com/iotaledger/iota.go/v3"
)

func computeWhiteFlagMutations(c echo.Context) (*ComputeWhiteFlagMutationsResponse, error) {

	request := &ComputeWhiteFlagMutationsRequest{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid request, error: %s", err)
	}

	requestedIndex := request.Index
	requestedTimestamp := request.Timestamp
	requestedParents, err := hornet.MessageIDsFromHex(request.Parents)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid parents, error: %s", err)
	}
	requestedLastMilestoneID := iotago.MilestoneID{}
	if len(request.LastMilestoneID) > 0 {
		lastMilestoneIDBytes, err := iotago.DecodeHex(request.LastMilestoneID)
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid lastMilestoneID, error: %s", err)
		}
		if len(lastMilestoneIDBytes) != iotago.MilestoneIDLength {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid lastMilestoneID, length should be %d bytes", iotago.MilestoneIDLength)
		}
		copy(requestedLastMilestoneID[:], lastMilestoneIDBytes)
	}

	mutations, err := deps.Tangle.CheckSolidityAndComputeWhiteFlagMutations(Plugin.Daemon().ContextStopped(), requestedIndex, requestedTimestamp, requestedParents, requestedLastMilestoneID)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrNodeNotSynced):
			return nil, errors.WithMessagef(echo.ErrServiceUnavailable, "failed to compute white flag mutations: %s", err.Error())
		case errors.Is(err, tangle.ErrParentsNotGiven):
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "failed to compute white flag mutations: %s", err.Error())
		case errors.Is(err, tangle.ErrParentsNotSolid):
			return nil, errors.WithMessagef(echo.ErrServiceUnavailable, "failed to compute white flag mutations: %s", err.Error())
		case errors.Is(err, common.ErrOperationAborted):
			return nil, errors.WithMessagef(echo.ErrServiceUnavailable, "failed to compute white flag mutations: %s", err.Error())
		default:
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "failed to compute white flag mutations: %s", err)
		}
	}

	return &ComputeWhiteFlagMutationsResponse{
		ConfirmedMerkleRoot: iotago.EncodeHex(mutations.ConfirmedMerkleRoot[:]),
		AppliedMerkleRoot:   iotago.EncodeHex(mutations.AppliedMerkleRoot[:]),
	}, nil
}
