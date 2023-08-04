package webapi

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/address"

	"github.com/iotaledger/hornet/pkg/model/hornet"
	"github.com/iotaledger/hornet/pkg/model/tangle"
)

func (s *WebAPIServer) rpcGetBalances(c echo.Context) (interface{}, error) {
	request := &GetBalances{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	if len(request.Addresses) == 0 {
		return nil, errors.WithMessage(ErrInvalidParameter, "invalid request, error: no addresses provided")
	}

	for _, addr := range request.Addresses {
		// Check if address is valid
		if err := address.ValidAddress(addr); err != nil {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid address hash provided: %s", addr)
		}
	}

	if !tangle.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, tangle.ErrNodeNotSynced.Error())
	}

	tangle.ReadLockLedger()
	defer tangle.ReadUnlockLedger()

	cachedLatestSolidMs := tangle.GetMilestoneOrNil(tangle.GetSolidMilestoneIndex()) // bundle +1
	if cachedLatestSolidMs == nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, "Ledger state invalid - Milestone not found")
	}
	defer cachedLatestSolidMs.Release(true) // bundle -1

	result := &GetBalancesResponse{}

	for _, addr := range request.Addresses {

		balance, _, err := tangle.GetBalanceForAddressWithoutLocking(hornet.HashFromAddressTrytes(addr))
		if err != nil {
			return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
		}

		// Address balance
		result.Balances = append(result.Balances, strconv.FormatUint(balance, 10))
	}

	// The index of the milestone that confirmed the most recent balance
	result.MilestoneIndex = cachedLatestSolidMs.GetBundle().GetMilestoneIndex()
	result.References = []string{cachedLatestSolidMs.GetBundle().GetMilestoneHash().Trytes()}

	return result, nil
}

func (s *WebAPIServer) addressBalance(c echo.Context) (interface{}, error) {
	addr, err := parseAddressParam(c)
	if err != nil {
		return nil, err
	}

	if !tangle.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, tangle.ErrNodeNotSynced.Error())
	}

	tangle.ReadLockLedger()
	defer tangle.ReadUnlockLedger()

	balance, _, err := tangle.GetBalanceForAddressWithoutLocking(addr)
	if err != nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	return &balanceResponse{
		Address:     addr.Trytes(),
		Balance:     strconv.FormatUint(balance, 10),
		LedgerIndex: tangle.GetSolidMilestoneIndex(),
	}, nil
}
