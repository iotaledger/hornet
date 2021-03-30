package v1

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/utils"
	restapiplugin "github.com/gohornet/hornet/plugins/restapi"
	"github.com/gohornet/hornet/plugins/urts"
	"github.com/iotaledger/hive.go/objectstorage"
	iotago "github.com/iotaledger/iota.go/v2"
)

var (
	messageProcessedTimeout = 1 * time.Second
)

func messageMetadataByID(c echo.Context) (*messageMetadataResponse, error) {

	if !deps.Storage.IsNodeAlmostSynced() {
		return nil, errors.WithMessage(restapi.ErrServiceUnavailable, "node is not synced")
	}

	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message ID: %s, error: %s", messageIDHex, err)
	}

	cachedMsgMeta := deps.Storage.GetCachedMessageMetadataOrNil(messageID)
	if cachedMsgMeta == nil {
		return nil, errors.WithMessagef(restapi.ErrNotFound, "message not found: %s", messageID.ToHex())
	}
	defer cachedMsgMeta.Release(true)

	metadata := cachedMsgMeta.GetMetadata()

	var referencedByMilestone *milestone.Index = nil
	referenced, referencedIndex := metadata.GetReferenced()
	if referenced {
		referencedByMilestone = &referencedIndex
	}

	messageMetadataResponse := &messageMetadataResponse{
		MessageID:                  metadata.GetMessageID().ToHex(),
		Parents:                    metadata.GetParents().ToHex(),
		Solid:                      metadata.IsSolid(),
		ReferencedByMilestoneIndex: referencedByMilestone,
	}

	if metadata.IsMilestone() {
		messageMetadataResponse.MilestoneIndex = referencedByMilestone
	}

	if referenced {
		inclusionState := "noTransaction"

		conflict := metadata.GetConflict()

		if conflict != storage.ConflictNone {
			inclusionState = "conflicting"
			messageMetadataResponse.ConflictReason = &conflict
		} else if metadata.IsIncludedTxInLedger() {
			inclusionState = "included"
		}

		messageMetadataResponse.LedgerInclusionState = &inclusionState
	} else if metadata.IsSolid() {
		// determine info about the quality of the tip if not referenced
		cmi := deps.Storage.GetConfirmedMilestoneIndex()
		ycri, ocri := dag.GetConeRootIndexes(deps.Storage, cachedMsgMeta.Retain(), cmi)

		// if none of the following checks is true, the tip is non-lazy, so there is no need to promote or reattach
		shouldPromote := false
		shouldReattach := false

		if (cmi - ocri) > milestone.Index(deps.BelowMaxDepth) {
			// if the OCRI to CMI delta is over BelowMaxDepth/below-max-depth, then the tip is lazy and should be reattached
			shouldPromote = false
			shouldReattach = true
		} else if (cmi - ycri) > milestone.Index(deps.NodeConfig.Int(urts.CfgTipSelMaxDeltaMsgYoungestConeRootIndexToCMI)) {
			// if the CMI to YCRI delta is over CfgTipSelMaxDeltaMsgYoungestConeRootIndexToCMI, then the tip is lazy and should be promoted
			shouldPromote = true
			shouldReattach = false
		} else if (cmi - ocri) > milestone.Index(deps.NodeConfig.Int(urts.CfgTipSelMaxDeltaMsgOldestConeRootIndexToCMI)) {
			// if the OCRI to CMI delta is over CfgTipSelMaxDeltaMsgOldestConeRootIndexToCMI, the tip is semi-lazy and should be promoted
			shouldPromote = true
			shouldReattach = false
		}

		messageMetadataResponse.ShouldPromote = &shouldPromote
		messageMetadataResponse.ShouldReattach = &shouldReattach
	}

	return messageMetadataResponse, nil
}

func messageByID(c echo.Context) (*iotago.Message, error) {
	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message ID: %s, error: %s", messageIDHex, err)
	}

	cachedMsg := deps.Storage.GetCachedMessageOrNil(messageID)
	if cachedMsg == nil {
		return nil, errors.WithMessagef(restapi.ErrNotFound, "message not found: %s", messageIDHex)
	}
	defer cachedMsg.Release(true)

	return cachedMsg.GetMessage().GetMessage(), nil
}

func messageBytesByID(c echo.Context) ([]byte, error) {
	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message ID: %s, error: %s", messageIDHex, err)
	}

	cachedMsg := deps.Storage.GetCachedMessageOrNil(messageID)
	if cachedMsg == nil {
		return nil, errors.WithMessagef(restapi.ErrNotFound, "message not found: %s", messageIDHex)
	}
	defer cachedMsg.Release(true)

	return cachedMsg.GetMessage().GetData(), nil
}

