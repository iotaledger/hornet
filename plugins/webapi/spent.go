package webapi

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/address"

	"github.com/iotaledger/hornet/pkg/model/hornet"
	"github.com/iotaledger/hornet/pkg/model/tangle"
)

func (s *WebAPIServer) rpcWereAddressesSpentFrom(c echo.Context) (interface{}, error) {
	request := &WereAddressesSpentFrom{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	if len(request.Addresses) == 0 {
		return nil, errors.WithMessage(ErrInvalidParameter, "invalid request, error: no addresses provided")
	}

	if !tangle.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, tangle.ErrNodeNotSynced.Error())
	}

	result := &WereAddressesSpentFromResponse{}

	for _, addr := range request.Addresses {
		if err := address.ValidAddress(addr); err != nil {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid address hash provided: %s", addr)
		}

		// State
		result.States = append(result.States, tangle.WasAddressSpentFrom(hornet.HashFromAddressTrytes(addr)))
	}

	return result, nil
}

func (s *WebAPIServer) addressWasSpent(c echo.Context) (interface{}, error) {
	addr, err := parseAddressParam(c)
	if err != nil {
		return nil, err
	}

	if !tangle.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, tangle.ErrNodeNotSynced.Error())
	}

	tangle.ReadLockLedger()
	defer tangle.ReadUnlockLedger()

	return &addressWasSpentResponse{
		Address:     addr.Trytes(),
		WasSpent:    tangle.WasAddressSpentFrom(addr),
		LedgerIndex: tangle.GetSolidMilestoneIndex(),
	}, nil
}
