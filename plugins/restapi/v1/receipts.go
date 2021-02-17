package v1

import (
	"strconv"

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
	migratedAtStr := c.Param(ParameterMilestoneIndex)
	migratedAt, err := strconv.ParseUint(migratedAtStr, 10, 32)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid migrated at milestone index: %s, error: %s", migratedAtStr, err)
	}

	receipts := make([]*iotago.Receipt, 0)
	if err := deps.UTXO.ForEachMigratedAtReceiptTuple(uint32(migratedAt), func(rt *utxo.ReceiptTuple) bool {
		receipts = append(receipts, rt.Receipt)
		return true
	}); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "unable to retrieve receipts for migrated at index %d: %s", migratedAt, err)
	}

	return &receiptsResponse{Receipts: receipts}, nil
}
