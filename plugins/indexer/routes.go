package indexer

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	iotago "github.com/iotaledger/iota.go/v3"

	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/restapi"
)

const (

	// RouteOutputs is the route for getting outputs filtered by the given parameters.
	// GET with query parameter (mandatory) returns all output IDs that fit these filter criteria (query parameters: "issuer", "sender", "index").
	RouteOutputs = "/outputs"

	// RouteAddressBech32Outputs is the route for getting all output IDs for an address.
	// The address must be encoded in bech32.
	// GET returns the outputIDs for all outputs of this address.
	RouteAddressBech32Outputs = "/addresses/:" + restapi.ParameterAddress + "/outputs"

	// RouteAddressEd25519Outputs is the route for getting all output IDs for an ed25519 address.
	// The ed25519 address must be encoded in hex.
	// GET returns the outputIDs for all outputs of this address.
	RouteAddressEd25519Outputs = "/addresses/ed25519/:" + restapi.ParameterAddress + "/outputs"

	// RouteAddressAliasOutputs is the route for getting all output IDs for an alias address.
	// The alias address must be encoded in hex.
	// GET returns the outputIDs for all outputs of this address.
	RouteAddressAliasOutputs = "/addresses/alias/:" + restapi.ParameterAddress + "/outputs"

	// RouteAddressNFTOutputs is the route for getting all output IDs for a nft address.
	// The nft address must be encoded in hex.
	// GET returns the outputIDs for all outputs of this address.
	RouteAddressNFTOutputs = "/addresses/nft/:" + restapi.ParameterAddress + "/outputs"

	// RouteAlias is the route for getting aliases by their aliasID.
	// GET returns the outputIDs.
	RouteAlias = "/aliases/:" + restapi.ParameterAliasID

	// RouteNFT is the route for getting NFT by their nftID.
	// GET returns the outputIDs.
	RouteNFT = "/nft/:" + restapi.ParameterNFTID

	// RouteFoundry is the route for getting foundries by their foundryID.
	// GET returns the outputIDs.
	RouteFoundry = "/foundries/:" + restapi.ParameterFoundryID
)

