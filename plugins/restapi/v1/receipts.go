package v1

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/restapi"
)

func receipts(_ echo.Context) (*receiptsResponse, error) {
	receipts := make([]*utxo.ReceiptTuple, 0)
	if err := deps.UTXO.ForEachReceiptTuple(func(rt *utxo.ReceiptTuple) bool {
		receipts = append(receipts, rt)
		return true
	}); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "unable to retrieve receipts: %s", err)
	}

	return &receiptsResponse{Receipts: receipts}, nil
}

func receiptsByMigratedAtIndex(c echo.Context) (*receiptsResponse, error) {
	migratedAt, err := ParseMilestoneIndexParam(c)
	if err != nil {
		return nil, err
	}

	receipts := make([]*utxo.ReceiptTuple, 0)
	if err := deps.UTXO.ForEachReceiptTupleMigratedAt(migratedAt, func(rt *utxo.ReceiptTuple) bool {
		receipts = append(receipts, rt)
		return true
	}); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "unable to retrieve receipts for migrated at index %d: %s", migratedAt, err)
	}

	return &receiptsResponse{Receipts: receipts}, nil
}
