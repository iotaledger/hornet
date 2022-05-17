package v2

import (
	"io/ioutil"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/iotaledger/hive.go/contextutils"
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

	blockID, err := restapi.ParseMessageIDParam(c)
	if err != nil {
		return nil, err
	}

	cachedBlockMeta := deps.Storage.CachedMessageMetadataOrNil(blockID)
	if cachedBlockMeta == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "message not found: %s", blockID.ToHex())
	}
	defer cachedBlockMeta.Release(true) // meta -1

	metadata := cachedBlockMeta.Metadata()

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

		tipScore, err := deps.TipScoreCalculator.TipScore(Plugin.Daemon().ContextStopped(), cachedBlockMeta.Metadata().MessageID(), cmi)
		if err != nil {
			if errors.Is(err, common.ErrOperationAborted) {
				return nil, errors.WithMessage(echo.ErrServiceUnavailable, err.Error())
			}
			return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
		}

		var shouldPromote bool
		var shouldReattach bool

		switch tipScore {
		case tangle.TipScoreNotFound:
			return nil, errors.WithMessage(echo.ErrInternalServerError, "tip score could not be calculated")
		case tangle.TipScoreOCRIThresholdReached, tangle.TipScoreYCRIThresholdReached:
			shouldPromote = true
			shouldReattach = false
		case tangle.TipScoreBelowMaxDepth:
			shouldPromote = false
			shouldReattach = true
		case tangle.TipScoreHealthy:
			shouldPromote = false
			shouldReattach = false
		}

		messageMetadataResponse.ShouldPromote = &shouldPromote
		messageMetadataResponse.ShouldReattach = &shouldReattach
	}

	return messageMetadataResponse, nil
}

func storageMessageByID(c echo.Context) (*storage.Message, error) {
	blockID, err := restapi.ParseMessageIDParam(c)
	if err != nil {
		return nil, err
	}

	cachedBlock := deps.Storage.CachedMessageOrNil(blockID) // message +1
	if cachedBlock == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "message not found: %s", blockID.ToHex())
	}
	defer cachedBlock.Release(true) // message -1

	return cachedBlock.Message(), nil
}

func messageByID(c echo.Context) (*iotago.Block, error) {
	message, err := storageMessageByID(c)
	if err != nil {
		return nil, err
	}
	return message.Message(), nil
}

func messageBytesByID(c echo.Context) ([]byte, error) {
	message, err := storageMessageByID(c)
	if err != nil {
		return nil, err
	}
	return message.Data(), nil
}

func childrenIDsByID(c echo.Context) (*childrenResponse, error) {

	blockID, err := restapi.ParseMessageIDParam(c)
	if err != nil {
		return nil, err
	}

	maxResults := deps.RestAPILimitsMaxResults
	childrenMessageIDs, err := deps.Storage.ChildrenMessageIDs(blockID, storage.WithIteratorMaxIterations(maxResults))
	if err != nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	return &childrenResponse{
		MessageID:  blockID.ToHex(),
		MaxResults: uint32(maxResults),
		Count:      uint32(len(childrenMessageIDs)),
		Children:   childrenMessageIDs.ToHex(),
	}, nil
}

func sendMessage(c echo.Context) (*messageCreatedResponse, error) {

	if !deps.SyncManager.IsNodeAlmostSynced() {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is not synced")
	}

	mimeType, err := restapi.GetRequestContentType(c, restapi.MIMEApplicationVendorIOTASerializerV1, echo.MIMEApplicationJSON)
	if err != nil {
		return nil, err
	}

	msg := &iotago.Block{}

	switch mimeType {
	case echo.MIMEApplicationJSON:
		if err := c.Bind(msg); err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
		}

	case restapi.MIMEApplicationVendorIOTASerializerV1:
		if c.Request().Body == nil {
			return nil, errors.WithMessage(restapi.ErrInvalidParameter, "invalid message, error: request body missing")
			// bad request
		}

		bytes, err := ioutil.ReadAll(c.Request().Body)
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
		}

		// Do not validate here, the parents might need to be set
		if _, err := msg.Deserialize(bytes, serializer.DeSeriModeNoValidation, deps.ProtocolParameters); err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
		}

	default:
		return nil, echo.ErrUnsupportedMediaType
	}

	if msg.ProtocolVersion != deps.ProtocolParameters.Version {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "invalid message, error: protocolVersion invalid")
	}

	switch payload := msg.Payload.(type) {
	case *iotago.Transaction:
		if payload.Essence.NetworkID != deps.ProtocolParameters.NetworkID() {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid payload, error: wrong networkID: %d", payload.Essence.NetworkID)
		}
	default:
	}

	mergedCtx, mergedCtxCancel := contextutils.MergeContexts(c.Request().Context(), Plugin.Daemon().ContextStopped())
	defer mergedCtxCancel()

	blockID, err := attacher.AttachMessage(mergedCtx, msg)
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
		MessageID: blockID.ToHex(),
	}, nil
}
