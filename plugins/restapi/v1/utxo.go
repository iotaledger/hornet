package v1

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/utxo"
	restapiplugin "github.com/gohornet/hornet/plugins/restapi"
)

func newOutputResponse(output *utxo.Output, spent bool) (*outputResponse, error) {

	sigLockedSingleDeposit := &iotago.SigLockedSingleOutput{
		Address: output.Address(),
		Amount:  output.Amount(),
	}

	sigLockedSingleDepositJSON, err := sigLockedSingleDeposit.MarshalJSON()
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "marshalling output failed: %s, error: %s", hex.EncodeToString(output.UTXOKey()), err)
	}

	rawMsgSigLockedSingleDepositJSON := json.RawMessage(sigLockedSingleDepositJSON)

	return &outputResponse{
		MessageID:     output.MessageID().Hex(),
		TransactionID: hex.EncodeToString(output.OutputID()[:iotago.TransactionIDLength]),
		Spent:         spent,
		OutputIndex:   binary.LittleEndian.Uint16(output.OutputID()[iotago.TransactionIDLength : iotago.TransactionIDLength+iotago.UInt16ByteSize]),
		RawOutput:     &rawMsgSigLockedSingleDepositJSON,
	}, nil
}

func outputByID(c echo.Context) (*outputResponse, error) {
	outputIDParam := strings.ToLower(c.Param(ParameterOutputID))

	outputIDBytes, err := hex.DecodeString(outputIDParam)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid output ID: %s, error: %s", outputIDParam, err)
	}

	if len(outputIDBytes) != (iotago.TransactionIDLength + iotago.UInt16ByteSize) {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid output ID: %s, error: %s", outputIDParam, err)
	}

	var outputID iotago.UTXOInputID
	copy(outputID[:], outputIDBytes)

	output, err := deps.UTXO.ReadOutputByOutputID(&outputID)
	if err != nil {
		if err == kvstore.ErrKeyNotFound {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "output not found: %s", outputIDParam)
		}

		return nil, errors.WithMessagef(restapi.ErrInternalError, "reading output failed: %s, error: %s", outputIDParam, err)
	}

	unspent, err := deps.UTXO.IsOutputUnspent(&outputID)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "reading spent status failed: %s, error: %s", outputIDParam, err)
	}

	return newOutputResponse(output, !unspent)
}

func ed25519Balance(address *iotago.Ed25519Address) (*addressBalanceResponse, error) {
	maxResults := deps.NodeConfig.Int(restapiplugin.CfgRestAPILimitsMaxResults)

	balance, count, err := deps.UTXO.AddressBalance(address, maxResults)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "reading address balance failed: %s, error: %s", address, err)
	}

	return &addressBalanceResponse{
		AddressType: iotago.AddressEd25519,
		Address:     address.String(),
		MaxResults:  uint32(maxResults),
		Count:       uint32(count),
		Balance:     balance,
	}, nil
}

func balanceByBech32Address(c echo.Context) (*addressBalanceResponse, error) {

	if !deps.Storage.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(restapi.ErrServiceUnavailable, "node is not synced")
	}

	addressParam := strings.ToLower(c.Param(ParameterAddress))

	_, bech32Address, err := iotago.ParseBech32(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address: %s, error: %s", addressParam, err)
	}

	switch address := bech32Address.(type) {
	case *iotago.WOTSAddress:
		// TODO: implement
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address: %s, error: %s", addressParam, iotago.ErrWOTSNotImplemented)
	case *iotago.Ed25519Address:
		return ed25519Balance(address)
	default:
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address: %s, error: unknown address type", addressParam)
	}
}

func balanceByEd25519Address(c echo.Context) (*addressBalanceResponse, error) {

	if !deps.Storage.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(restapi.ErrServiceUnavailable, "node is not synced")
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

func ed25519Outputs(address *iotago.Ed25519Address, includeSpent bool) (*addressOutputsResponse, error) {
	maxResults := deps.NodeConfig.Int(restapiplugin.CfgRestAPILimitsMaxResults)

	unspentOutputs, err := deps.UTXO.UnspentOutputsForAddress(address, maxResults)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "reading unspent outputs failed: %s, error: %s", address, err)
	}

	outputIDs := []string{}
	for _, unspentOutput := range unspentOutputs {
		outputIDs = append(outputIDs, hex.EncodeToString(unspentOutput.OutputID()[:]))
	}

	if includeSpent && maxResults-len(outputIDs) > 0 {

		spents, err := deps.UTXO.SpentOutputsForAddress(address, maxResults-len(outputIDs))
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInternalError, "reading spent outputs failed: %s, error: %s", address, err)
		}

		for _, spent := range spents {
			outputIDs = append(outputIDs, hex.EncodeToString(spent.OutputID()[:]))
		}
	}

	return &addressOutputsResponse{
		AddressType: iotago.AddressEd25519,
		Address:     address.String(),
		MaxResults:  uint32(maxResults),
		Count:       uint32(len(outputIDs)),
		OutputIDs:   outputIDs,
	}, nil
}

func outputsIDsByBech32Address(c echo.Context) (*addressOutputsResponse, error) {

	if !deps.Storage.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(restapi.ErrServiceUnavailable, "node is not synced")
	}

	addressParam := strings.ToLower(c.Param(ParameterAddress))
	includeSpent, _ := strconv.ParseBool(strings.ToLower(c.QueryParam("include-spent")))

	_, bech32Address, err := iotago.ParseBech32(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address: %s, error: %s", addressParam, err)
	}

	switch address := bech32Address.(type) {
	case *iotago.WOTSAddress:
		// TODO: implement
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address: %s, error: %s", addressParam, iotago.ErrWOTSNotImplemented)
	case *iotago.Ed25519Address:
		return ed25519Outputs(address, includeSpent)
	default:
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address: %s, error: unknown address type", addressParam)
	}
}

func outputsIDsByEd25519Address(c echo.Context) (*addressOutputsResponse, error) {

	if !deps.Storage.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(restapi.ErrServiceUnavailable, "node is not synced")
	}

	addressParam := strings.ToLower(c.Param(ParameterAddress))
	includeSpent, _ := strconv.ParseBool(strings.ToLower(c.QueryParam("include-spent")))

	addressBytes, err := hex.DecodeString(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address: %s, error: %s", addressParam, err)
	}

	if len(addressBytes) != (iotago.Ed25519AddressBytesLength) {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address length: %s", addressParam)
	}

	var address iotago.Ed25519Address
	copy(address[:], addressBytes)

	return ed25519Outputs(&address, includeSpent)
}
