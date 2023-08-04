package webapi

import (
	"io"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

type rpcEndpoint func(c echo.Context) (any, error)

func (s *WebAPIServer) configureRPCEndpoints() {
	addEndpoint := func(endpointName string, implementation rpcEndpoint) {
		s.rpcEndpoints[strings.ToLower(endpointName)] = func(c echo.Context) (any, error) {
			return implementation(c)
		}
	}

	addEndpoint("getBalances", s.rpcGetBalances)
	addEndpoint("checkConsistency", s.rpcCheckConsistency)
	addEndpoint("getRequests", s.rpcGetRequests)
	addEndpoint("searchConfirmedApprover", s.rpcSearchConfirmedApprover)
	addEndpoint("searchEntryPoints", s.rpcSearchEntryPoints)
	addEndpoint("triggerSolidifier", s.rpcTriggerSolidifier)
	addEndpoint("getFundsOnSpentAddresses", s.rpcGetFundsOnSpentAddresses)
	addEndpoint("getInclusionStates", s.rpcGetInclusionStates)
	addEndpoint("getNodeInfo", s.rpcGetNodeInfo)
	addEndpoint("getNodeAPIConfiguration", s.rpcGetNodeAPIConfiguration)
	addEndpoint("getLedgerDiff", s.rpcGetLedgerDiff)
	addEndpoint("getLedgerDiffExt", s.rpcGetLedgerDiffExt)
	addEndpoint("getLedgerState", s.rpcGetLedgerState)
	addEndpoint("addNeighbors", s.rpcAddNeighbors)
	addEndpoint("removeNeighbors", s.rpcRemoveNeighbors)
	addEndpoint("getNeighbors", s.rpcGetNeighbors)
	addEndpoint("attachToTangle", s.rpcAttachToTangle)
	addEndpoint("pruneDatabase", s.rpcPruneDatabase)
	addEndpoint("createSnapshotFile", s.rpcCreateSnapshotFile)
	addEndpoint("wereAddressesSpentFrom", s.rpcWereAddressesSpentFrom)
	addEndpoint("getTipInfo", s.rpcGetTipInfo)
	addEndpoint("getTransactionsToApprove", s.rpcGetTransactionsToApprove)
	addEndpoint("getSpammerTips", s.rpcGetSpammerTips)
	addEndpoint("broadcastTransactions", s.rpcBroadcastTransactions)
	addEndpoint("findTransactions", s.rpcFindTransactions)
	addEndpoint("storeTransactions", s.rpcStoreTransactions)
	addEndpoint("getTrytes", s.rpcGetTrytes)
	addEndpoint("getWhiteFlagConfirmation", s.rpcGetWhiteFlagConfirmation)
}

func rpc(c echo.Context, implementedAPIcalls map[string]rpcEndpoint) (interface{}, error) {

	request := &Request{}

	// Read the content of the body
	var bodyBytes []byte
	if c.Request().Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(c.Request().Body)
		if err != nil {
			return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
		}
	}

	// we need to restore the body after reading it
	restoreBody(c, bodyBytes)

	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	// we need to restore the body after reading it
	restoreBody(c, bodyBytes)

	implementation, exists := implementedAPIcalls[strings.ToLower(request.Command)]
	if !exists {
		return nil, errors.WithMessagef(ErrInvalidParameter, "command is unknown: %s", request.Command)
	}

	return implementation(c)
}
