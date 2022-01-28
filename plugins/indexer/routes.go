package indexer

import (
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/indexer"
	"github.com/gohornet/hornet/pkg/restapi"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (

	// RouteOutputs is the route for getting outputs filtered by the given parameters.
	// GET with query parameter returns all outputIDs that fit these filter criteria (query parameters: "address", "requiresDustReturn", "sender", "tag").
	// Returns an empty list if no results are found.
	RouteOutputs = "/outputs"

	// RouteAliases is the route for getting aliases filtered by the given parameters.
	// GET with query parameter returns all outputIDs that fit these filter criteria (query parameters: "stateController", "governor", "issuer", "sender").
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

	// QueryParameterAddress is used to filter for a certain address.
	QueryParameterAddress = "address"

	// QueryParameterIssuer is used to filter for a certain issuer.
	QueryParameterIssuer = "issuer"

	// QueryParameterSender is used to filter for a certain sender.
	QueryParameterSender = "sender"

	// QueryParameterTag is used to filter for a certain tag.
	QueryParameterTag = "tag"

	// QueryParameterHasDustReturnCondition is used to filter for outputs requiring a dust return.
	QueryParameterHasDustReturnCondition = "hasDustReturnCondition"

	// QueryParameterDustReturnAddress is used to filter for outputs with a certain dust return address.
	QueryParameterDustReturnAddress = "dustReturnAddress"

	// QueryParameterExpirationReturnAddress is used to filter for outputs with a certain expiration return address.
	QueryParameterExpirationReturnAddress = "expirationReturnAddress"

	// QueryParameterStateController is used to filter for a certain state controller address.
	QueryParameterStateController = "stateController"

	// QueryParameterGovernor is used to filter for a certain governance controller address.
	QueryParameterGovernor = "governor"

	// QueryParameterLimit is used to define the page size for the results.
	QueryParameterLimit = "limit"

	// QueryParameterOffset is used to pass the outputID we want to start the results from.
	QueryParameterOffset = "offset"

	// QueryParameterCreatedBefore is used to filter for outputs that were created before the given time.
	QueryParameterCreatedBefore = "createdBefore"

	// QueryParameterCreatedAfter is used to filter for outputs that were created after the given time.
	QueryParameterCreatedAfter = "createdAfter"
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

		return c.JSON(http.StatusOK, resp)
	})

	routeGroup.GET(RouteAliases, func(c echo.Context) error {
		resp, err := aliasesWithFilter(c)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, resp)
	})

	routeGroup.GET(RouteAliasByID, func(c echo.Context) error {
		resp, err := aliasByID(c)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, resp)
	})

	routeGroup.GET(RouteNFT, func(c echo.Context) error {
		resp, err := nftWithFilter(c)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, resp)
	})

	routeGroup.GET(RouteNFTByID, func(c echo.Context) error {
		resp, err := nftByID(c)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, resp)
	})

	routeGroup.GET(RouteFoundries, func(c echo.Context) error {
		resp, err := foundriesWithFilter(c)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, resp)
	})

	routeGroup.GET(RouteFoundryByID, func(c echo.Context) error {
		resp, err := foundryByID(c)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, resp)
	})
}

