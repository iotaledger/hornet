package indexer

import (
	"encoding/hex"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"net/http"
	"strconv"

	"github.com/gohornet/hornet/pkg/indexer"
	"github.com/gohornet/hornet/pkg/restapi"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (

	// RouteOutputs is the route for getting outputs filtered by the given parameters.
	// GET with query parameter returns all outputIDs that fit these filter criteria.
	// Query parameters: "address", "hasDustReturnCondition", "dustReturnAddress", "hasExpirationCondition",
	//					 "expiresBefore", "expiresAfter", "expiresBeforeMilestone", "expiresAfterMilestone",
	//					 "hasTimelockCondition", "timelockedBefore", "timelockedAfter", "timelockedBeforeMilestone",
	//					 "timelockedAfterMilestone", "sender", "tag", "createdBefore", "createdAfter"
	// Returns an empty list if no results are found.
	RouteOutputs = "/outputs"

	// RouteAliases is the route for getting aliases filtered by the given parameters.
	// GET with query parameter returns all outputIDs that fit these filter criteria.
	// Query parameters: "stateController", "governor", "issuer", "sender", "createdBefore", "createdAfter"
	// Returns an empty list if no results are found.
	RouteAliases = "/aliases"

	// RouteAliasByID is the route for getting aliases by their aliasID.
	// GET returns the outputIDs or 404 if no record is found.
	RouteAliasByID = "/aliases/:" + restapi.ParameterAliasID

	// RouteNFTs is the route for getting NFT filtered by the given parameters.
	// Query parameters: "address", "hasDustReturnCondition", "dustReturnAddress", "hasExpirationCondition",
	//					 "expiresBefore", "expiresAfter", "expiresBeforeMilestone", "expiresAfterMilestone",
	//					 "hasTimelockCondition", "timelockedBefore", "timelockedAfter", "timelockedBeforeMilestone",
	//					 "timelockedAfterMilestone", "issuer", "sender", "tag", "createdBefore", "createdAfter"
	// Returns an empty list if no results are found.
	RouteNFTs = "/nfts"

	// RouteNFTByID is the route for getting NFT by their nftID.
	// GET returns the outputIDs or 404 if no record is found.
	RouteNFTByID = "/nfts/:" + restapi.ParameterNFTID

	// RouteFoundries is the route for getting foundries filtered by the given parameters.
	// GET with query parameter returns all outputIDs that fit these filter criteria.
	// Query parameters: "address", "createdBefore", "createdAfter"
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

	// QueryParameterHasDustReturnCondition is used to filter for outputs having a dust return unlock condition.
	QueryParameterHasDustReturnCondition = "hasDustReturnCondition"

	// QueryParameterDustReturnAddress is used to filter for outputs with a certain dust return address.
	QueryParameterDustReturnAddress = "dustReturnAddress"

	// QueryParameterHasExpirationCondition is used to filter for outputs having an expiration unlock condition.
	QueryParameterHasExpirationCondition = "hasExpirationCondition"

	// QueryParameterExpiresBefore is used to filter for outputs that expire before a certain unix time.
	QueryParameterExpiresBefore = "expiresBefore"

	// QueryParameterExpiresAfter is used to filter for outputs that expire after a certain unix time.
	QueryParameterExpiresAfter = "expiresAfter"

	// QueryParameterExpiresBeforeMilestone is used to filter for outputs that expire before a certain milestone index.
	QueryParameterExpiresBeforeMilestone = "expiresBeforeMilestone"

	// QueryParameterExpiresAfterMilestone is used to filter for outputs that expire after a certain milestone index.
	QueryParameterExpiresAfterMilestone = "expiresAfterMilestone"

	// QueryParameterExpirationReturnAddress is used to filter for outputs with a certain expiration return address.
	QueryParameterExpirationReturnAddress = "expirationReturnAddress"

	// QueryParameterHasTimelockCondition is used to filter for outputs having a timelock unlock condition.
	QueryParameterHasTimelockCondition = "hasTimelockCondition"

	// QueryParameterTimelockedBefore is used to filter for outputs that are timelocked before a certain unix time.
	QueryParameterTimelockedBefore = "timelockedBefore"

	// QueryParameterTimelockedAfter is used to filter for outputs that are timelocked after a certain unix time.
	QueryParameterTimelockedAfter = "timelockedAfter"

	// QueryParameterTimelockedBeforeMilestone is used to filter for outputs that are timelocked before a certain milestone index.
	QueryParameterTimelockedBeforeMilestone = "timelockedBeforeMilestone"

	// QueryParameterTimelockedAfterMilestone is used to filter for outputs that are timelocked after a certain milestone index.
	QueryParameterTimelockedAfterMilestone = "timelockedAfterMilestone"

	// QueryParameterStateController is used to filter for a certain state controller address.
	QueryParameterStateController = "stateController"

	// QueryParameterGovernor is used to filter for a certain governance controller address.
	QueryParameterGovernor = "governor"

	// QueryParameterPageSize is used to define the page size for the results.
	QueryParameterPageSize = "pageSize"

	// QueryParameterCursor is used to pass the offset we want to start the next results from.
	QueryParameterCursor = "cursor"

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

	routeGroup.GET(RouteNFTs, func(c echo.Context) error {
		resp, err := nftsWithFilter(c)
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
		value, err := restapi.ParseBoolQueryParam(c, QueryParameterHasDustReturnCondition)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputHasDustReturnCondition(value))
	}

	if len(c.QueryParam(QueryParameterDustReturnAddress)) > 0 {
		addr, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterDustReturnAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputDustReturnAddress(addr))
	}

	if len(c.QueryParam(QueryParameterHasExpirationCondition)) > 0 {
		value, err := restapi.ParseBoolQueryParam(c, QueryParameterHasExpirationCondition)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputHasExpirationCondition(value))
	}

	if len(c.QueryParam(QueryParameterExpirationReturnAddress)) > 0 {
		addr, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterExpirationReturnAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputExpirationReturnAddress(addr))
	}

	if len(c.QueryParam(QueryParameterExpiresBefore)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterExpiresBefore)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputExpiresBefore(timestamp))
	}

	if len(c.QueryParam(QueryParameterExpiresAfter)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterExpiresAfter)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputExpiresAfter(timestamp))
	}

	if len(c.QueryParam(QueryParameterExpiresBeforeMilestone)) > 0 {
		msIndex, err := restapi.ParseMilestoneIndexQueryParam(c, QueryParameterExpiresBeforeMilestone)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputExpiresBeforeMilestone(msIndex))
	}

	if len(c.QueryParam(QueryParameterExpiresAfterMilestone)) > 0 {
		msIndex, err := restapi.ParseMilestoneIndexQueryParam(c, QueryParameterExpiresAfterMilestone)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputExpiresAfterMilestone(msIndex))
	}

	if len(c.QueryParam(QueryParameterHasTimelockCondition)) > 0 {
		value, err := restapi.ParseBoolQueryParam(c, QueryParameterHasTimelockCondition)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputHasTimelockCondition(value))
	}

	if len(c.QueryParam(QueryParameterTimelockedBefore)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterTimelockedBefore)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputTimelockedBefore(timestamp))
	}

	if len(c.QueryParam(QueryParameterTimelockedAfter)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterTimelockedAfter)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputTimelockedAfter(timestamp))
	}

	if len(c.QueryParam(QueryParameterTimelockedBeforeMilestone)) > 0 {
		msIndex, err := restapi.ParseMilestoneIndexQueryParam(c, QueryParameterTimelockedBeforeMilestone)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputTimelockedBeforeMilestone(msIndex))
	}

	if len(c.QueryParam(QueryParameterTimelockedAfterMilestone)) > 0 {
		msIndex, err := restapi.ParseMilestoneIndexQueryParam(c, QueryParameterTimelockedAfterMilestone)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputTimelockedAfterMilestone(msIndex))
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

	if len(c.QueryParam(QueryParameterCursor)) > 0 {
		offset, err := restapi.ParseHexQueryParam(c, QueryParameterCursor, 38)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputCursor(offset))
	}

	if len(c.QueryParam(QueryParameterCreatedBefore)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterCreatedBefore)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.ExtendedOutputCreatedBefore(timestamp))
	}

	if len(c.QueryParam(QueryParameterCreatedAfter)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterCreatedAfter)
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

	if len(c.QueryParam(QueryParameterCursor)) > 0 {
		offset, err := restapi.ParseHexQueryParam(c, QueryParameterCursor, indexer.OffsetLength)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.AliasCursor(offset))
	}

	if len(c.QueryParam(QueryParameterCreatedBefore)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterCreatedBefore)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.AliasCreatedBefore(timestamp))
	}

	if len(c.QueryParam(QueryParameterCreatedAfter)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterCreatedAfter)
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

