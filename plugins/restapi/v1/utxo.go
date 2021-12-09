package v1

import (
	"encoding/hex"
	"encoding/json"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go/v3"
)

func NewOutputResponse(output *utxo.Output, spent bool, ledgerIndex milestone.Index) (*OutputResponse, error) {
	rawOutputJSON, err := output.Output().MarshalJSON()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "marshaling output failed: %s, error: %s", output.OutputID().ToHex(), err)
	}

	rawRawOutputJSON := json.RawMessage(rawOutputJSON)

	transactionID := output.OutputID().TransactionID()

	return &OutputResponse{
		MessageID:          output.MessageID().ToHex(),
		TransactionID:      hex.EncodeToString(transactionID[:]),
		Spent:              spent,
		OutputIndex:        output.OutputID().Index(),
		RawOutput:          &rawRawOutputJSON,
		MilestoneIndex:     output.MilestoneIndex(),
		MilestoneTimestamp: output.MilestoneTimestamp(),
		LedgerIndex:        ledgerIndex,
	}, nil
}

func outputByID(c echo.Context) (*OutputResponse, error) {
	outputID, err := restapi.ParseOutputIDParam(c)
	if err != nil {
		return nil, err
	}

	// we need to lock the ledger here to have the correct index for unspent info of the output.
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	ledgerIndex, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading output failed: %s, error: %s", outputID.ToHex(), err)
	}

	output, err := deps.UTXOManager.ReadOutputByOutputIDWithoutLocking(outputID)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "output not found: %s", outputID.ToHex())
		}
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading output failed: %s, error: %s", outputID.ToHex(), err)
	}

	unspent, err := deps.UTXOManager.IsOutputUnspentWithoutLocking(output)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading spent status failed: %s, error: %s", outputID.ToHex(), err)
	}

	return NewOutputResponse(output, !unspent, ledgerIndex)
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

func treasury(_ echo.Context) (*treasuryResponse, error) {

	treasuryOutput, err := deps.UTXOManager.UnspentTreasuryOutputWithoutLocking()
	if err != nil {
		return nil, err
	}

	return &treasuryResponse{
		MilestoneID: hex.EncodeToString(treasuryOutput.MilestoneID[:]),
		Amount:      treasuryOutput.Amount,
	}, nil
}