func outputsWithFilter(c echo.Context) (*outputsResponse, error) {
	filters := []indexer.ExtendedOutputFilterOption{indexer.ExtendedOutputPageSize(pageSizeFromContext(c))}

	if len(c.QueryParam(QueryParameterAddress)) > 0 {
		addr, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputUnlockableByAddress(addr))
	}

	if len(c.QueryParam(QueryParameterHasDustReturnCondition)) > 0 {
		requiresDust, err := restapi.ParseBoolQueryParam(c, QueryParameterHasDustReturnCondition)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputHasDustReturnCondition(requiresDust))
	}

	if len(c.QueryParam(QueryParameterDustReturnAddress)) > 0 {
		addr, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterDustReturnAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputDustReturnAddress(addr))
	}

	if len(c.QueryParam(QueryParameterExpirationReturnAddress)) > 0 {
		addr, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterExpirationReturnAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputExpirationReturnAddress(addr))
	}

	if len(c.QueryParam(QueryParameterSender)) > 0 {
		addr, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterSender)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputSender(addr))
	}

	if len(c.QueryParam(QueryParameterTag)) > 0 {
		tagBytes, err := restapi.ParseHexQueryParam(c, QueryParameterTag, iotago.MaxTagLength)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputTag(tagBytes))
	}

	if len(c.QueryParam(QueryParameterOffset)) > 0 {
		offset, err := restapi.ParseHexQueryParam(c, QueryParameterOffset, 38)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputOffset(offset))
	}

	if len(c.QueryParam(QueryParameterCreatedBefore)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampParam(c, QueryParameterCreatedBefore)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputCreatedBefore(timestamp))
	}

	if len(c.QueryParam(QueryParameterCreatedAfter)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampParam(c, QueryParameterCreatedAfter)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputCreatedAfter(timestamp))
	}

	return outputsResponseFromResult(deps.Indexer.ExtendedOutputsWithFilters(filters...))
}

func aliasByID(c echo.Context) (*outputsResponse, error) {
	aliasID, err := restapi.ParseAliasIDParam(c)
	if err != nil {
		return nil, err
	}
	return singleOutputResponseFromResult(deps.Indexer.AliasOutput(aliasID))
}

func aliasesWithFilter(c echo.Context) (*outputsResponse, error) {
	filters := []indexer.AliasFilterOption{indexer.AliasPageSize(pageSizeFromContext(c))}

	if len(c.QueryParam(QueryParameterStateController)) > 0 {
		stateController, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterStateController)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.AliasStateController(stateController))
	}

	if len(c.QueryParam(QueryParameterGovernor)) > 0 {
		governor, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterGovernor)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.AliasGovernor(governor))
	}

	if len(c.QueryParam(QueryParameterIssuer)) > 0 {
		issuer, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterIssuer)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.AliasIssuer(issuer))
	}

	if len(c.QueryParam(QueryParameterSender)) > 0 {
		sender, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterSender)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.AliasSender(sender))
	}

	if len(c.QueryParam(QueryParameterOffset)) > 0 {
		offset, err := restapi.ParseHexQueryParam(c, QueryParameterOffset, indexer.OffsetLength)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.AliasOffset(offset))
	}

	if len(c.QueryParam(QueryParameterCreatedBefore)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampParam(c, QueryParameterCreatedBefore)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.AliasCreatedBefore(timestamp))
	}

	if len(c.QueryParam(QueryParameterCreatedAfter)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampParam(c, QueryParameterCreatedAfter)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.AliasCreatedAfter(timestamp))
	}

	return outputsResponseFromResult(deps.Indexer.AliasOutputsWithFilters(filters...))
}

func nftByID(c echo.Context) (*outputsResponse, error) {
	nftID, err := restapi.ParseNFTIDParam(c)
	if err != nil {
		return nil, err
	}
	return singleOutputResponseFromResult(deps.Indexer.NFTOutput(nftID))
}