func configureRoutes(routeGroup *echo.Group) {

	routeGroup.GET(RouteOutputs, func(c echo.Context) error {
		resp, err := outputs(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAddressBech32Outputs, func(c echo.Context) error {
		resp, err := outputsIDsByBech32Address(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAddressEd25519Outputs, func(c echo.Context) error {
		resp, err := outputsIDsByEd25519Address(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAddressAliasOutputs, func(c echo.Context) error {
		resp, err := outputsIDsByAliasAddress(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAddressNFTOutputs, func(c echo.Context) error {
		resp, err := outputsIDsByNFTAddress(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAlias, func(c echo.Context) error {
		resp, err := aliasByID(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteNFT, func(c echo.Context) error {
		resp, err := nftByID(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteFoundry, func(c echo.Context) error {
		resp, err := foundryByID(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})
}

func outputsByAddressResponse(address iotago.Address, filterType *iotago.OutputType) (*outputsResponse, error) {
	maxResults := deps.RestAPILimitsMaxResults

	var filter *utxo.FilterOptions
	if filterType != nil {
		filter = utxo.FilterOutputType(*filterType)
	}

	// we need to lock the ledger here to have the same index for unspent outputs.
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	ledgerIndex, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading unspent outputs failed: %s, error: %s", address, err)
	}

	outputIDs := make([]string, 0)
	if err := deps.UTXOManager.ForEachUnspentOutputOnAddress(address, filter, func(output *utxo.Output) bool {
		outputIDs = append(outputIDs, output.OutputID().ToHex())
		return true
	}, utxo.ReadLockLedger(false), utxo.MaxResultCount(maxResults)); err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading unspent outputs failed: %s, error: %s", address, err)
	}

	return &outputsResponse{
		MaxResults:  uint32(maxResults),
		Count:       uint32(len(outputIDs)),
		OutputIDs:   outputIDs,
		LedgerIndex: ledgerIndex,
	}, nil
}

func outputsIDsByBech32Address(c echo.Context) (*outputsResponse, error) {

	if !deps.SyncManager.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is not synced")
	}

	filteredType, err := restapi.ParseOutputTypeQueryParam(c)
	if err != nil {
		return nil, err
	}

	bech32Address, err := restapi.ParseBech32AddressParam(c, deps.Bech32HRP)
	if err != nil {
		return nil, err
	}
	return outputsByAddressResponse(bech32Address, filteredType)
}

func outputsIDsByEd25519Address(c echo.Context) (*outputsResponse, error) {

	if !deps.SyncManager.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is not synced")
	}

	filteredType, err := restapi.ParseOutputTypeQueryParam(c)
	if err != nil {
		return nil, err
	}

	address, err := restapi.ParseEd25519AddressParam(c)
	if err != nil {
		return nil, err
	}
	return outputsByAddressResponse(address, filteredType)
}

func outputsIDsByAliasAddress(c echo.Context) (*outputsResponse, error) {

	if !deps.SyncManager.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is not synced")
	}

	filteredType, err := restapi.ParseOutputTypeQueryParam(c)
	if err != nil {
		return nil, err
	}

	address, err := restapi.ParseAliasAddressParam(c)
	if err != nil {
		return nil, err
	}
	return outputsByAddressResponse(address, filteredType)
}

func outputsIDsByNFTAddress(c echo.Context) (*outputsResponse, error) {

	if !deps.SyncManager.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is not synced")
	}

	filteredType, err := restapi.ParseOutputTypeQueryParam(c)
	if err != nil {
		return nil, err
	}

	address, err := restapi.ParseNFTAddressParam(c)
	if err != nil {
		return nil, err
	}
	return outputsByAddressResponse(address, filteredType)
}

func outputs(c echo.Context) (*outputsResponse, error) {
	filterType, err := restapi.ParseOutputTypeQueryParam(c)
	if err != nil {
		return nil, err
	}

	if len(c.QueryParam(restapi.QueryParameterIssuer)) > 0 {
		issuer, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, restapi.QueryParameterIssuer)
		if err != nil {
			return nil, err
		}
		return outputsByIssuerResponse(issuer, filterType)
	}

	if len(c.QueryParam(restapi.QueryParameterSender)) > 0 {
		sender, err := restapi.ParseBech32AddressQueryParam(c, deps.Bech32HRP, restapi.QueryParameterSender)
		if err != nil {
			return nil, err
		}

		_, indexBytes, err := restapi.ParseIndexQueryParam(c)
		if err != nil {
			return nil, err
		}
		return outputsBySenderAndIndexResponse(sender, indexBytes, filterType)
	}

	return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "no %s or %s query parameter provided", restapi.QueryParameterIssuer, restapi.QueryParameterSender)
}

func outputsByIssuerResponse(issuer iotago.Address, filterType *iotago.OutputType) (*outputsResponse, error) {
	maxResults := deps.RestAPILimitsMaxResults

	var filter *utxo.FilterOptions
	if filterType != nil {
		filter = utxo.FilterOutputType(*filterType)
	}

	// we need to lock the ledger here to have the same index for unspent outputs.
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	ledgerIndex, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading outputs by issuer failed: %s", err)
	}

	outputIDs := make([]string, 0)
	if err := deps.UTXOManager.ForEachUnspentOutputWithIssuer(issuer, filter, func(output *utxo.Output) bool {
		outputIDs = append(outputIDs, output.OutputID().ToHex())
		return true
	}, utxo.ReadLockLedger(false), utxo.MaxResultCount(maxResults)); err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading outputs by issuer failed: %s", err)
	}

	return &outputsResponse{
		MaxResults:  uint32(maxResults),
		Count:       uint32(len(outputIDs)),
		OutputIDs:   outputIDs,
		LedgerIndex: ledgerIndex,
	}, nil
}

func outputsBySenderAndIndexResponse(sender iotago.Address, index []byte, filterType *iotago.OutputType) (*outputsResponse, error) {
	maxResults := deps.RestAPILimitsMaxResults

	var filter *utxo.FilterOptions
	if filterType != nil {
		filter = utxo.FilterOutputType(*filterType)
	}

	// we need to lock the ledger here to have the same index for unspent outputs.
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	ledgerIndex, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading outputs by sender failed: %s", err)
	}

	outputIDs := make([]string, 0)

	if len(index) > 0 {
		if err := deps.UTXOManager.ForEachUnspentOutputWithSenderAndIndexTag(sender, index, filter, func(output *utxo.Output) bool {
			outputIDs = append(outputIDs, output.OutputID().ToHex())
			return true
		}, utxo.ReadLockLedger(false), utxo.MaxResultCount(maxResults)); err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading outputs by sender and tag failed: %s", err)
		}
	} else {
		if err := deps.UTXOManager.ForEachUnspentOutputWithSender(sender, filter, func(output *utxo.Output) bool {
			outputIDs = append(outputIDs, output.OutputID().ToHex())
			return true
		}, utxo.ReadLockLedger(false), utxo.MaxResultCount(maxResults)); err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading outputs by sender failed: %s", err)
		}
	}

	return &outputsResponse{
		MaxResults:  uint32(maxResults),
		Count:       uint32(len(outputIDs)),
		OutputIDs:   outputIDs,
		LedgerIndex: ledgerIndex,
	}, nil
}

func aliasByID(c echo.Context) (*outputsResponse, error) {
	maxResults := deps.RestAPILimitsMaxResults

	aliasID, err := restapi.ParseAliasIDParam(c)
	if err != nil {
		return nil, err
	}

	// we need to lock the ledger here to have the correct index for unspent info of the output.
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	ledgerIndex, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading alias failed: %s, error: %s", aliasID.String(), err)
	}

	outputIDs := make([]string, 0)
	if err := deps.UTXOManager.ForEachUnspentAliasOutput(aliasID, func(output *utxo.Output) bool {
		outputIDs = append(outputIDs, output.OutputID().ToHex())
		return true
	}, utxo.ReadLockLedger(false), utxo.MaxResultCount(maxResults)); err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading alias failed: %s", err)
	}

	return &outputsResponse{
		MaxResults:  uint32(maxResults),
		Count:       uint32(len(outputIDs)),
		OutputIDs:   outputIDs,
		LedgerIndex: ledgerIndex,
	}, nil
}

