package v1

import (
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/restapi"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

func receipts(_ echo.Context) (*receiptsResponse, error) {
	receipts := make([]*iotago.Receipt, 0)
	if err := deps.UTXO.ForEachReceiptTuple(func(rt *utxo.ReceiptTuple) bool {
		receipts = append(receipts, rt.Receipt)
		return true
	}); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "unable to retrieve receipts: %s", err)
	}

	return &receiptsResponse{Receipts: receipts}, nil
}

func receiptsByMigratedAtIndex(c echo.Context) (*receiptsResponse, error) {
	migratedAt, err := ParseMilestoneIndexParam(c)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid migrated at milestone index: %d, error: %s", migratedAt, err)
	}

	receipts := make([]*iotago.Receipt, 0)
	if err := deps.UTXO.ForEachReceiptTupleMigratedAt(migratedAt, func(rt *utxo.ReceiptTuple) bool {
		receipts = append(receipts, rt.Receipt)
		return true
	}); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "unable to retrieve receipts for migrated at index %d: %s", migratedAt, err)
	}

	return &receiptsResponse{Receipts: receipts}, nil
}
