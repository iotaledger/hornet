package restapi

import (
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// ParameterMessageID is used to identify a message by its ID.
	ParameterMessageID = "messageID"

	// ParameterTransactionID is used to identify a transaction by its ID.
	ParameterTransactionID = "transactionID"

	// ParameterOutputID is used to identify an output by its ID.
	ParameterOutputID = "outputID"

	// ParameterAddress is used to identify an address.
	ParameterAddress = "address"

	// ParameterMilestoneIndex is used to identify a milestone.
	ParameterMilestoneIndex = "milestoneIndex"

	// ParameterPeerID is used to identify a peer.
	ParameterPeerID = "peerID"

	// QueryParameterOutputType is used to filter for a certain output type.
	QueryParameterOutputType = "type"
)

var (
	// ErrInvalidParameter defines the invalid parameter error.
	ErrInvalidParameter = echo.NewHTTPError(http.StatusBadRequest, "invalid parameter")

	// ErrServiceNotImplemented defines the service not implemented error.
	ErrServiceNotImplemented = echo.NewHTTPError(http.StatusNotImplemented, "service not implemented")
)

// JSONResponse wraps the result into a "data" field and sends the JSON response with status code.
func JSONResponse(c echo.Context, statusCode int, result interface{}) error {
	return c.JSON(statusCode, &HTTPOkResponseEnvelope{Data: result})
}

// HTTPErrorResponse defines the error struct for the HTTPErrorResponseEnvelope.
type HTTPErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HTTPErrorResponseEnvelope defines the error response schema for node API responses.
type HTTPErrorResponseEnvelope struct {
	Error HTTPErrorResponse `json:"error"`
}

// HTTPOkResponseEnvelope defines the ok response schema for node API responses.
type HTTPOkResponseEnvelope struct {
	// The response is encapsulated in the Data field.
	Data interface{} `json:"data"`
}

type (
	// AllowedRoute defines a function to allow or disallow routes.
	AllowedRoute func(echo.Context) bool
)

func ParseMessageIDParam(c echo.Context) (hornet.MessageID, error) {
	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid message ID: %s, error: %s", messageIDHex, err)
	}
	return messageID, nil
}

func ParseTransactionIDParam(c echo.Context) (*iotago.TransactionID, error) {
	transactionIDHex := strings.ToLower(c.Param(ParameterTransactionID))

	transactionIDBytes, err := hex.DecodeString(transactionIDHex)
	if err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid transaction ID: %s, error: %s", transactionIDHex, err)
	}

	if len(transactionIDBytes) != iotago.TransactionIDLength {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid transaction ID: %s, invalid length: %d", transactionIDHex, len(transactionIDBytes))
	}

	var transactionID iotago.TransactionID
	copy(transactionID[:], transactionIDBytes)
	return &transactionID, nil
}

func ParseOutputIDParam(c echo.Context) (*iotago.OutputID, error) {
	outputIDParam := strings.ToLower(c.Param(ParameterOutputID))

	outputIDBytes, err := hex.DecodeString(outputIDParam)
	if err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid output ID: %s, error: %s", outputIDParam, err)
	}

	if len(outputIDBytes) != iotago.OutputIDLength {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid output ID: %s, error: %s", outputIDParam, err)
	}

	var outputID iotago.OutputID
	copy(outputID[:], outputIDBytes)
	return &outputID, nil
}

func ParseBech32AddressParam(c echo.Context, prefix iotago.NetworkPrefix) (iotago.Address, error) {
	addressParam := strings.ToLower(c.Param(ParameterAddress))

	hrp, bech32Address, err := iotago.ParseBech32(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid address: %s, error: %s", addressParam, err)
	}

	if hrp != prefix {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid bech32 address, expected prefix: %s", prefix)
	}

	return bech32Address, nil
}

func ParseEd25519AddressParam(c echo.Context) (*iotago.Ed25519Address, error) {
	addressParam := strings.ToLower(c.Param(ParameterAddress))

	addressBytes, err := hex.DecodeString(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid address: %s, error: %s", addressParam, err)
	}

	if len(addressBytes) != (iotago.Ed25519AddressBytesLength) {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid address length: %s", addressParam)
	}

	var address iotago.Ed25519Address
	copy(address[:], addressBytes)
	return &address, nil
}

func ParseMilestoneIndexParam(c echo.Context) (milestone.Index, error) {
	milestoneIndex := strings.ToLower(c.Param(ParameterMilestoneIndex))
	if milestoneIndex == "" {
		return 0, errors.WithMessagef(ErrInvalidParameter, "parameter \"%s\" not specified", ParameterMilestoneIndex)
	}

	msIndex, err := strconv.ParseUint(milestoneIndex, 10, 32)
	if err != nil {
		return 0, errors.WithMessagef(ErrInvalidParameter, "invalid milestone index: %s, error: %s", milestoneIndex, err)
	}

	return milestone.Index(msIndex), nil
}

func ParsePeerIDParam(c echo.Context) (peer.ID, error) {
	peerID, err := peer.Decode(c.Param(ParameterPeerID))
	if err != nil {
		return "", errors.WithMessagef(ErrInvalidParameter, "invalid peerID, error: %s", err)
	}
	return peerID, nil
}

func ParseOutputTypeQueryParam(c echo.Context) (*iotago.OutputType, error) {
	typeParam := strings.ToLower(c.QueryParam(QueryParameterOutputType))
	var filteredType *iotago.OutputType

	if len(typeParam) > 0 {
		outputTypeInt, err := strconv.ParseInt(typeParam, 10, 32)
		if err != nil {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid type: %s, error: unknown output type", typeParam)
		}
		outputType := iotago.OutputType(outputTypeInt)
		switch outputType {
		case iotago.OutputExtended, iotago.OutputAlias, iotago.OutputNFT, iotago.OutputFoundry:
		default:
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid type: %s, error: unknown output type", typeParam)
		}
		filteredType = &outputType
	}
	return filteredType, nil
}
