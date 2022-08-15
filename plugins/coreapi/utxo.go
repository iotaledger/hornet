package coreapi

import (
	"encoding/json"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/restapi"
	"github.com/iotaledger/inx-app/httpserver"
	iotago "github.com/iotaledger/iota.go/v3"
)

func NewOutputMetadataResponse(output *utxo.Output, ledgerIndex iotago.MilestoneIndex) *OutputMetadataResponse {
	return &OutputMetadataResponse{
		BlockID:                  output.BlockID().ToHex(),
		TransactionID:            output.OutputID().TransactionID().ToHex(),
		Spent:                    false,
		OutputIndex:              output.OutputID().Index(),
		MilestoneIndexBooked:     output.MilestoneIndexBooked(),
		MilestoneTimestampBooked: output.MilestoneTimestampBooked(),
		LedgerIndex:              ledgerIndex,
	}
}

func rawMessageForOutput(output *utxo.Output) (*json.RawMessage, error) {
	rawOutputJSON, err := output.Output().MarshalJSON()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "marshaling output failed: %s, error: %s", output.OutputID().ToHex(), err)
	}
	rawRawOutputJSON := json.RawMessage(rawOutputJSON)

	return &rawRawOutputJSON, nil
}

func NewSpentMetadataResponse(spent *utxo.Spent, ledgerIndex iotago.MilestoneIndex) *OutputMetadataResponse {
	metadata := NewOutputMetadataResponse(spent.Output(), ledgerIndex)
	metadata.Spent = true
	metadata.MilestoneTimestampSpent = spent.MilestoneTimestampSpent()
	metadata.TransactionIDSpent = spent.TransactionIDSpent().ToHex()
	metadata.MilestoneIndexSpent = spent.MilestoneIndexSpent()

	return metadata
}

func NewOutputResponse(output *utxo.Output, ledgerIndex iotago.MilestoneIndex) (*OutputResponse, error) {
	rawOutput, err := rawMessageForOutput(output)
	if err != nil {
		return nil, err
	}

	return &OutputResponse{
		Metadata:  NewOutputMetadataResponse(output, ledgerIndex),
		RawOutput: rawOutput,
	}, nil
}

func NewSpentResponse(spent *utxo.Spent, ledgerIndex iotago.MilestoneIndex) (*OutputResponse, error) {
	rawOutput, err := rawMessageForOutput(spent.Output())
	if err != nil {
		return nil, err
	}

	return &OutputResponse{
		Metadata:  NewSpentMetadataResponse(spent, ledgerIndex),
		RawOutput: rawOutput,
	}, nil
}

func outputByID(c echo.Context) (*OutputResponse, error) {
	outputID, err := httpserver.ParseOutputIDParam(c, restapi.ParameterOutputID)
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

	isUnspent, err := deps.UTXOManager.IsOutputIDUnspentWithoutLocking(outputID)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading output spent status failed: %s, error: %s", outputID.ToHex(), err)
	}

	if isUnspent {
		output, err := deps.UTXOManager.ReadOutputByOutputIDWithoutLocking(outputID)
		if err != nil {
			if errors.Is(err, kvstore.ErrKeyNotFound) {
				return nil, errors.WithMessagef(echo.ErrNotFound, "output not found: %s", outputID.ToHex())
			}

			return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading output failed: %s, error: %s", outputID.ToHex(), err)
		}

		return NewOutputResponse(output, ledgerIndex)
	}

	spent, err := deps.UTXOManager.ReadSpentForOutputIDWithoutLocking(outputID)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "output not found: %s", outputID.ToHex())
		}

		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading output failed: %s, error: %s", outputID.ToHex(), err)
	}

	return NewSpentResponse(spent, ledgerIndex)
}

func outputMetadataByID(c echo.Context) (*OutputMetadataResponse, error) {
	outputID, err := httpserver.ParseOutputIDParam(c, restapi.ParameterOutputID)
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

	isUnspent, err := deps.UTXOManager.IsOutputIDUnspentWithoutLocking(outputID)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading output spent status failed: %s, error: %s", outputID.ToHex(), err)
	}

	if isUnspent {
		output, err := deps.UTXOManager.ReadOutputByOutputIDWithoutLocking(outputID)
		if err != nil {
			if errors.Is(err, kvstore.ErrKeyNotFound) {
				return nil, errors.WithMessagef(echo.ErrNotFound, "output not found: %s", outputID.ToHex())
			}

			return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading output failed: %s, error: %s", outputID.ToHex(), err)
		}

		return NewOutputMetadataResponse(output, ledgerIndex), nil
	}

	spent, err := deps.UTXOManager.ReadSpentForOutputIDWithoutLocking(outputID)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "output not found: %s", outputID.ToHex())
		}

		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading output failed: %s, error: %s", outputID.ToHex(), err)
	}

	return NewSpentMetadataResponse(spent, ledgerIndex), nil
}

func rawOutputByID(c echo.Context) ([]byte, error) {
	outputID, err := httpserver.ParseOutputIDParam(c, restapi.ParameterOutputID)
	if err != nil {
		return nil, err
	}

	bytes, err := deps.UTXOManager.ReadRawOutputBytesByOutputIDWithoutLocking(outputID)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "output not found: %s", outputID.ToHex())
		}

		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading raw output failed: %s, error: %s", outputID.ToHex(), err)
	}

	return bytes, nil
}

func treasury(_ echo.Context) (*utxo.TreasuryOutput, error) {
	return deps.UTXOManager.UnspentTreasuryOutputWithoutLocking()
}
