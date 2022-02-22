package v2

import (
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/utils"
	"io/ioutil"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	messageProcessedTimeout = 1 * time.Second
)

func messageMetadataByID(c echo.Context) (*messageMetadataResponse, error) {

	if !deps.SyncManager.IsNodeAlmostSynced() {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is not synced")
	}

	messageID, err := restapi.ParseMessageIDParam(c)
	if err != nil {
		return nil, err
	}

	cachedMsgMeta := deps.Storage.CachedMessageMetadataOrNil(messageID)
	if cachedMsgMeta == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "message not found: %s", messageID.ToHex())
	}
	defer cachedMsgMeta.Release(true)

	metadata := cachedMsgMeta.Metadata()

	var referencedByMilestone *milestone.Index = nil
	referenced, referencedIndex := metadata.ReferencedWithIndex()
	if referenced {
		referencedByMilestone = &referencedIndex
	}

	messageMetadataResponse := &messageMetadataResponse{
		MessageID:                  metadata.MessageID().ToHex(),
		Parents:                    metadata.Parents().ToHex(),
		Solid:                      metadata.IsSolid(),
		ReferencedByMilestoneIndex: referencedByMilestone,
	}

	if metadata.IsMilestone() {
		messageMetadataResponse.MilestoneIndex = referencedByMilestone
	}

	if referenced {
		inclusionState := "noTransaction"

		conflict := metadata.Conflict()

		if conflict != storage.ConflictNone {
			inclusionState = "conflicting"
			messageMetadataResponse.ConflictReason = &conflict
		} else if metadata.IsIncludedTxInLedger() {
			inclusionState = "included"
		}

		messageMetadataResponse.LedgerInclusionState = &inclusionState
	} else if metadata.IsSolid() {
		// determine info about the quality of the tip if not referenced
		cmi := deps.SyncManager.ConfirmedMilestoneIndex()
		ycri, ocri, err := dag.ConeRootIndexes(Plugin.Daemon().ContextStopped(), deps.Storage, cachedMsgMeta.Retain(), cmi)
		if err != nil {
			if errors.Is(err, common.ErrOperationAborted) {
				return nil, errors.WithMessage(echo.ErrServiceUnavailable, err.Error())
			}
			return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
		}

		// if none of the following checks is true, the tip is non-lazy, so there is no need to promote or reattach
		shouldPromote := false
		shouldReattach := false

		if (cmi - ocri) > milestone.Index(deps.BelowMaxDepth) {
			// if the OCRI to CMI delta is over BelowMaxDepth/below-max-depth, then the tip is lazy and should be reattached
			shouldPromote = false
			shouldReattach = true
		} else if (cmi - ycri) > milestone.Index(deps.MaxDeltaMsgYoungestConeRootIndexToCMI) {
			// if the CMI to YCRI delta is over CfgTipSelMaxDeltaMsgYoungestConeRootIndexToCMI, then the tip is lazy and should be promoted
			shouldPromote = true
			shouldReattach = false
		} else if (cmi - ocri) > milestone.Index(deps.MaxDeltaMsgOldestConeRootIndexToCMI) {
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
	messageID, err := restapi.ParseMessageIDParam(c)
	if err != nil {
		return nil, nil
	}

	cachedMsg := deps.Storage.CachedMessageOrNil(messageID)
	if cachedMsg == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "message not found: %s", messageID.ToHex())
	}
	defer cachedMsg.Release(true)

	return cachedMsg.Message().Message(), nil
}

func messageBytesByID(c echo.Context) ([]byte, error) {
	messageID, err := restapi.ParseMessageIDParam(c)
	if err != nil {
		return nil, err
	}

	cachedMsg := deps.Storage.CachedMessageOrNil(messageID)
	if cachedMsg == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "message not found: %s", messageID.ToHex())
	}
	defer cachedMsg.Release(true)

	return cachedMsg.Message().Data(), nil
}

func childrenIDsByID(c echo.Context) (*childrenResponse, error) {

	messageID, err := restapi.ParseMessageIDParam(c)
	if err != nil {
		return nil, nil
	}

	maxResults := deps.RestAPILimitsMaxResults
	childrenMessageIDs := deps.Storage.ChildrenMessageIDs(messageID, objectstorage.WithIteratorMaxIterations(maxResults))

	return &childrenResponse{
		MessageID:  messageID.ToHex(),
		MaxResults: uint32(maxResults),
		Count:      uint32(len(childrenMessageIDs)),
		Children:   childrenMessageIDs.ToHex(),
	}, nil
}

func sendMessage(c echo.Context) (*messageCreatedResponse, error) {

	if !deps.SyncManager.IsNodeAlmostSynced() {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is not synced")
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

		// Do not validate here, the parents might need to be set
		if _, err := msg.Deserialize(bytes, serializer.DeSeriModeNoValidation, deps.DeserializationParameters); err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
		}
	}

	if msg.ProtocolVersion != iotago.ProtocolVersion {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "invalid message, error: protocolVersion invalid")
	}

	switch payload := msg.Payload.(type) {
	case *iotago.Transaction:
		if payload.Essence.NetworkID != deps.NetworkID {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid payload, error: wrong networkID: %d", payload.Essence.NetworkID)
		}
	default:
	}

	mergedCtx, mergedCtxCancel := utils.MergeContexts(c.Request().Context(), Plugin.Daemon().ContextStopped())
	defer mergedCtxCancel()

	messageID, err := attacher.AttachMessage(mergedCtx, msg)
	if err != nil {
		if errors.Is(err, tangle.ErrMessageAttacherAttachingNotPossible) {
			return nil, errors.WithMessage(echo.ErrServiceUnavailable, err.Error())
		}
		if errors.Is(err, tangle.ErrMessageAttacherInvalidMessage) {
			return nil, errors.WithMessage(restapi.ErrInvalidParameter, err.Error())
		}
		return nil, err
	}

	return &messageCreatedResponse{
		MessageID: messageID.ToHex(),
	}, nil
}
