package restapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/iotaledger/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	MIMEApplicationVendorIOTASerializerV1 = "application/vnd.iota.serializer-v1"
)

const (
	// ParameterBlockID is used to identify a block by its ID.
	ParameterBlockID = "blockID"

	// ParameterTransactionID is used to identify a transaction by its ID.
	ParameterTransactionID = "transactionID"

	// ParameterOutputID is used to identify an output by its ID.
	ParameterOutputID = "outputID"

	// ParameterMilestoneIndex is used to identify a milestone by index.
	ParameterMilestoneIndex = "milestoneIndex"

	// ParameterMilestoneID is used to identify a milestone by its ID.
	ParameterMilestoneID = "milestoneID"

	// ParameterPeerID is used to identify a peer.
	ParameterPeerID = "peerID"

	// QueryParameterOutputType is used to filter for a certain output type.
	QueryParameterOutputType = "type"
)

var (
	// ErrInvalidParameter defines the invalid parameter error.
	ErrInvalidParameter = echo.NewHTTPError(http.StatusBadRequest, "invalid parameter")

	// ErrNotAcceptable defines the not acceptable error.
	ErrNotAcceptable = echo.NewHTTPError(http.StatusNotAcceptable)
)

// JSONResponse sends the JSON response with status code.
func JSONResponse(c echo.Context, statusCode int, result interface{}) error {
	return c.JSON(statusCode, result)
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

type (
	// AllowedRoute defines a function to allow or disallow routes.
	AllowedRoute func(echo.Context) bool
)

func ErrorHandler() func(error, echo.Context) {
	return func(err error, c echo.Context) {

		var statusCode int
		var message string

		var e *echo.HTTPError
		if errors.As(err, &e) {
			statusCode = e.Code
			message = fmt.Sprintf("%s, error: %s", e.Message, err)
		} else {
			statusCode = http.StatusInternalServerError
			message = fmt.Sprintf("internal server error. error: %s", err)
		}

		_ = c.JSON(statusCode, HTTPErrorResponseEnvelope{Error: HTTPErrorResponse{Code: strconv.Itoa(statusCode), Message: message}})
	}
}

func GetAcceptHeaderContentType(c echo.Context, supportedContentTypes ...string) (string, error) {
	ctype := c.Request().Header.Get(echo.HeaderAccept)
	for _, supportedContentType := range supportedContentTypes {
		if strings.HasPrefix(ctype, supportedContentType) {
			return supportedContentType, nil
		}
	}
	return "", ErrNotAcceptable
}

func GetRequestContentType(c echo.Context, supportedContentTypes ...string) (string, error) {
	ctype := c.Request().Header.Get(echo.HeaderContentType)
	for _, supportedContentType := range supportedContentTypes {
		if strings.HasPrefix(ctype, supportedContentType) {
			return supportedContentType, nil
		}
	}
	return "", echo.ErrUnsupportedMediaType
}

func ParseBlockIDParam(c echo.Context) (iotago.BlockID, error) {
	blockIDHex := strings.ToLower(c.Param(ParameterBlockID))

	blockID, err := iotago.BlockIDFromHexString(blockIDHex)
	if err != nil {
		return iotago.EmptyBlockID(), errors.WithMessagef(ErrInvalidParameter, "invalid block ID: %s, error: %s", blockIDHex, err)
	}
	return blockID, nil
}

func ParseTransactionIDParam(c echo.Context) (iotago.TransactionID, error) {
	transactionID := iotago.TransactionID{}
	transactionIDHex := strings.ToLower(c.Param(ParameterTransactionID))

	transactionIDBytes, err := iotago.DecodeHex(transactionIDHex)
	if err != nil {
		return transactionID, errors.WithMessagef(ErrInvalidParameter, "invalid transaction ID: %s, error: %s", transactionIDHex, err)
	}

	if len(transactionIDBytes) != iotago.TransactionIDLength {
		return transactionID, errors.WithMessagef(ErrInvalidParameter, "invalid transaction ID: %s, invalid length: %d", transactionIDHex, len(transactionIDBytes))
	}

	copy(transactionID[:], transactionIDBytes)
	return transactionID, nil
}

func ParseOutputIDParam(c echo.Context) (iotago.OutputID, error) {
	outputIDParam := strings.ToLower(c.Param(ParameterOutputID))

	outputID, err := iotago.OutputIDFromHex(outputIDParam)
	if err != nil {
		return iotago.OutputID{}, errors.WithMessagef(ErrInvalidParameter, "invalid output ID: %s, error: %s", outputIDParam, err)
	}
	return outputID, nil
}

func ParseMilestoneIndexParam(c echo.Context, paramName string) (milestone.Index, error) {
	milestoneIndex := strings.ToLower(c.Param(paramName))
	if milestoneIndex == "" {
		return 0, errors.WithMessagef(ErrInvalidParameter, "parameter \"%s\" not specified", paramName)
	}

	msIndex, err := strconv.ParseUint(milestoneIndex, 10, 32)
	if err != nil {
		return 0, errors.WithMessagef(ErrInvalidParameter, "invalid milestone index: %s, error: %s", milestoneIndex, err)
	}

	return milestone.Index(msIndex), nil
}

func ParseMilestoneIDParam(c echo.Context) (*iotago.MilestoneID, error) {
	milestoneIDHex := strings.ToLower(c.Param(ParameterMilestoneID))

	milestoneIDBytes, err := iotago.DecodeHex(milestoneIDHex)
	if err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid milestone ID: %s, error: %s", milestoneIDHex, err)
	}

	if len(milestoneIDBytes) != iotago.MilestoneIDLength {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid milestone ID: %s, invalid length: %d", milestoneIDHex, len(milestoneIDBytes))
	}

	var milestoneID iotago.MilestoneID
	copy(milestoneID[:], milestoneIDBytes)
	return &milestoneID, nil
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
		case iotago.OutputBasic, iotago.OutputAlias, iotago.OutputNFT, iotago.OutputFoundry:
		default:
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid type: %s, error: unknown output type", typeParam)
		}
		filteredType = &outputType
	}
	return filteredType, nil
}
