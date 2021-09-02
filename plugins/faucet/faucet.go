package faucet

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/faucet"
	"github.com/gohornet/hornet/pkg/restapi"
	iotago "github.com/iotaledger/iota.go/v2"
)

func parseBech32Address(addressParam string) (*iotago.Ed25519Address, error) {

	hrp, bech32Address, err := iotago.ParseBech32(addressParam)
	if err != nil {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "Invalid bech32 address provided!")
	}

	if hrp != deps.Faucet.NetworkPrefix() {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "Invalid bech32 address provided! Address does not start with \"%s\".", deps.Faucet.NetworkPrefix())
	}

	switch address := bech32Address.(type) {
	case *iotago.Ed25519Address:
		return address, nil
	default:
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "Invalid bech32 address provided! Unknown address type.")
	}
}

func getFaucetInfo(_ echo.Context) (*faucet.FaucetInfoResponse, error) {
	return deps.Faucet.Info()
}

func addFaucetOutputToQueue(c echo.Context) (*faucet.FaucetEnqueueResponse, error) {

	request := &faucetEnqueueRequest{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "Invalid Request! Error: %s", err)
	}

	bech32Addr := request.Address
	ed25519Addr, err := parseBech32Address(bech32Addr)
	if err != nil {
		return nil, err
	}

	response, err := deps.Faucet.Enqueue(bech32Addr, ed25519Addr)
	if err != nil {
		return nil, err
	}

	return response, nil
}
