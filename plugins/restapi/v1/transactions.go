package v1

import (
	"encoding/hex"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hornet/pkg/model/hornet"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	"github.com/iotaledger/hornet/pkg/restapi"
	iotago "github.com/iotaledger/iota.go/v2"
)

func messageIDByTransactionID(c echo.Context) (hornet.MessageID, error) {
	transactionID, err := restapi.ParseTransactionIDParam(c)
	if err != nil {
		return nil, err
	}

	// Get the first output of that transaction (using index 0)
	outputID := &iotago.UTXOInputID{}
	copy(outputID[:], transactionID[:])

	output, err := deps.UTXOManager.ReadOutputByOutputIDWithoutLocking(outputID)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "output for transaction not found: %s", hex.EncodeToString(transactionID[:]))
		}
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "failed to load output for transaction: %s", hex.EncodeToString(transactionID[:]))
	}

	return output.MessageID(), nil
}

func transactionHistoryByAddress(c echo.Context, address iotago.Address) (*transactionHistoryResponse, error) {
	// we need to lock the ledger here to have the correct index for unspent info of the output.
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	ledgerIndex, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading ledger index failed, error: %s", err)
	}

	maxResults := maxResultsFromContext(c)

	messageIDs := make(map[string]struct{}, 0)
	if err := deps.UTXOManager.ForEachUnspentOutput(func(output *utxo.Output) bool {
		messageIDs[output.MessageID().ToMapKey()] = struct{}{}
		return maxResults-len(messageIDs) > 0
	}, utxo.FilterAddress(address), utxo.ReadLockLedger(false)); err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading unspent outputs failed: %s, error: %s", address, err)
	}

	if maxResults-len(messageIDs) > 0 {
		if err := deps.UTXOManager.ForEachSpentOutput(func(spent *utxo.Spent) bool {
			messageIDs[spent.MessageID().ToMapKey()] = struct{}{}
			return maxResults-len(messageIDs) > 0
		}, utxo.FilterAddress(address), utxo.ReadLockLedger(false)); err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading spent outputs failed: %s, error: %s", address, err)
		}
	}

	getTransactionHistoryItem := func(messageID hornet.MessageID) (*transactionHistoryItem, error) {
		cachedMsg := deps.Storage.CachedMessageOrNil(messageID)
		if cachedMsg == nil {
			// if we don't have the message, we don't have the history, which is fine.
			return nil, nil
		}
		defer cachedMsg.Release(true)

		msg := cachedMsg.Message()
		if msg == nil {
			return nil, fmt.Errorf("message not found: %s", messageID.ToHex())
		}

		msgMeta := cachedMsg.Metadata()
		if msgMeta == nil {
			return nil, fmt.Errorf("message not found: %s", messageID.ToHex())
		}

		var referencedByMilestoneIndex milestone.Index
		if referenced, referencedIndex := msgMeta.ReferencedWithIndex(); referenced {
			referencedByMilestoneIndex = referencedIndex
		}

		ledgerInclusionState := "noTransaction"
		conflict := msgMeta.Conflict()
		var conflictReason *storage.Conflict

		if conflict != storage.ConflictNone {
			ledgerInclusionState = "conflicting"
			conflictReason = &conflict
		} else if msgMeta.IsIncludedTxInLedger() {
			ledgerInclusionState = "included"
		}

		txPayload := msg.Transaction()
		if txPayload == nil {
			return nil, fmt.Errorf("message does not contain a transaction payload: %s", messageID.ToHex())
		}

		transactionID, err := txPayload.ID()
		if err != nil {
			return nil, fmt.Errorf("can't compute the transaction ID, msgID: %s, error: %w", messageID.ToHex(), err)
		}
		txID := *transactionID

		txEssence := msg.TransactionEssence()
		if txEssence == nil {
			return nil, fmt.Errorf("transaction does not contain a valid transactionEssence: msgID: %s", messageID.ToHex())
		}

		var addressBalanceInputs int64
		for _, input := range txEssence.Inputs {
			utxoInput, ok := input.(*iotago.UTXOInput)
			if !ok {
				return nil, fmt.Errorf("transaction contains an unsupported input type: msgID: %s", messageID.ToHex())
			}

			utxoInputID := utxoInput.ID()
			output, err := deps.UTXOManager.ReadOutputByOutputIDWithoutLocking(&utxoInputID)
			if err != nil {
				// if we don't have the input, we don't have the history, which is fine.
				return nil, nil
			}

			if output.Address().String() != address.String() {
				continue
			}

			addressBalanceInputs += int64(output.Amount())
		}

		var addressBalanceOutputs int64
		for _, txOutput := range txEssence.Outputs {
			switch output := txOutput.(type) {
			case *iotago.SigLockedSingleOutput:
				//nolint:forcetypeassert
				if output.Address.(iotago.Address).String() != address.String() {
					continue
				}
				addressBalanceOutputs += int64(output.Amount)
			case *iotago.SigLockedDustAllowanceOutput:
				//nolint:forcetypeassert
				if output.Address.(iotago.Address).String() != address.String() {
					continue
				}
				addressBalanceOutputs += int64(output.Amount)
			default:
				return nil, fmt.Errorf("transaction contains an unsupported output type: msgID: %s", messageID.ToHex())
			}
		}

		milestoneTimestampReferenced, err := deps.Storage.MilestoneTimestampUnixByIndex(referencedByMilestoneIndex)
		if err != nil {
			return nil, err
		}

		return &transactionHistoryItem{
			MessageID:                    messageID.ToHex(),
			TransactionID:                hex.EncodeToString(txID[:]),
			ReferencedByMilestoneIndex:   referencedByMilestoneIndex,
			MilestoneTimestampReferenced: milestoneTimestampReferenced,
			LedgerInclusionState:         ledgerInclusionState,
			ConflictReason:               conflictReason,
			InputsCount:                  len(txEssence.Inputs),
			OutputsCount:                 len(txEssence.Outputs),
			AddressBalanceChange:         addressBalanceOutputs - addressBalanceInputs,
		}, nil
	}

	txHistoryItems := make([]*transactionHistoryItem, 0, len(messageIDs))
	for messageID := range messageIDs {
		txHistoryItem, err := getTransactionHistoryItem(hornet.MessageIDFromMapKey(messageID))
		if err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "get transaction history failed: %s, error: %s", address, err)
		}

		if txHistoryItem == nil {
			// skip if we don't have the history
			continue
		}

		txHistoryItems = append(txHistoryItems, txHistoryItem)
	}

	return &transactionHistoryResponse{
		AddressType: address.Type(),
		Address:     address.String(),
		MaxResults:  uint32(maxResults),
		Count:       uint32(len(txHistoryItems)),
		History:     txHistoryItems,
		LedgerIndex: ledgerIndex,
	}, nil
}
