package webapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

const (
	ParameterAddress         = "address"
	ParameterTransactionHash = "txHash"
	ParameterMilestoneIndex  = "index"

	QueryParameterBundle     = "bundle"
	QueryParameterAddress    = "address"
	QueryParameterTag        = "tag"
	QueryParameterApprovee   = "approvee"
	QueryParameterMaxResults = "maxResults"
)

const (
	// RouteRPCEndpoint is the route for sending RPC requests to the API.
	// POST sends an IOTA legacy API request and returns the results.
	RouteRPCEndpoint = "/"

	// RouteInfo is the route for getting the node info.
	// GET returns the node info.
	RouteInfo = "/info"

	// RouteMilestoneByIndex is the route for getting a milestone by its milestoneIndex.
	// GET will return the milestone.
	RouteMilestoneByIndex = "/milestones/by-index/:" + ParameterMilestoneIndex

	// RouteTransactions is the route for getting transactions filtered by the given parameters.
	// GET with query parameter returns all txHashes that fit these filter criteria.
	// Query parameters: "bundle", "address", "tag", "approvee", "maxResults"
	// Returns an empty list if no results are found.
	RouteTransactions = "/transactions" // former findTransactions

	// RouteTransaction is the route for getting a transaction.
	// GET will return the transaction.
	RouteTransaction = "/transactions/:" + ParameterTransactionHash

	// RouteTransactionTrytes is the route for getting the trytes of a transaction.
	// GET will return the transaction trytes.
	RouteTransactionTrytes = "/transactions/:" + ParameterTransactionHash + "/trytes" // former getTrytes

	// RouteTransactionMetadata is the route for getting the metadata of a transaction.
	// GET will return the metadata.
	RouteTransactionMetadata = "/transactions/:" + ParameterTransactionHash + "/metadata" // former getInclusionStates

	// RouteAddressBalance is the route for getting the balance of an address.
	// GET will return the balance.
	RouteAddressBalance = "/addresses/:" + ParameterAddress + "/balance" // former getBalances

	// RouteAddressBalance is the route to check whether an address was already spent or not.
	// GET will return true if the address was already spent.
	RouteAddressWasSpent = "/addresses/:" + ParameterAddress + "/was-spent" // former wereAddressesSpentFrom

	// RouteLedgerState is the route to return the current ledger state.
	// GET will return all addresses with their balances.
	RouteLedgerState = "/ledger/state" // former getLedgerState

	// RouteLedgerStateNonMigrated is the route to return the current non-migrated ledger state.
	// GET will return all non-migrated addresses with their balances.
	RouteLedgerStateNonMigrated = "/ledger/state/non-migrated"

	// RouteLedgerStateByIndex is the route to return the ledger state of a given ledger index.
	// GET will return all addresses with their balances.
	RouteLedgerStateByIndex = "/ledger/state/by-index/:" + ParameterMilestoneIndex // former getLedgerState

	// RouteLedgerStateNonMigratedByIndex is the route to return the non-migrated ledger state of a given ledger index.
	// GET will return all non-migrated addresses with their balances.
	RouteLedgerStateNonMigratedByIndex = "/ledger/state/by-index/:" + ParameterMilestoneIndex + "/non-migrated"

	// RouteLedgerDiffByIndex is the route to return the ledger diff of a given ledger index.
	// GET will return all addresses with their diffs.
	RouteLedgerDiffByIndex = "/ledger/diff/by-index/:" + ParameterMilestoneIndex // former getLedgerDiff

	// RouteLedgerDiffExtendedByIndex is the route to return the ledger diff of a given ledger index with extended informations.
	// GET will return all addresses with their diffs, the confirmed transactions and the confirmed bundles.
	RouteLedgerDiffExtendedByIndex = "/ledger/diff-extended/by-index/:" + ParameterMilestoneIndex // former getLedgerDiffExt
)

func (s *WebAPIServer) configureRPCEndpoint(routeGroup *echo.Group) {

	s.configureRPCEndpoints()

	routeGroup.POST(RouteRPCEndpoint, func(c echo.Context) error {
		resp, err := rpc(c, s.rpcEndpoints)
		if err != nil {
			// the RPC endpoint has custom error handling for compatibility reasons
			var e *echo.HTTPError

			var statusCode int
			var message string
			if errors.As(err, &e) {
				statusCode = e.Code
				message = fmt.Sprintf("%s, error: %s", e.Message, err)
			} else {
				statusCode = http.StatusInternalServerError
				message = fmt.Sprintf("internal server error. error: %s", err)
			}

			return JSONResponse(c, statusCode, &ErrorReturn{Error: message})
		}

		return JSONResponse(c, http.StatusOK, resp)
	})
}

func (s *WebAPIServer) configureRestRoutes(routeGroup *echo.Group) {
	routeGroup.GET(RouteInfo, func(c echo.Context) error {
		resp, err := s.info()
		if err != nil {
			return err
		}

		return JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteMilestoneByIndex, func(c echo.Context) error {
		resp, err := s.milestone(c)
		if err != nil {
			return err
		}

		return JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteTransactions, func(c echo.Context) error {
		resp, err := s.transactions(c)
		if err != nil {
			return err
		}

		return JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteTransaction, func(c echo.Context) error {
		resp, err := s.transaction(c)
		if err != nil {
			return err
		}

		return JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteTransactionTrytes, func(c echo.Context) error {
		resp, err := s.transactionTrytes(c)
		if err != nil {
			return err
		}

		return JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteTransactionMetadata, func(c echo.Context) error {
		resp, err := s.transactionMetadata(c)
		if err != nil {
			return err
		}

		return JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAddressBalance, func(c echo.Context) error {
		resp, err := s.addressBalance(c)
		if err != nil {
			return err
		}

		return JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAddressWasSpent, func(c echo.Context) error {
		resp, err := s.addressWasSpent(c)
		if err != nil {
			return err
		}

		return JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteLedgerState, func(c echo.Context) error {
		resp, err := s.ledgerStateByLatestSolidIndex(c)
		if err != nil {
			return err
		}

		return ledgerStateResponseByMimeType(c, resp.(*ledgerStateResponse))
	})

	routeGroup.GET(RouteLedgerStateNonMigrated, func(c echo.Context) error {
		resp, err := s.ledgerStateNonMigratedByLatestSolidIndex(c)
		if err != nil {
			return err
		}

		return ledgerStateResponseByMimeType(c, resp.(*ledgerStateResponse))
	})

	routeGroup.GET(RouteLedgerStateByIndex, func(c echo.Context) error {
		resp, err := s.ledgerStateByIndex(c)
		if err != nil {
			return err
		}

		return ledgerStateResponseByMimeType(c, resp.(*ledgerStateResponse))
	})

	routeGroup.GET(RouteLedgerStateNonMigratedByIndex, func(c echo.Context) error {
		resp, err := s.ledgerStateNonMigratedByIndex(c)
		if err != nil {
			return err
		}

		return ledgerStateResponseByMimeType(c, resp.(*ledgerStateResponse))
	})

	routeGroup.GET(RouteLedgerDiffByIndex, func(c echo.Context) error {
		resp, err := s.ledgerDiff(c)
		if err != nil {
			return err
		}

		return JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteLedgerDiffExtendedByIndex, func(c echo.Context) error {
		resp, err := s.ledgerDiffExtended(c)
		if err != nil {
			return err
		}

		return JSONResponse(c, http.StatusOK, resp)
	})
}
