package v1

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go/v2"
)

func NewOutputResponse(output *utxo.Output, spent bool, ledgerIndex milestone.Index) (*OutputResponse, error) {

	var rawOutput iotago.Output
	switch output.OutputType() {
	case iotago.OutputSigLockedSingleOutput:
		rawOutput = &iotago.SigLockedSingleOutput{
			Address: output.Address(),
			Amount:  output.Amount(),
		}
	case iotago.OutputSigLockedDustAllowanceOutput:
		rawOutput = &iotago.SigLockedDustAllowanceOutput{
			Address: output.Address(),
			Amount:  output.Amount(),
		}
	default:
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "unsupported output type: %d", output.OutputType())
	}

	rawOutputJSON, err := rawOutput.MarshalJSON()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "marshaling output failed: %s, error: %s", output.OutputID().ToHex(), err)
	}

	rawRawOutputJSON := json.RawMessage(rawOutputJSON)

	return &OutputResponse{
		MessageID:     output.MessageID().ToHex(),
		TransactionID: hex.EncodeToString(output.OutputID()[:iotago.TransactionIDLength]),
		Spent:         spent,
		OutputIndex:   binary.LittleEndian.Uint16(output.OutputID()[iotago.TransactionIDLength : iotago.TransactionIDLength+iotago.UInt16ByteSize]),
		RawOutput:     &rawRawOutputJSON,
		LedgerIndex:   ledgerIndex,
	}, nil
}

func outputByID(c echo.Context) (*OutputResponse, error) {
	outputIDParam := strings.ToLower(c.Param(ParameterOutputID))

	outputIDBytes, err := hex.DecodeString(outputIDParam)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid output ID: %s, error: %s", outputIDParam, err)
	}

	if len(outputIDBytes) != utxo.OutputIDLength {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid output ID: %s, error: %s", outputIDParam, err)
	}

	var outputID iotago.UTXOInputID
	copy(outputID[:], outputIDBytes)

	// we need to lock the ledger here to have the correct index for unspent info of the output.
	deps.UTXO.ReadLockLedger()
	defer deps.UTXO.ReadUnlockLedger()

	ledgerIndex, err := deps.UTXO.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading output failed: %s, error: %s", outputIDParam, err)
	}

	output, err := deps.UTXO.ReadOutputByOutputIDWithoutLocking(&outputID)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "output not found: %s", outputIDParam)
		}
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading output failed: %s, error: %s", outputIDParam, err)
	}

	unspent, err := deps.UTXO.IsOutputUnspentWithoutLocking(output)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading spent status failed: %s, error: %s", outputIDParam, err)
	}

	return NewOutputResponse(output, !unspent, ledgerIndex)
}

func ed25519Balance(address *iotago.Ed25519Address) (*addressBalanceResponse, error) {

	balance, dustAllowed, ledgerIndex, err := deps.UTXO.AddressBalance(address)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading address balance failed: %s, error: %s", address, err)
	}

	return &addressBalanceResponse{
		AddressType: address.Type(),
		Address:     address.String(),
		Balance:     balance,
		DustAllowed: dustAllowed,
		LedgerIndex: ledgerIndex,
	}, nil
}

func balanceByBech32Address(c echo.Context) (*addressBalanceResponse, error) {

	if !deps.Storage.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is not synced")
	}

	addressParam := strings.ToLower(c.Param(ParameterAddress))

	_, bech32Address, err := iotago.ParseBech32(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address: %s, error: %s", addressParam, err)
	}

	switch address := bech32Address.(type) {
	case *iotago.Ed25519Address:
		return ed25519Balance(address)
	default:
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address: %s, error: unknown address type", addressParam)
	}
}

func balanceByEd25519Address(c echo.Context) (*addressBalanceResponse, error) {

	if !deps.Storage.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is not synced")
	}

	addressParam := strings.ToLower(c.Param(ParameterAddress))

	addressBytes, err := hex.DecodeString(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address: %s, error: %s", addressParam, err)
	}

	if len(addressBytes) != (iotago.Ed25519AddressBytesLength) {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address length: %s", addressParam)
	}

	var address iotago.Ed25519Address
	copy(address[:], addressBytes)

	return ed25519Balance(&address)
}

