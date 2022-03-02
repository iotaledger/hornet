package restapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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

	// ParameterFoundryID is used to identify a foundry by its ID.
	ParameterFoundryID = "foundryID"

	// ParameterAliasID is used to identify an alias by its ID.
	ParameterAliasID = "aliasID"

	// ParameterNFTID is used to identify a nft by its ID.
	ParameterNFTID = "nftID"

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

	transactionIDBytes, err := iotago.DecodeHex(transactionIDHex)
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

	outputID, err := iotago.OutputIDFromHex(outputIDParam)
	if err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid output ID: %s, error: %s", outputIDParam, err)
	}

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

	addressBytes, err := iotago.DecodeHex(addressParam)
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

func ParseAliasAddressParam(c echo.Context) (*iotago.AliasAddress, error) {
	addressParam := strings.ToLower(c.Param(ParameterAddress))

	addressBytes, err := iotago.DecodeHex(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid address: %s, error: %s", addressParam, err)
	}

	if len(addressBytes) != (iotago.AliasAddressBytesLength) {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid address length: %s", addressParam)
	}

	var address iotago.AliasAddress
	copy(address[:], addressBytes)
	return &address, nil
}

func ParseNFTAddressParam(c echo.Context) (*iotago.NFTAddress, error) {
	addressParam := strings.ToLower(c.Param(ParameterAddress))

	addressBytes, err := iotago.DecodeHex(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid address: %s, error: %s", addressParam, err)
	}

	if len(addressBytes) != (iotago.NFTAddressBytesLength) {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid address length: %s", addressParam)
	}

	var address iotago.NFTAddress
	copy(address[:], addressBytes)
	return &address, nil
}

func ParseAliasIDParam(c echo.Context) (*iotago.AliasID, error) {
	aliasIDParam := strings.ToLower(c.Param(ParameterAliasID))

	aliasIDBytes, err := iotago.DecodeHex(aliasIDParam)
	if err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid alias ID: %s, error: %s", aliasIDParam, err)
	}

	if len(aliasIDBytes) != iotago.AliasIDLength {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid alias ID: %s, error: %s", aliasIDParam, err)
	}

	var aliasID iotago.AliasID
	copy(aliasID[:], aliasIDBytes)
	return &aliasID, nil
}

func ParseNFTIDParam(c echo.Context) (*iotago.NFTID, error) {
	nftIDParam := strings.ToLower(c.Param(ParameterNFTID))

	nftIDBytes, err := iotago.DecodeHex(nftIDParam)
	if err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid NFT ID: %s, error: %s", nftIDParam, err)
	}

	if len(nftIDBytes) != iotago.NFTIDLength {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid NFT ID: %s, error: %s", nftIDParam, err)
	}

	var nftID iotago.NFTID
	copy(nftID[:], nftIDBytes)
	return &nftID, nil
}

func ParseFoundryIDParam(c echo.Context) (*iotago.FoundryID, error) {
	foundryIDParam := strings.ToLower(c.Param(ParameterFoundryID))

	foundryIDBytes, err := iotago.DecodeHex(foundryIDParam)
	if err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid foundry ID: %s, error: %s", foundryIDParam, err)
	}

	if len(foundryIDBytes) != iotago.FoundryIDLength {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid foundry ID: %s, error: %s", foundryIDParam, err)
	}

	var foundryID iotago.FoundryID
	copy(foundryID[:], foundryIDBytes)
	return &foundryID, nil
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

func ParseUint32QueryParam(c echo.Context, paramName string, maxValue ...uint32) (uint32, error) {
	intString := strings.ToLower(c.QueryParam(paramName))
	if intString == "" {
		return 0, errors.WithMessagef(ErrInvalidParameter, "parameter \"%s\" not specified", paramName)
	}

	value, err := strconv.ParseUint(intString, 10, 32)
	if err != nil {
		return 0, errors.WithMessagef(ErrInvalidParameter, "invalid value: %s, error: %s", intString, err)
	}

	if len(maxValue) > 0 {
		if uint32(value) > maxValue[0] {
			return 0, errors.WithMessagef(ErrInvalidParameter, "invalid value: %s, higher than the max number %d", intString, maxValue)
		}
	}
	return uint32(value), nil
}

func ParseMilestoneIndexQueryParam(c echo.Context, paramName string) (milestone.Index, error) {
	msIndex, err := ParseUint32QueryParam(c, paramName)
	if err != nil {
		return 0, err
	}
	return milestone.Index(msIndex), nil
}

func ParseUnixTimestampQueryParam(c echo.Context, paramName string) (time.Time, error) {
	timestamp, err := ParseUint32QueryParam(c, paramName)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(int64(timestamp), 0), nil
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

func ParseBech32AddressQueryParam(c echo.Context, prefix iotago.NetworkPrefix, paramName string) (iotago.Address, error) {
	addressParam := strings.ToLower(c.QueryParam(paramName))

	hrp, bech32Address, err := iotago.ParseBech32(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid address: %s, error: %s", addressParam, err)
	}

	if hrp != prefix {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid bech32 address, expected prefix: %s", prefix)
	}

	return bech32Address, nil
}

func ParseHexQueryParam(c echo.Context, paramName string, maxLen int) ([]byte, error) {
	param := c.QueryParam(paramName)

	paramBytes, err := iotago.DecodeHex(param)
	if err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid param: %s, error: %s", paramName, err)
	}
	if len(paramBytes) > maxLen {
		return nil, errors.WithMessage(ErrInvalidParameter, fmt.Sprintf("query parameter %s too long, max. %d bytes but is %d", paramName, maxLen, len(paramBytes)))
	}
	return paramBytes, nil
}

func ParseBoolQueryParam(c echo.Context, paramName string) (bool, error) {
	return strconv.ParseBool(c.QueryParam(paramName))
}