func nftWithFilter(c echo.Context) (*outputsResponse, error) {
	filters := []indexer.NFTFilterOption{indexer.NFTPageSize(pageSizeFromContext(c))}

	if len(c.QueryParam(QueryParameterAddress)) > 0 {
		addr, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTUnlockableByAddress(addr))
	}

	if len(c.QueryParam(QueryParameterHasDustReturnCondition)) > 0 {
		requiresDust, err := restapi.ParseBoolQueryParam(c, QueryParameterHasDustReturnCondition)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTHasDustReturnCondition(requiresDust))
	}

	if len(c.QueryParam(QueryParameterDustReturnAddress)) > 0 {
		addr, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterDustReturnAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTDustReturnAddress(addr))
	}

	if len(c.QueryParam(QueryParameterExpirationReturnAddress)) > 0 {
		addr, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterExpirationReturnAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTExpirationReturnAddress(addr))
	}

	if len(c.QueryParam(QueryParameterIssuer)) > 0 {
		addr, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterIssuer)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTIssuer(addr))
	}

	if len(c.QueryParam(QueryParameterSender)) > 0 {
		addr, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterSender)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTSender(addr))
	}

	if len(c.QueryParam(QueryParameterTag)) > 0 {
		tagBytes, err := restapi.ParseHexQueryParam(c, QueryParameterTag, iotago.MaxTagLength)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTTag(tagBytes))
	}

	if len(c.QueryParam(QueryParameterOffset)) > 0 {
		offset, err := restapi.ParseHexQueryParam(c, QueryParameterOffset, indexer.OffsetLength)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTOffset(offset))
	}

	if len(c.QueryParam(QueryParameterCreatedBefore)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampParam(c, QueryParameterCreatedBefore)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTCreatedBefore(timestamp))
	}

	if len(c.QueryParam(QueryParameterCreatedAfter)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampParam(c, QueryParameterCreatedAfter)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTCreatedAfter(timestamp))
	}

	return outputsResponseFromResult(deps.Indexer.NFTOutputsWithFilters(filters...))
}

func foundryByID(c echo.Context) (*outputsResponse, error) {
	foundryID, err := restapi.ParseFoundryIDParam(c)
	if err != nil {
		return nil, err
	}
	return singleOutputResponseFromResult(deps.Indexer.FoundryOutput(foundryID))
}

func foundriesWithFilter(c echo.Context) (*outputsResponse, error) {
	filters := []indexer.FoundryFilterOption{indexer.FoundryPageSize(pageSizeFromContext(c))}

	if len(c.QueryParam(QueryParameterAddress)) > 0 {
		address, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.FoundryUnlockableByAddress(address))
	}

	if len(c.QueryParam(QueryParameterOffset)) > 0 {
		offset, err := restapi.ParseHexQueryParam(c, QueryParameterOffset, indexer.OffsetLength)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.FoundryOffset(offset))
	}

	if len(c.QueryParam(QueryParameterCreatedBefore)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampParam(c, QueryParameterCreatedBefore)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.FoundryCreatedBefore(timestamp))
	}

	if len(c.QueryParam(QueryParameterCreatedAfter)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampParam(c, QueryParameterCreatedAfter)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.FoundryCreatedAfter(timestamp))
	}

	return outputsResponseFromResult(deps.Indexer.FoundryOutputsWithFilters(filters...))
}

func singleOutputResponseFromResult(result *indexer.IndexerResult) (*outputsResponse, error) {
	if result.Error != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading outputIDs failed: %s", result.Error)
	}
	if len(result.OutputIDs) == 0 {
		return nil, errors.WithMessage(echo.ErrNotFound, "record not found")
	}
	return outputsResponseFromResult(result)
}

func outputsResponseFromResult(result *indexer.IndexerResult) (*outputsResponse, error) {
	if result.Error != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading outputIDs failed: %s", result.Error)
	}

	return &outputsResponse{
		LedgerIndex: result.LedgerIndex,
		Limit:       uint32(result.PageSize),
		Offset:      hex.EncodeToString(result.NextOffset),
		Count:       uint32(len(result.OutputIDs)),
		OutputIDs:   result.OutputIDs.ToHex(),
	}, nil
}

func pageSizeFromContext(c echo.Context) int {
	pageSize := deps.RestAPILimitsMaxResults
	if len(c.QueryParam(QueryParameterLimit)) > 0 {
		i, err := strconv.Atoi(c.QueryParam(QueryParameterLimit))
		if err != nil {
			return pageSize
		}
		if i > 0 && i < pageSize {
			pageSize = i
		}
	}
	return pageSize
}
