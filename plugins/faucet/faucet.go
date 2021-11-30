package faucet

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/faucet"
	"github.com/gohornet/hornet/pkg/restapi"
)

func getFaucetInfo(_ echo.Context) (*faucet.FaucetInfoResponse, error) {
	return deps.Faucet.Info()
}

func addFaucetOutputToQueue(c echo.Context) (*faucet.FaucetEnqueueResponse, error) {

	request := &faucetEnqueueRequest{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "Invalid Request! Error: %s", err)
	}

	response, err := deps.Faucet.Enqueue(request.Address)
	if err != nil {
		return nil, err
	}

	return response, nil
}
