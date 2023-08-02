package webapi

import (
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hornet/pkg/model/hornet"

	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/trinary"
)

func restoreBody(c echo.Context, bodyBytes []byte) {
	// Restore the io.ReadCloser to its original state
	c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
}

func parseAddressParam(c echo.Context) (hornet.Hash, error) {
	addr := strings.ToUpper(c.Param(ParameterAddress))

	// Check if address is valid
	if err := address.ValidAddress(addr); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid address hash provided: %s, error: %s", addr, err)
	}

	return hornet.HashFromAddressTrytes(addr), nil
}

func parseTransactionHashParam(c echo.Context) (hornet.Hash, error) {
	txHash := strings.ToUpper(c.Param(ParameterTransactionHash))

	if !guards.IsTransactionHash(txHash) {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid transaction hash provided: %s", txHash)
	}

	return hornet.HashFromHashTrytes(txHash), nil
}

func parseBundleQueryParam(c echo.Context) (hornet.Hash, error) {
	value := strings.ToUpper(c.QueryParam(QueryParameterBundle))

	if len(value) > 0 {
		if !guards.IsTransactionHash(value) {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid bundle hash provided: %s", value)
		}

		return hornet.HashFromHashTrytes(value), nil
	}

	return nil, nil
}

func parseApproveeQueryParam(c echo.Context) (hornet.Hash, error) {
	value := strings.ToUpper(c.QueryParam(QueryParameterApprovee))

	if len(value) > 0 {
		if !guards.IsTransactionHash(value) {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid approvee hash provided: %s", value)
		}

		return hornet.HashFromHashTrytes(value), nil
	}

	return nil, nil
}

func parseAddressQueryParam(c echo.Context) (hornet.Hash, error) {
	value := strings.ToUpper(c.QueryParam(QueryParameterAddress))

	if len(value) > 0 {
		// Check if address is valid
		if err := address.ValidAddress(value); err != nil {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid address hash provided: %s, error: %s", value, err)
		}

		return hornet.HashFromAddressTrytes(value), nil
	}

	return nil, nil
}

func parseTagQueryParam(c echo.Context) (hornet.Hash, error) {
	value := strings.ToUpper(c.QueryParam(QueryParameterTag))

	if len(value) > 0 {
		if err := trinary.ValidTrytes(value); err != nil {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid tag trytes provided: %s", value)
		}
		if len(value) > 27 {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid tag length: %s", value)
		}
		if len(value) < 27 {
			value = trinary.MustPad(value, 27)
		}

		return hornet.HashFromTagTrytes(value), nil
	}

	return nil, nil
}

func parseMaxResultsQueryParam(c echo.Context, maxResults int) (int, error) {
	value := c.QueryParam(QueryParameterMaxResults)

	if len(value) > 0 {
		requestMaxResults, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return 0, errors.WithMessagef(ErrInvalidParameter, "invalid %s, error: %s", QueryParameterMaxResults, err)
		}

		if (requestMaxResults > 0) && (int(requestMaxResults) < maxResults) {
			maxResults = int(requestMaxResults)
		}

	}

	return maxResults, nil
}
