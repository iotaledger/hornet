package v2

import (
	"encoding/json"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go/v3"
)

func NewOutputResponse(output *utxo.Output, ledgerIndex milestone.Index, metadataOnly bool) (*OutputResponse, error) {
	rawOutputJSON, err := output.Output().MarshalJSON()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "marshaling output failed: %s, error: %s", output.OutputID().ToHex(), err)
	}

	transactionID := output.OutputID().TransactionID()

	r := &OutputResponse{
		MessageID:                output.MessageID().ToHex(),
		TransactionID:            transactionID.ToHex(),
		Spent:                    false,
		OutputIndex:              output.OutputID().Index(),
		MilestoneIndexBooked:     output.MilestoneIndex(),
		MilestoneTimestampBooked: output.MilestoneTimestamp(),
		LedgerIndex:              ledgerIndex,
	}

	if !metadataOnly {
		rawRawOutputJSON := json.RawMessage(rawOutputJSON)
		r.RawOutput = &rawRawOutputJSON
	}

	return r, nil
}

func NewSpentResponse(spent *utxo.Spent, ledgerIndex milestone.Index, metadataOnly bool) (*OutputResponse, error) {
	response, err := NewOutputResponse(spent.Output(), ledgerIndex, metadataOnly)
	if err != nil {
		return nil, err
	}
	response.Spent = true
	response.MilestoneIndexSpent = spent.MilestoneIndex()
	response.TransactionIDSpent = spent.TargetTransactionID().ToHex()
	response.MilestoneTimestampSpent = spent.MilestoneTimestamp()
	return response, nil
}

func outputByID(c echo.Context, metadataOnly bool) (*OutputResponse, error) {
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
		return NewOutputResponse(output, ledgerIndex, metadataOnly)
	}

	spent, err := deps.UTXOManager.ReadSpentForOutputIDWithoutLocking(outputID)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "output not found: %s", outputID.ToHex())
		}
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading output failed: %s, error: %s", outputID.ToHex(), err)
	}
	return NewSpentResponse(spent, ledgerIndex, metadataOnly)
}

func rawOutputByID(c echo.Context) ([]byte, error) {
	outputID, err := restapi.ParseOutputIDParam(c)
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

func treasury(_ echo.Context) (*treasuryResponse, error) {

	treasuryOutput, err := deps.UTXOManager.UnspentTreasuryOutputWithoutLocking()
	if err != nil {
		return nil, err
	}

	return &treasuryResponse{
		MilestoneID: treasuryOutput.MilestoneID.ToHex(),
		Amount:      iotago.EncodeUint64(treasuryOutput.Amount),
	}, nil
}
