package restapi

import (
	"github.com/labstack/echo/v4"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"

	"github.com/iotaledger/inx-app/httpserver"
)

const (
	// ParameterBlockID is used to identify a block by its ID.
	ParameterBlockID = "blockID"

	// ParameterTransactionID is used to identify a transaction by its ID.
	ParameterTransactionID = "transactionID"

	// ParameterOutputID is used to identify an output by its ID.
	ParameterOutputID = "outputID"

	// ParameterMilestoneIndex is used to identify a milestone by index.
	ParameterMilestoneIndex = "milestoneIndex"

	// ParameterMilestoneID is used to identify a milestone by its ID.
	ParameterMilestoneID = "milestoneID"

	// ParameterPeerID is used to identify a peer.
	ParameterPeerID = "peerID"
)

type (
	// AllowedRoute defines a function to allow or disallow routes.
	AllowedRoute func(echo.Context) bool
)

func ParsePeerIDParam(c echo.Context) (peer.ID, error) {
	peerID, err := peer.Decode(c.Param(ParameterPeerID))
	if err != nil {
		return "", errors.WithMessagef(httpserver.ErrInvalidParameter, "invalid peerID, error: %s", err)
	}

	return peerID, nil
}
