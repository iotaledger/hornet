package coreapi

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/restapi"
	"github.com/iotaledger/inx-app/httpserver"
)

func receipts(_ echo.Context) (*receiptsResponse, error) {
	receipts := make([]*utxo.ReceiptTuple, 0)
	if err := deps.UTXOManager.ForEachReceiptTuple(func(rt *utxo.ReceiptTuple) bool {
		receipts = append(receipts, rt)

		return true
	}, utxo.ReadLockLedger(false)); err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "unable to retrieve receipts: %s", err)
	}

	return &receiptsResponse{Receipts: receipts}, nil
}

func receiptsByMigratedAtIndex(c echo.Context) (*receiptsResponse, error) {
	migratedAt, err := httpserver.ParseMilestoneIndexParam(c, restapi.ParameterMilestoneIndex)
	if err != nil {
		return nil, err
	}

	receipts := make([]*utxo.ReceiptTuple, 0)
	if err := deps.UTXOManager.ForEachReceiptTupleMigratedAt(migratedAt, func(rt *utxo.ReceiptTuple) bool {
		receipts = append(receipts, rt)

		return true
	}, utxo.ReadLockLedger(false)); err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "unable to retrieve receipts for migrated at index %d: %s", migratedAt, err)
	}

	return &receiptsResponse{Receipts: receipts}, nil
}
