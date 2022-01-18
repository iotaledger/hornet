package indexer

import (
	"github.com/gohornet/hornet/pkg/indexer"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"net/http"
)

const (

	// RouteOutputs is the route for getting outputs filtered by the given parameters.
	// GET with query parameter returns all outputIDs that fit these filter criteria (query parameters: "address", "requiresDustReturn", "sender", "tag").
	// Returns an empty list if no results are found.
	RouteOutputs = "/outputs"

	// RouteAliases is the route for getting aliases filtered by the given parameters.
	// GET with query parameter  returns all outputIDs that fit these filter criteria (query parameters: "stateController", "governanceController", "issuer", "sender").
	// Returns an empty list if no results are found.
	RouteAliases = "/aliases"

	// RouteAliasByID is the route for getting aliases by their aliasID.
	// GET returns the outputIDs or 404 if no record is found.
	RouteAliasByID = "/aliases/:" + restapi.ParameterAliasID

	// RouteNFT is the route for getting NFT filtered by the given parameters.
	// GET with query parameter returns all outputIDs that fit these filter criteria (query parameters: "address", "requiresDustReturn", "issuer", "sender", "tag").
	// Returns an empty list if no results are found.
	RouteNFT = "/nft"

	// RouteNFTByID is the route for getting NFT by their nftID.
	// GET returns the outputIDs or 404 if no record is found.
	RouteNFTByID = "/nft/:" + restapi.ParameterNFTID

	// RouteFoundries is the route for getting foundries filtered by the given parameters.
	// GET with query parameter returns all outputIDs that fit these filter criteria (query parameters: "address").
	// Returns an empty list if no results are found.
	RouteFoundries = "/foundries"

	// RouteFoundryByID is the route for getting foundries by their foundryID.
	// GET returns the outputIDs or 404 if no record is found.
	RouteFoundryByID = "/foundries/:" + restapi.ParameterFoundryID
)

const (
	// QueryParameterRequiresDustReturn is used to filter for outputs requiring a dust return.
	QueryParameterRequiresDustReturn = "requiresDustReturn"

	// QueryParameterStateController is used to filter for a certain state controller address.
	QueryParameterStateController = "stateController"

	// QueryParameterGovernanceController is used to filter for a certain governance controller address.
	QueryParameterGovernanceController = "governanceController"
)

func nodeSyncedMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			if !deps.SyncManager.WaitForNodeSynced(waitForNodeSyncedTimeout) {
				return errors.WithMessage(echo.ErrServiceUnavailable, "node is not synced")
			}
			return next(c)
		}
	}
}

func configureRoutes(routeGroup *echo.Group) {

	routeGroup.Use(nodeSyncedMiddleware())

	routeGroup.GET(RouteOutputs, func(c echo.Context) error {
		resp, err := outputsWithFilter(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAliases, func(c echo.Context) error {
		resp, err := aliasesWithFilter(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAliasByID, func(c echo.Context) error {
		resp, err := aliasByID(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteNFT, func(c echo.Context) error {
		resp, err := nftWithFilter(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteNFTByID, func(c echo.Context) error {
		resp, err := nftByID(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteFoundries, func(c echo.Context) error {
		resp, err := foundriesWithFilter(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteFoundryByID, func(c echo.Context) error {
		resp, err := foundryByID(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})
}

func outputsWithFilter(c echo.Context) (*outputsResponse, error) {
	maxResults := deps.RestAPILimitsMaxResults
	filters := []indexer.ExtendedOutputFilterOption{indexer.ExtendedOutputMaxResults(maxResults)}

	if len(c.QueryParam(restapi.QueryParameterAddress)) > 0 {
		address, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, restapi.QueryParameterAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputUnlockableByAddress(address))
	}

	if len(c.QueryParam(QueryParameterRequiresDustReturn)) > 0 {
		requiresDust, err := restapi.ParseBoolQueryParam(c, QueryParameterRequiresDustReturn)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputRequiresDustReturn(requiresDust))
	}

	if len(c.QueryParam(restapi.QueryParameterSender)) > 0 {
		sender, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, restapi.QueryParameterSender)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputSender(sender))
	}

	if len(c.QueryParam(restapi.QueryParameterIndex)) > 0 {
		_, indexBytes, err := restapi.ParseIndexQueryParam(c)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputTag(indexBytes))
	}

	outputIDs, ledgerIndex, err := deps.Indexer.ExtendedOutputsWithFilters(filters...)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading outputs failed: %s", err)
	}

	return &outputsResponse{
		MaxResults:  uint32(maxResults),
		Count:       uint32(len(outputIDs)),
		OutputIDs:   outputIDs.ToHex(),
		LedgerIndex: ledgerIndex,
	}, nil
}

func aliasByID(c echo.Context) (*outputsResponse, error) {
	maxResults := deps.RestAPILimitsMaxResults

	aliasID, err := restapi.ParseAliasIDParam(c)
	if err != nil {
		return nil, err
	}

	outputID, ledgerIndex, err := deps.Indexer.AliasOutput(aliasID)
	if err != nil {
		if errors.Is(err, indexer.ErrNotFound) {
			return nil, errors.WithMessage(echo.ErrNotFound, "alias not found")
		}
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading alias failed: %s", err)
	}

	return &outputsResponse{
		MaxResults:  uint32(maxResults),
		Count:       1,
		OutputIDs:   []string{outputID.ToHex()},
		LedgerIndex: ledgerIndex,
	}, nil
}

func aliasesWithFilter(c echo.Context) (*outputsResponse, error) {
	maxResults := deps.RestAPILimitsMaxResults
	filters := []indexer.AliasFilterOption{indexer.AliasMaxResults(maxResults)}

	if len(c.QueryParam(QueryParameterStateController)) > 0 {
		stateController, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterStateController)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.AliasStateController(stateController))
	}

	if len(c.QueryParam(QueryParameterGovernanceController)) > 0 {
		governanceController, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterGovernanceController)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.AliasGovernanceController(governanceController))
	}

	if len(c.QueryParam(restapi.QueryParameterIssuer)) > 0 {
		issuer, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, restapi.QueryParameterIssuer)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.AliasIssuer(issuer))
	}

	if len(c.QueryParam(restapi.QueryParameterSender)) > 0 {
		sender, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, restapi.QueryParameterSender)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.AliasSender(sender))
	}

	outputIDs, ledgerIndex, err := deps.Indexer.AliasOutputsWithFilters(filters...)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading aliases failed: %s", err)
	}

	return &outputsResponse{
		MaxResults:  uint32(maxResults),
		Count:       uint32(len(outputIDs)),
		OutputIDs:   outputIDs.ToHex(),
		LedgerIndex: ledgerIndex,
	}, nil
}

