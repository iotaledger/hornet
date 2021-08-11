package v1

import (
	"encoding/hex"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go/v2"
)

func messageByTransactionID(c echo.Context) (*iotago.Message, error) {
	transactionIDHex := strings.ToLower(c.Param(ParameterTransactionID))

	transactionID, err := hex.DecodeString(transactionIDHex)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid transaction ID: %s, error: %s", transactionIDHex, err)
	}

	if len(transactionID) != iotago.TransactionIDLength {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid transaction ID: %s, invalid length: %d", transactionIDHex, len(transactionID))
	}

	// Get the first output of that transaction (using index 0)
	outputID := &iotago.UTXOInputID{}
	copy(outputID[:], transactionID)

	output, err := deps.UTXO.ReadOutputByOutputIDWithoutLocking(outputID)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "output for transaction not found: %s", transactionIDHex)
		}
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "failed to load output for transaction: %s", transactionIDHex)
	}

	cachedMsg := deps.Storage.CachedMessageOrNil(output.MessageID())
	if cachedMsg == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "transaction not found: %s", transactionIDHex)
	}
	defer cachedMsg.Release(true)

	return cachedMsg.Message().Message(), nil
}
