package v1

import (
	"io/ioutil"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/restapi/common"
	tangleplugin "github.com/gohornet/hornet/plugins/tangle"
	"github.com/gohornet/hornet/plugins/urts"
)

var (
	messageProcessedTimeout = 1 * time.Second
)

func messageMetadataByMessageID(messageID *hornet.MessageID) (*messageMetadataResponse, error) {
	cachedMsgMeta := tangle.GetCachedMessageMetadataOrNil(messageID)
	if cachedMsgMeta == nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "message not found: %s", messageID.Hex())
	}
	defer cachedMsgMeta.Release(true)

	metadata := cachedMsgMeta.GetMetadata()

	var referencedByMilestone *milestone.Index = nil
	referenced, referencedIndex := metadata.GetReferenced()
	if referenced {
		referencedByMilestone = &referencedIndex
	}

	messageMetadataResponse := &messageMetadataResponse{
		MessageID:             metadata.GetMessageID().Hex(),
		Parent1:               metadata.GetParent1MessageID().Hex(),
		Parent2:               metadata.GetParent2MessageID().Hex(),
		Solid:                 metadata.IsSolid(),
		ReferencedByMilestoneIndex: referencedByMilestone,
	}

	if referenced {
		inclusionState := "noTransaction"

		if metadata.IsConflictingTx() {
			inclusionState = "conflicting"
		} else if metadata.IsIncludedTxInLedger() {
			inclusionState = "included"
		}

		messageMetadataResponse.LedgerInclusionState = &inclusionState
	} else if metadata.IsSolid() {
		// determine info about the quality of the tip if not referenced
		lsmi := tangle.GetSolidMilestoneIndex()
		ycri, ocri := dag.GetConeRootIndexes(cachedMsgMeta.Retain(), lsmi)

		// if none of the following checks is true, the tip is non-lazy, so there is no need to promote or reattach
		shouldPromote := false
		shouldReattach := false

		if (lsmi - ocri) > milestone.Index(config.NodeConfig.GetInt(config.CfgTipSelBelowMaxDepth)) {
			// if the OCRI to LSMI delta is over BelowMaxDepth/below-max-depth, then the tip is lazy and should be reattached
			shouldPromote = false
			shouldReattach = true
		} else if (lsmi - ycri) > milestone.Index(config.NodeConfig.GetInt(config.CfgTipSelMaxDeltaMsgYoungestConeRootIndexToLSMI)) {
			// if the LSMI to YCRI delta is over CfgTipSelMaxDeltaMsgYoungestConeRootIndexToLSMI, then the tip is lazy and should be promoted
			shouldPromote = true
			shouldReattach = false
		} else if (lsmi - ocri) > milestone.Index(config.NodeConfig.GetInt(config.CfgTipSelMaxDeltaMsgOldestConeRootIndexToLSMI)) {
			// if the OCRI to LSMI delta is over CfgTipSelMaxDeltaMsgOldestConeRootIndexToLSMI, the tip is semi-lazy and should be promoted
			shouldPromote = true
			shouldReattach = false
		}

		messageMetadataResponse.ShouldPromote = &shouldPromote
		messageMetadataResponse.ShouldReattach = &shouldReattach
	}

	return messageMetadataResponse, nil
}

func messageMetadataByID(c echo.Context) (*messageMetadataResponse, error) {
	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message ID: %s, error: %w", messageIDHex, err)
	}

	return messageMetadataByMessageID(messageID)
}

func messageByID(c echo.Context) (*iotago.Message, error) {
	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message ID: %s, error: %w", messageIDHex, err)
	}

	cachedMsg := tangle.GetCachedMessageOrNil(messageID)
	if cachedMsg == nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "message not found: %s", messageIDHex)
	}
	defer cachedMsg.Release(true)

	return cachedMsg.GetMessage().GetMessage(), nil
}

func messageBytesByID(c echo.Context) ([]byte, error) {
	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message ID: %s, error: %w", messageIDHex, err)
	}

	cachedMsg := tangle.GetCachedMessageOrNil(messageID)
	if cachedMsg == nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "message not found: %s", messageIDHex)
	}
	defer cachedMsg.Release(true)

	return cachedMsg.GetMessage().GetData(), nil
}

func childrenIDsByID(c echo.Context) (*childrenResponse, error) {
	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message ID: %s, error: %w", messageIDHex, err)
	}

	maxResults := config.NodeConfig.GetInt(config.CfgRestAPILimitsMaxResults)

	childrenMessageIDsHex := []string{}
	for _, childrenMessageID := range tangle.GetChildrenMessageIDs(messageID, maxResults) {
		childrenMessageIDsHex = append(childrenMessageIDsHex, childrenMessageID.Hex())
	}

	return &childrenResponse{
		MessageID:  messageID.Hex(),
		MaxResults: uint32(maxResults),
		Count:      uint32(len(childrenMessageIDsHex)),
		Children:   childrenMessageIDsHex,
	}, nil
}

func messageIDsByIndex(c echo.Context) (*messageIDsByIndexResponse, error) {
	index := c.QueryParam("index")

	if index == "" {
		return nil, errors.WithMessage(common.ErrInvalidParameter, "query parameter index empty")
	}

	maxResults := config.NodeConfig.GetInt(config.CfgRestAPILimitsMaxResults)

	messageIDsHex := []string{}
	for _, messageID := range tangle.GetIndexMessageIDs(index, maxResults) {
		messageIDsHex = append(messageIDsHex, messageID.Hex())
	}

	return &messageIDsByIndexResponse{
		Index:      index,
		MaxResults: uint32(maxResults),
		Count:      uint32(len(messageIDsHex)),
		MessageIDs: messageIDsHex,
	}, nil
}

func sendMessage(c echo.Context) (*messageCreatedResponse, error) {

	msg := &iotago.Message{}

	contentType := c.Request().Header.Get(echo.HeaderContentType)

	if strings.HasPrefix(contentType, echo.MIMEApplicationJSON) {
		if err := c.Bind(msg); err != nil {
			return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message, error: %w", err)
		}
	} else {
		if c.Request().Body == nil {
			return nil, errors.WithMessage(common.ErrInvalidParameter, "invalid message, error: request body missing")
			// bad request
		}

		bytes, err := ioutil.ReadAll(c.Request().Body)
		if err != nil {
			return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message, error: %w", err)
		}

		if _, err := msg.Deserialize(bytes, iotago.DeSeriModeNoValidation); err != nil {
			return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message, error: %w", err)
		}
	}

	var emptyMessageID = hornet.MessageID{}
	if msg.Parent1 == emptyMessageID || msg.Parent2 == emptyMessageID {

		tips, err := urts.TipSelector.SelectNonLazyTips()

		if err != nil {
			if err == tangle.ErrNodeNotSynced || err == tipselect.ErrNoTipsAvailable {
				return nil, errors.WithMessage(common.ErrServiceUnavailable, err.Error())
			}
			return nil, errors.WithMessage(common.ErrInternalError, err.Error())
		}
		msg.Parent1 = *tips[0]
		msg.Parent2 = *tips[1]
	}

	if msg.Nonce == 0 {
		//TODO: Do PoW
	}

	// ToDo: check PoW

	message, err := tangle.NewMessage(msg, iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message, error: %w", err)
	}

	msgProcessedChan := tangleplugin.RegisterMessageProcessedEvent(message.GetMessageID())

	if err := gossip.MessageProcessor().Emit(message); err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message, error: %w", err)
	}

	utils.WaitForChannelClosed(msgProcessedChan, messageProcessedTimeout)

	return &messageCreatedResponse{
		MessageID: message.GetMessageID().Hex(),
	}, nil
}
