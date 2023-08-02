package webapi

import (
	"sort"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hornet/pkg/config"
	"github.com/iotaledger/hornet/plugins/curl"
	"github.com/iotaledger/hornet/plugins/pow"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
)

func (s *WebAPIServer) rpcAttachToTangle(c echo.Context) (interface{}, error) {
	request := &AttachToTangle{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	mwm := config.NodeConfig.GetInt(config.CfgCoordinatorMWM)

	// mwm is an optional parameter
	if request.MinWeightMagnitude == 0 {
		request.MinWeightMagnitude = mwm
	}

	// Reject wrong MWM
	if request.MinWeightMagnitude != mwm {
		return nil, errors.WithMessagef(echo.ErrBadRequest, "Wrong MinWeightMagnitude. requested: %d, expected: %d", request.MinWeightMagnitude, mwm)
	}

	// Reject empty requests
	if len(request.Trytes) == 0 {
		return nil, errors.WithMessage(echo.ErrBadRequest, "No trytes given.")
	}

	txs, err := transaction.AsTransactionObjects(request.Trytes, nil)
	if err != nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	// Reject bundles with invalid tx amount
	if uint64(len(txs)) != txs[0].LastIndex+1 {
		return nil, errors.WithMessagef(echo.ErrBadRequest, "Invalid bundle length. Received txs: %v, Bundle requires: %v", len(txs), txs[0].LastIndex+1)
	}

	// Sort transactions (highest to lowest index)
	sort.Slice(txs, func(i, j int) bool {
		return txs[i].CurrentIndex > txs[j].CurrentIndex
	})

	// Check transaction indexes
	for i, j := uint64(0), uint64(len(txs)-1); j > 0; i, j = i+1, j-1 {
		if txs[i].CurrentIndex != j {
			return nil, errors.WithMessagef(echo.ErrBadRequest, "Invalid transaction index. Got: %d, expected: %d", txs[i].CurrentIndex, j)
		}
	}

	var prev trinary.Hash
	for i := 0; i < len(txs); i++ {

		switch {
		case i == 0:
			txs[i].TrunkTransaction = request.TrunkTransaction
			txs[i].BranchTransaction = request.BranchTransaction
		default:
			txs[i].TrunkTransaction = prev
			txs[i].BranchTransaction = request.TrunkTransaction
		}

		txs[i].AttachmentTimestamp = time.Now().UnixNano() / int64(time.Millisecond)
		txs[i].AttachmentTimestampLowerBound = consts.LowerBoundAttachmentTimestamp
		txs[i].AttachmentTimestampUpperBound = consts.UpperBoundAttachmentTimestamp

		// Convert tx to trytes
		trytes, err := transaction.TransactionToTrytes(&txs[i])
		if err != nil {
			return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
		}

		// Do the PoW
		ts := time.Now()
		txs[i].Nonce, err = pow.Handler().DoPoW(trytes, request.MinWeightMagnitude)
		if err != nil {
			return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
		}
		s.logger.Debugf("PoW method: \"%s\", MWM: %d, took %v", pow.Handler().GetPoWType(), mwm, time.Since(ts).Truncate(time.Millisecond))

		// Convert tx to trits
		txTrits, err := transaction.TransactionToTrits(&txs[i])
		if err != nil {
			return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
		}

		// Calculate the transaction hash with the batched hasher
		hashTrits, err := curl.Hasher().Hash(txTrits)
		if err != nil {
			return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
		}

		txs[i].Hash = trinary.MustTritsToTrytes(hashTrits)

		prev = txs[i].Hash

		// Check tx
		if !transaction.HasValidNonce(&txs[i], uint64(request.MinWeightMagnitude)) {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "invalid nonce: %s", prev)
		}
	}

	// Reverse the transactions the same way IRI does (for whatever reason)
	for i, j := 0, len(txs)-1; i < j; i, j = i+1, j-1 {
		txs[i], txs[j] = txs[j], txs[i]
	}

	powedTxTrytes := transaction.MustTransactionsToTrytes(txs)

	return &AttachToTangleResponse{Trytes: powedTxTrytes}, nil
}