func outputsResponse(address iotago.Address, includeSpent bool, filterType *iotago.OutputType) (*addressOutputsResponse, error) {
	maxResults := deps.RestAPILimitsMaxResults

	opts := []utxo.UTXOIterateOption{
		utxo.FilterAddress(address),
		utxo.ReadLockLedger(false),
	}

	if filterType != nil {
		opts = append(opts, utxo.FilterOutputType(*filterType))
	}

	// we need to lock the ledger here to have the same index for unspent and spent outputs.
	deps.UTXO.ReadLockLedger()
	defer deps.UTXO.ReadUnlockLedger()

	ledgerIndex, err := deps.UTXO.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading unspent outputs failed: %s, error: %s", address, err)
	}

	unspentOutputs, err := deps.UTXO.UnspentOutputs(append(opts, utxo.MaxResultCount(maxResults))...)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading unspent outputs failed: %s, error: %s", address, err)
	}

	outputIDs := make([]string, len(unspentOutputs))
	for i, unspentOutput := range unspentOutputs {
		outputIDs[i] = unspentOutput.OutputID().ToHex()
	}

	if includeSpent && maxResults-len(outputIDs) > 0 {

		spents, err := deps.UTXO.SpentOutputs(append(opts, utxo.MaxResultCount(maxResults-len(outputIDs)))...)
		if err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading spent outputs failed: %s, error: %s", address, err)
		}

		outputIDsSpent := make([]string, len(spents))
		for i, spent := range spents {
			outputIDsSpent[i] = spent.OutputID().ToHex()
		}

		outputIDs = append(outputIDs, outputIDsSpent...)
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

	if !deps.Storage.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is not synced")
	}

	// error is ignored because it returns false in case it can't be parsed
	includeSpent, _ := strconv.ParseBool(strings.ToLower(c.QueryParam("include-spent")))

	typeParam := strings.ToLower(c.QueryParam("type"))
	var filteredType *iotago.OutputType

	if len(typeParam) > 0 {
		outputTypeInt, err := strconv.ParseInt(typeParam, 10, 32)
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid type: %s, error: unknown output type", typeParam)
		}
		outputType := iotago.OutputType(outputTypeInt)
		if outputType != iotago.OutputSigLockedSingleOutput && outputType != iotago.OutputSigLockedDustAllowanceOutput {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid type: %s, error: unknown output type", typeParam)
		}
		filteredType = &outputType
	}

	addressParam := strings.ToLower(c.Param(ParameterAddress))
	_, bech32Address, err := iotago.ParseBech32(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address: %s, error: %s", addressParam, err)
	}

	return outputsResponse(bech32Address, includeSpent, filteredType)
}

func outputsIDsByEd25519Address(c echo.Context) (*addressOutputsResponse, error) {

	if !deps.Storage.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is not synced")
	}

	// error is ignored because it returns false in case it can't be parsed
	includeSpent, _ := strconv.ParseBool(strings.ToLower(c.QueryParam("include-spent")))

	var filteredType *iotago.OutputType
	typeParam := strings.ToLower(c.QueryParam("type"))
	if len(typeParam) > 0 {
		outputTypeInt, err := strconv.ParseInt(typeParam, 10, 32)
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid type: %s, error: unknown output type", typeParam)
		}
		outputType := iotago.OutputType(outputTypeInt)
		if outputType != iotago.OutputSigLockedSingleOutput && outputType != iotago.OutputSigLockedDustAllowanceOutput {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid type: %s, error: unknown output type", typeParam)
		}
		filteredType = &outputType
	}

	addressParam := strings.ToLower(c.Param(ParameterAddress))
	addressBytes, err := hex.DecodeString(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address: %s, error: %s", addressParam, err)
	}

	if len(addressBytes) != (iotago.Ed25519AddressBytesLength) {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address length: %s", addressParam)
	}

	var address iotago.Ed25519Address
	copy(address[:], addressBytes)

	return outputsResponse(&address, includeSpent, filteredType)
}

func treasury(_ echo.Context) (*treasuryResponse, error) {

	treasuryOutput, err := deps.UTXO.UnspentTreasuryOutputWithoutLocking()
	if err != nil {
		return nil, err
	}

	return &treasuryResponse{
		MilestoneID: hex.EncodeToString(treasuryOutput.MilestoneID[:]),
		Amount:      treasuryOutput.Amount,
	}, nil
}