func nftByID(c echo.Context) (*outputsResponse, error) {
	maxResults := deps.RestAPILimitsMaxResults

	nftID, err := restapi.ParseNFTIDParam(c)
	if err != nil {
		return nil, err
	}

	// we need to lock the ledger here to have the correct index for unspent info of the output.
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	ledgerIndex, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading NFT failed: %s, error: %s", nftID.String(), err)
	}

	outputIDs := make([]string, 0)
	if err := deps.UTXOManager.ForEachUnspentNFTOutput(nftID, func(output *utxo.Output) bool {
		outputIDs = append(outputIDs, output.OutputID().ToHex())
		return true
	}, utxo.ReadLockLedger(false), utxo.MaxResultCount(maxResults)); err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading NFT failed: %s", err)
	}

	return &outputsResponse{
		MaxResults:  uint32(maxResults),
		Count:       uint32(len(outputIDs)),
		OutputIDs:   outputIDs,
		LedgerIndex: ledgerIndex,
	}, nil
}

func foundryByID(c echo.Context) (*outputsResponse, error) {
	maxResults := deps.RestAPILimitsMaxResults

	foundryID, err := restapi.ParseFoundryIDParam(c)
	if err != nil {
		return nil, err
	}

	// we need to lock the ledger here to have the correct index for unspent info of the output.
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	ledgerIndex, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading foundry failed: %s, error: %s", foundryID.String(), err)
	}

	outputIDs := make([]string, 0)
	if err := deps.UTXOManager.ForEachUnspentFoundryOutput(foundryID, func(output *utxo.Output) bool {
		outputIDs = append(outputIDs, output.OutputID().ToHex())
		return true
	}, utxo.ReadLockLedger(false), utxo.MaxResultCount(maxResults)); err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading foundry failed: %s", err)
	}

	return &outputsResponse{
		MaxResults:  uint32(maxResults),
		Count:       uint32(len(outputIDs)),
		OutputIDs:   outputIDs,
		LedgerIndex: ledgerIndex,
	}, nil
}
