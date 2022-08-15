package coreapi

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/restapi"
	"github.com/iotaledger/inx-app/httpserver"
	iotago "github.com/iotaledger/iota.go/v3"
)

func storageBlockByTransactionID(c echo.Context) (*storage.Block, error) {

	transactionID, err := httpserver.ParseTransactionIDParam(c, restapi.ParameterTransactionID)
	if err != nil {
		return nil, err
	}

	// Get the first output of that transaction (using index 0)
	outputID := iotago.OutputID{}
	copy(outputID[:], transactionID[:])

	output, err := deps.UTXOManager.ReadOutputByOutputIDWithoutLocking(outputID)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "output for transaction not found: %s", transactionID.ToHex())
		}

		return nil, errors.WithMessagef(echo.ErrInternalServerError, "failed to load output for transaction: %s", transactionID.ToHex())
	}

	cachedBlock := deps.Storage.CachedBlockOrNil(output.BlockID()) // block +1
	if cachedBlock == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "transaction not found: %s", transactionID.ToHex())
	}
	defer cachedBlock.Release(true) // block -1

	return cachedBlock.Block(), nil
}

func blockByTransactionID(c echo.Context) (*iotago.Block, error) {
	block, err := storageBlockByTransactionID(c)
	if err != nil {
		return nil, err
	}

	return block.Block(), nil
}

func blockBytesByTransactionID(c echo.Context) ([]byte, error) {
	block, err := storageBlockByTransactionID(c)
	if err != nil {
		return nil, err
	}

	return block.Data(), nil
}
