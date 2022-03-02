package v1

import (
	"encoding/hex"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go/v2"
)

func messageByTransactionID(c echo.Context) (*iotago.Message, error) {
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

	cachedMsg := deps.Storage.CachedMessageOrNil(output.MessageID()) // message +1
	if cachedMsg == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "transaction not found: %s", hex.EncodeToString(transactionID[:]))
	}
	defer cachedMsg.Release(true) // message -1

	return cachedMsg.Message().Message(), nil
}