func nftsWithFilter(c echo.Context) (*outputsResponse, error) {
	filters := []indexer.NFTFilterOption{indexer.NFTPageSize(pageSizeFromContext(c))}

	if len(c.QueryParam(QueryParameterAddress)) > 0 {
		addr, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTUnlockableByAddress(addr))
	}

	if len(c.QueryParam(QueryParameterHasDustReturnCondition)) > 0 {
		value, err := restapi.ParseBoolQueryParam(c, QueryParameterHasDustReturnCondition)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTHasDustReturnCondition(value))
	}

	if len(c.QueryParam(QueryParameterDustReturnAddress)) > 0 {
		addr, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterDustReturnAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTDustReturnAddress(addr))
	}

	if len(c.QueryParam(QueryParameterHasExpirationCondition)) > 0 {
		value, err := restapi.ParseBoolQueryParam(c, QueryParameterHasExpirationCondition)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTHasExpirationCondition(value))
	}

	if len(c.QueryParam(QueryParameterExpirationReturnAddress)) > 0 {
		addr, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, QueryParameterExpirationReturnAddress)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTExpirationReturnAddress(addr))
	}

	if len(c.QueryParam(QueryParameterExpiresBefore)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterExpiresBefore)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTExpiresBefore(timestamp))
	}

	if len(c.QueryParam(QueryParameterExpiresAfter)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterExpiresAfter)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTExpiresAfter(timestamp))
	}

	if len(c.QueryParam(QueryParameterExpiresBeforeMilestone)) > 0 {
		msIndex, err := restapi.ParseMilestoneIndexQueryParam(c, QueryParameterExpiresBeforeMilestone)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTExpiresBeforeMilestone(msIndex))
	}

	if len(c.QueryParam(QueryParameterExpiresAfterMilestone)) > 0 {
		msIndex, err := restapi.ParseMilestoneIndexQueryParam(c, QueryParameterExpiresAfterMilestone)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTExpiresAfterMilestone(msIndex))
	}

	if len(c.QueryParam(QueryParameterHasTimelockCondition)) > 0 {
		value, err := restapi.ParseBoolQueryParam(c, QueryParameterHasTimelockCondition)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTHasTimelockCondition(value))
	}

	if len(c.QueryParam(QueryParameterTimelockedBefore)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterTimelockedBefore)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTTimelockedBefore(timestamp))
	}

	if len(c.QueryParam(QueryParameterTimelockedAfter)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterTimelockedAfter)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTTimelockedAfter(timestamp))
	}

	if len(c.QueryParam(QueryParameterTimelockedBeforeMilestone)) > 0 {
		msIndex, err := restapi.ParseMilestoneIndexQueryParam(c, QueryParameterTimelockedBeforeMilestone)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTTimelockedBeforeMilestone(msIndex))
	}

	if len(c.QueryParam(QueryParameterTimelockedAfterMilestone)) > 0 {
		msIndex, err := restapi.ParseMilestoneIndexQueryParam(c, QueryParameterTimelockedAfterMilestone)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTTimelockedAfterMilestone(msIndex))
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

	if len(c.QueryParam(QueryParameterCursor)) > 0 {
		offset, err := restapi.ParseHexQueryParam(c, QueryParameterCursor, indexer.OffsetLength)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTCursor(offset))
	}

	if len(c.QueryParam(QueryParameterCreatedBefore)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterCreatedBefore)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.NFTCreatedBefore(timestamp))
	}

	if len(c.QueryParam(QueryParameterCreatedAfter)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterCreatedAfter)
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

	if len(c.QueryParam(QueryParameterCursor)) > 0 {
		offset, err := restapi.ParseHexQueryParam(c, QueryParameterCursor, indexer.OffsetLength)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.FoundryCursor(offset))
	}

	if len(c.QueryParam(QueryParameterCreatedBefore)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterCreatedBefore)
		if err != nil {
			return nil, err
		}
		filters = append(filters, indexer.FoundryCreatedBefore(timestamp))
	}

	if len(c.QueryParam(QueryParameterCreatedAfter)) > 0 {
		timestamp, err := restapi.ParseUnixTimestampQueryParam(c, QueryParameterCreatedAfter)
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
		PageSize:    uint32(result.PageSize),
		Cursor:      hex.EncodeToString(result.Cursor),
		Items:       result.OutputIDs.ToHex(),
	}, nil
}

func pageSizeFromContext(c echo.Context) int {
	pageSize := deps.RestAPILimitsMaxResults
	if len(c.QueryParam(QueryParameterPageSize)) > 0 {
		i, err := strconv.Atoi(c.QueryParam(QueryParameterPageSize))
		if err != nil {
			return pageSize
		}
		if i > 0 && i < pageSize {
			pageSize = i
		}
	}
	return pageSize
}
