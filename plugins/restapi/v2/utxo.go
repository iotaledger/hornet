package v2

import (
	"encoding/hex"
	"encoding/json"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/iotaledger/hive.go/kvstore"
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
