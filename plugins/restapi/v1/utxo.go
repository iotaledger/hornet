package v1

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/plugins/restapi/common"
)

func newOutputResponse(output *utxo.Output, spent bool) (*outputResponse, error) {

	sigLockedSingleDeposit := &iotago.SigLockedSingleOutput{
		Address: output.Address(),
		Amount:  output.Amount(),
	}

	sigLockedSingleDepositJSON, err := sigLockedSingleDeposit.MarshalJSON()
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInternalError, "marshalling output failed: %s, error: %w", hex.EncodeToString(output.UTXOKey()), err)
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
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid output ID: %s, error: %w", outputIDParam, err)
	}

	if len(outputIDBytes) != (iotago.TransactionIDLength + iotago.UInt16ByteSize) {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid output ID: %s, error: %w", outputIDParam, err)
	}

	var outputID iotago.UTXOInputID
	copy(outputID[:], outputIDBytes)

	output, err := tangle.UTXO().ReadOutputByOutputID(&outputID)
	if err != nil {
		if err == kvstore.ErrKeyNotFound {
			return nil, errors.WithMessagef(common.ErrInvalidParameter, "output not found: %s", outputIDParam)
		}

		return nil, errors.WithMessagef(common.ErrInternalError, "reading output failed: %s, error: %w", outputIDParam, err)
	}

	unspent, err := tangle.UTXO().IsOutputUnspent(&outputID)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInternalError, "reading spent status failed: %s, error: %w", outputIDParam, err)
	}

	return newOutputResponse(output, !unspent)
}

func balanceByAddress(c echo.Context) (*addressBalanceResponse, error) {
	addressParam := strings.ToLower(c.Param(ParameterAddress))

	// ToDo: accept bech32 input

	addressBytes, err := hex.DecodeString(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid address: %s, error: %w", addressParam, err)
	}

	if len(addressBytes) != (iotago.Ed25519AddressBytesLength) {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid address length: %s", addressParam)
	}

	var address iotago.Ed25519Address
	copy(address[:], addressBytes)

	maxResults := config.NodeConfig.Int(config.CfgRestAPILimitsMaxResults)

	balance, count, err := tangle.UTXO().AddressBalance(&address, maxResults)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInternalError, "reading address balance failed: %s, error: %w", address, err)
	}

	return &addressBalanceResponse{
		Address:    addressParam,
		MaxResults: uint32(maxResults),
		Count:      uint32(count),
		Balance:    balance,
	}, nil
}

func outputsIDsByAddress(c echo.Context) (*addressOutputsResponse, error) {
	addressParam := strings.ToLower(c.Param(ParameterAddress))

	// ToDo: accept bech32 input

	addressBytes, err := hex.DecodeString(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid address: %s, error: %w", addressParam, err)
	}

	if len(addressBytes) != (iotago.Ed25519AddressBytesLength) {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid address length: %s", addressParam)
	}

	var address iotago.Ed25519Address
	copy(address[:], addressBytes)

	maxResults := config.NodeConfig.Int(config.CfgRestAPILimitsMaxResults)

	unspentOutputs, err := tangle.UTXO().UnspentOutputsForAddress(&address, maxResults)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInternalError, "reading unspent outputs failed: %s, error: %w", address, err)
	}

	outputIDs := []string{}
	for _, unspentOutput := range unspentOutputs {
		outputIDs = append(outputIDs, hex.EncodeToString(unspentOutput.OutputID()[:]))
	}

	includeSpent, _ := strconv.ParseBool(strings.ToLower(c.QueryParam("include-spent")))

	if includeSpent && maxResults-len(outputIDs) > 0 {

		spents, err := tangle.UTXO().SpentOutputsForAddress(&address, maxResults-len(outputIDs))
		if err != nil {
			return nil, errors.WithMessagef(common.ErrInternalError, "reading spent outputs failed: %s, error: %w", address, err)
		}

		for _, spent := range spents {
			outputIDs = append(outputIDs, hex.EncodeToString(spent.OutputID()[:]))
		}
	}

	return &addressOutputsResponse{
		Address:    addressParam,
		MaxResults: uint32(maxResults),
		Count:      uint32(len(outputIDs)),
		OutputIDs:  outputIDs,
	}, nil
}
