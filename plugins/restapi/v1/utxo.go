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
		MessageID:     output.MessageID().ToHex(),
		TransactionID: hex.EncodeToString(transactionID[:]),
		Spent:         spent,
		OutputIndex:   output.OutputID().Index(),
		RawOutput:     &rawRawOutputJSON,
		LedgerIndex:   ledgerIndex,
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

func outputsResponse(address iotago.Address, filterType *iotago.OutputType) (*addressOutputsResponse, error) {
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

	return &addressOutputsResponse{
		AddressType: address.Type(),
		Address:     address.String(),
		MaxResults:  uint32(maxResults),
		Count:       uint32(len(outputIDs)),
		OutputIDs:   outputIDs,
		LedgerIndex: ledgerIndex,
	}, nil
}

func outputsIDsByBech32Address(c echo.Context) (*addressOutputsResponse, error) {

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
	return outputsResponse(bech32Address, filteredType)
}

func outputsIDsByEd25519Address(c echo.Context) (*addressOutputsResponse, error) {

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
	return outputsResponse(address, filteredType)
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