func childrenIDsByID(c echo.Context) (*childrenResponse, error) {
	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message ID: %s, error: %s", messageIDHex, err)
	}

	maxResults := deps.NodeConfig.Int(restapiplugin.CfgRestAPILimitsMaxResults)
	childrenMessageIDs := deps.Storage.GetChildrenMessageIDs(messageID, objectstorage.WithIteratorMaxIterations(maxResults))

	return &childrenResponse{
		MessageID:  messageID.ToHex(),
		MaxResults: uint32(maxResults),
		Count:      uint32(len(childrenMessageIDs)),
		Children:   childrenMessageIDs.ToHex(),
	}, nil
}

func messageIDsByIndex(c echo.Context) (*messageIDsByIndexResponse, error) {
	index := c.QueryParam("index")

	if index == "" {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "query parameter index empty")
	}

	indexBytes, err := hex.DecodeString(index)
	if err != nil {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "query parameter index invalid hex")
	}

	if len(indexBytes) > storage.IndexationIndexLength {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, fmt.Sprintf("query parameter index too long, max. %d bytes but is %d", storage.IndexationIndexLength, len(indexBytes)))
	}

	maxResults := deps.NodeConfig.Int(restapiplugin.CfgRestAPILimitsMaxResults)
	indexMessageIDs := deps.Storage.GetIndexMessageIDs(indexBytes, objectstorage.WithIteratorMaxIterations(maxResults))

	return &messageIDsByIndexResponse{
		Index:      index,
		MaxResults: uint32(maxResults),
		Count:      uint32(len(indexMessageIDs)),
		MessageIDs: indexMessageIDs.ToHex(),
	}, nil
}

func sendMessage(c echo.Context) (*messageCreatedResponse, error) {

	if !deps.Storage.IsNodeAlmostSynced() {
		return nil, errors.WithMessage(restapi.ErrServiceUnavailable, "node is not synced")
	}

	msg := &iotago.Message{}

	contentType := c.Request().Header.Get(echo.HeaderContentType)

	if strings.HasPrefix(contentType, echo.MIMEApplicationJSON) {
		if err := c.Bind(msg); err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
		}
	} else {
		if c.Request().Body == nil {
			return nil, errors.WithMessage(restapi.ErrInvalidParameter, "invalid message, error: request body missing")
			// bad request
		}

		bytes, err := ioutil.ReadAll(c.Request().Body)
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
		}

		if _, err := msg.Deserialize(bytes, iotago.DeSeriModeNoValidation); err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
		}
	}

	if msg.NetworkID == 0 && msg.Nonce != 0 {
		// Message was PoWed without the correct networkId being set, so reject it
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "invalid message, error: PoW done but networkId missing")
	}

	if msg.NetworkID == 0 {
		msg.NetworkID = deps.NetworkID
	}

	if len(msg.Parents) == 0 {
		tips, err := deps.TipSelector.SelectNonLazyTips()
		if err != nil {
			if err == common.ErrNodeNotSynced || err == tipselect.ErrNoTipsAvailable {
				return nil, errors.WithMessage(restapi.ErrServiceUnavailable, err.Error())
			}
			return nil, errors.WithMessage(restapi.ErrInternalError, err.Error())
		}
		msg.Parents = tips.ToSliceOfArrays()
	}

	if msg.Nonce == 0 {
		score, err := msg.POW()
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
		}

		if score < deps.MinPowScore {
			if !powEnabled {
				return nil, errors.WithMessage(restapi.ErrInvalidParameter, "proof of work is not enabled on this node")
			}

			if err := deps.PoWHandler.DoPoW(msg, nil, powWorkerCount); err != nil {
				return nil, err
			}
		}
	}

	message, err := storage.NewMessage(msg, iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
	}

	msgProcessedChan := deps.Tangle.RegisterMessageProcessedEvent(message.GetMessageID())

	if err := deps.MessageProcessor.Emit(message); err != nil {
		deps.Tangle.DeregisterMessageProcessedEvent(message.GetMessageID())
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
	}

	// wait for at most "messageProcessedTimeout" for the message to be processed
	ctx, cancel := context.WithTimeout(context.Background(), messageProcessedTimeout)
	defer cancel()

	if err := utils.WaitForChannelClosed(ctx, msgProcessedChan); err == context.DeadlineExceeded {
		deps.Tangle.DeregisterMessageProcessedEvent(message.GetMessageID())
	}

	return &messageCreatedResponse{
		MessageID: message.GetMessageID().ToHex(),
	}, nil
}
