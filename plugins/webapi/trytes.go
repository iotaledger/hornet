package webapi

import (
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/transaction"

	"github.com/iotaledger/hornet/pkg/model/hornet"
	"github.com/iotaledger/hornet/pkg/model/tangle"
)

func (s *WebAPIServer) rpcGetTrytes(c echo.Context) (interface{}, error) {
	request := &GetTrytes{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	maxResults := s.limitsMaxResults
	if len(request.Hashes) > maxResults {
		return nil, errors.WithMessagef(ErrInvalidParameter, "too many hashes. maximum allowed: %d", maxResults)
	}

	trytes := []string{}

	for _, hash := range request.Hashes {
		if !guards.IsTransactionHash(hash) {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid hash provided: %s", hash)
		}
	}

	getTrytes := func(hash string) error {
		cachedTx := tangle.GetCachedTransactionOrNil(hornet.HashFromHashTrytes(hash))
		if cachedTx == nil {
			trytes = append(trytes, strings.Repeat("9", 2673))
			return nil
		}
		defer cachedTx.Release(true)

		txTrytes, err := transaction.TransactionToTrytes(cachedTx.GetTransaction().Tx)
		if err != nil {
			return err
		}

		trytes = append(trytes, txTrytes)
		return nil
	}

	for _, hash := range request.Hashes {
		if err := getTrytes(hash); err != nil {
			return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
		}
	}

	return &GetTrytesResponse{
		Trytes: trytes,
	}, nil
}

func (s *WebAPIServer) transaction(c echo.Context) (interface{}, error) {
	txHash, err := parseTransactionHashParam(c)
	if err != nil {
		return nil, err
	}

	cachedTx := tangle.GetCachedTransactionOrNil(txHash)
	if cachedTx == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "transaction not found: %s", txHash.Trytes())
	}
	defer cachedTx.Release(true)

	return cachedTx.GetTransaction().Tx, nil
}

func (s *WebAPIServer) transactionTrytes(c echo.Context) (interface{}, error) {
	txHash, err := parseTransactionHashParam(c)
	if err != nil {
		return nil, err
	}

	cachedTx := tangle.GetCachedTransactionOrNil(txHash)
	if cachedTx == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "transaction not found: %s", txHash.Trytes())
	}
	defer cachedTx.Release(true)

	txTrytes, err := transaction.TransactionToTrytes(cachedTx.GetTransaction().Tx)
	if err != nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	return &transactionTrytesResponse{
		TxHash: txHash.Trytes(),
		Trytes: txTrytes,
	}, nil
}
