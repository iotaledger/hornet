package coreapi

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hornet/v2/pkg/restapi"
	"github.com/iotaledger/inx-app/pkg/httpserver"
	iotago "github.com/iotaledger/iota.go/v3"
)

func blockIDByTransactionID(c echo.Context) (iotago.BlockID, error) {

	transactionID, err := httpserver.ParseTransactionIDParam(c, restapi.ParameterTransactionID)
	if err != nil {
		return iotago.BlockID{}, err
	}

	// Get the first output of that transaction (using index 0)
	outputID := iotago.OutputID{}
	copy(outputID[:], transactionID[:])

	output, err := deps.UTXOManager.ReadOutputByOutputIDWithoutLocking(outputID)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return iotago.BlockID{}, errors.WithMessagef(echo.ErrNotFound, "output for transaction not found: %s", transactionID.ToHex())
		}

		return iotago.BlockID{}, errors.WithMessagef(echo.ErrInternalServerError, "failed to load output for transaction: %s", transactionID.ToHex())
	}

	return output.BlockID(), nil
}

func blockByTransactionID(c echo.Context) (*iotago.Block, error) {
	blockID, err := blockIDByTransactionID(c)
	if err != nil {
		return nil, err
	}

	block, err := storageBlockByBlockID(blockID)
	if err != nil {
		return nil, err
	}

	return block.Block(), nil
}

func blockBytesByTransactionID(c echo.Context) ([]byte, error) {
	blockID, err := blockIDByTransactionID(c)
	if err != nil {
		return nil, err
	}

	block, err := storageBlockByBlockID(blockID)
	if err != nil {
		return nil, err
	}

	return block.Data(), nil
}

func blockMetadataByTransactionID(c echo.Context) (*blockMetadataResponse, error) {
	blockID, err := blockIDByTransactionID(c)
	if err != nil {
		return nil, err
	}

	return blockMetadataByBlockID(blockID)
}
