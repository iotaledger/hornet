package v1

import (
	"encoding/hex"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hornet/pkg/model/hornet"
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