func nftByID(c echo.Context) (*outputsResponse, error) {
	maxResults := deps.RestAPILimitsMaxResults

	nftID, err := restapi.ParseNFTIDParam(c)
	if err != nil {
		return nil, err
	}

	outputID, ledgerIndex, err := deps.Indexer.NFTOutput(nftID)
	if err != nil {
		if errors.Is(err, indexer.ErrNotFound) {
			return nil, errors.WithMessage(echo.ErrNotFound, "NFT not found")
		}
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading NFT failed: %s", err)
	}

	return &outputsResponse{
		MaxResults:  uint32(maxResults),
		Count:       1,
		OutputIDs:   []string{outputID.ToHex()},
		LedgerIndex: ledgerIndex,
	}, nil
}

func nftWithFilter(c echo.Context) (*outputsResponse, error) {
	maxResults := deps.RestAPILimitsMaxResults
	filters := []indexer.NFTFilterOption{indexer.NFTMaxResults(maxResults)}

	if len(c.QueryParam(restapi.QueryParameterAddress)) > 0 {
		address, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, restapi.QueryParameterAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTUnlockableByAddress(address))
	}

	if len(c.QueryParam(QueryParameterRequiresDustReturn)) > 0 {
		requiresDust, err := restapi.ParseBoolQueryParam(c, QueryParameterRequiresDustReturn)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTRequiresDustReturn(requiresDust))
	}

	if len(c.QueryParam(restapi.QueryParameterIssuer)) > 0 {
		issuer, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, restapi.QueryParameterIssuer)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTIssuer(issuer))
	}

	if len(c.QueryParam(restapi.QueryParameterSender)) > 0 {
		sender, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, restapi.QueryParameterSender)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTSender(sender))
	}

	if len(c.QueryParam(restapi.QueryParameterIndex)) > 0 {
		_, indexBytes, err := restapi.ParseIndexQueryParam(c)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTTag(indexBytes))
	}

	outputIDs, ledgerIndex, err := deps.Indexer.NFTOutputsWithFilters(filters...)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading NFTs failed: %s", err)
	}

	return &outputsResponse{
		MaxResults:  uint32(maxResults),
		Count:       uint32(len(outputIDs)),
		OutputIDs:   outputIDs.ToHex(),
		LedgerIndex: ledgerIndex,
	}, nil
}

func foundryByID(c echo.Context) (*outputsResponse, error) {
	maxResults := deps.RestAPILimitsMaxResults

	foundryID, err := restapi.ParseFoundryIDParam(c)
	if err != nil {
		return nil, err
	}

	outputID, ledgerIndex, err := deps.Indexer.FoundryOutput(foundryID)
	if err != nil {
		if errors.Is(err, indexer.ErrNotFound) {
			return nil, errors.WithMessage(echo.ErrNotFound, "foundry not found")
		}
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading foundry failed: %s", err)
	}

	return &outputsResponse{
		MaxResults:  uint32(maxResults),
		Count:       1,
		OutputIDs:   []string{outputID.ToHex()},
		LedgerIndex: ledgerIndex,
	}, nil
}

func foundriesWithFilter(c echo.Context) (*outputsResponse, error) {
	maxResults := deps.RestAPILimitsMaxResults
	filters := []indexer.FoundryFilterOption{indexer.FoundryMaxResults(maxResults)}

	if len(c.QueryParam(restapi.QueryParameterAddress)) > 0 {
		address, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, restapi.QueryParameterAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.FoundryUnlockableByAddress(address))
	}

	outputIDs, ledgerIndex, err := deps.Indexer.FoundryOutputsWithFilters(filters...)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading foundries failed: %s", err)
	}

	return &outputsResponse{
		MaxResults:  uint32(maxResults),
		Count:       uint32(len(outputIDs)),
		OutputIDs:   outputIDs.ToHex(),
		LedgerIndex: ledgerIndex,
	}, nil
}
