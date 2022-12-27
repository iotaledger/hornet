package v1

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/serializer"
	"github.com/iotaledger/hornet/pkg/common"
	"github.com/iotaledger/hornet/pkg/dag"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/pow"
	"github.com/iotaledger/hornet/pkg/restapi"
	"github.com/iotaledger/hornet/pkg/tipselect"
	"github.com/iotaledger/hornet/pkg/utils"
	iotago "github.com/iotaledger/iota.go/v2"
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
	defer cachedMsgMeta.Release(true) // meta -1

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
		ycri, ocri, err := dag.ConeRootIndexes(Plugin.Daemon().ContextStopped(), deps.Storage, cachedMsgMeta.Retain(), cmi) // meta pass +1
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
		return nil, err
	}

	cachedMsg := deps.Storage.CachedMessageOrNil(messageID) // message +1
	if cachedMsg == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "message not found: %s", messageID.ToHex())
	}
	defer cachedMsg.Release(true) // message -1

	return cachedMsg.Message().Message(), nil
}

func messageBytesByID(c echo.Context) ([]byte, error) {
	messageID, err := restapi.ParseMessageIDParam(c)
	if err != nil {
		return nil, err
	}

	cachedMsg := deps.Storage.CachedMessageOrNil(messageID) // message +1
	if cachedMsg == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "message not found: %s", messageID.ToHex())
	}
	defer cachedMsg.Release(true) // message -1

	return cachedMsg.Message().Data(), nil
}

func childrenIDsByID(c echo.Context) (*childrenResponse, error) {

	messageID, err := restapi.ParseMessageIDParam(c)
	if err != nil {
		return nil, err
	}

	maxResults := deps.RestAPILimitsMaxResults
	childrenMessageIDs, err := deps.Storage.ChildrenMessageIDs(messageID, storage.WithIteratorMaxIterations(maxResults))
	if err != nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

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

	maxResults := deps.RestAPILimitsMaxResults
	indexMessageIDs := deps.Storage.IndexMessageIDs(indexBytes, storage.WithIteratorMaxIterations(maxResults))

	return &messageIDsByIndexResponse{
		Index:      index,
		MaxResults: uint32(maxResults),
		Count:      uint32(len(indexMessageIDs)),
		MessageIDs: indexMessageIDs.ToHex(),
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

		bytes, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
		}

		if _, err := msg.Deserialize(bytes, serializer.DeSeriModeNoValidation); err != nil {
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

	var refreshTipsFunc pow.RefreshTipsFunc

	if len(msg.Parents) == 0 {
		if deps.TipSelector == nil {
			return nil, errors.WithMessage(restapi.ErrInvalidParameter, "invalid message, error: no parents given and node tipselection disabled")
		}

		if !powEnabled {
			return nil, errors.WithMessage(restapi.ErrInvalidParameter, "invalid message, error: no parents given and node PoW is disabled")
		}

		tips, err := deps.TipSelector.SelectNonLazyTips()
		if err != nil {
			if errors.Is(err, common.ErrNodeNotSynced) || errors.Is(err, tipselect.ErrNoTipsAvailable) {
				return nil, errors.WithMessagef(echo.ErrServiceUnavailable, "tipselection failed, error: %s", err.Error())
			}
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "tipselection failed, error: %s", err.Error())
		}
		msg.Parents = tips.ToSliceOfArrays()

		// this function pointer is used to refresh the tips of a message
		// if no parents were given and the PoW takes longer than a configured duration.
		// only allow to update tips during proof of work if no parents were given
		refreshTipsFunc = deps.TipSelector.SelectNonLazyTips
	}

	if msg.Nonce == 0 {
		score, err := msg.POW()
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
		}

		if score < deps.MinPoWScore {
			if !powEnabled {
				return nil, errors.WithMessage(restapi.ErrInvalidParameter, "proof of work is not enabled on this node")
			}

			mergedCtx, mergedCtxCancel := utils.MergeContexts(c.Request().Context(), Plugin.Daemon().ContextStopped())
			defer mergedCtxCancel()

			ts := time.Now()
			messageSize, err := deps.PoWHandler.DoPoW(mergedCtx, msg, powWorkerCount, refreshTipsFunc)
			if err != nil {
				return nil, err
			}
			deps.RestAPIMetrics.TriggerPoWCompleted(messageSize, time.Since(ts))
		}
	}

	message, err := storage.NewMessage(msg, serializer.DeSeriModePerformValidation)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
	}

	msgProcessedChan := deps.Tangle.RegisterMessageProcessedEvent(message.MessageID())

	if err := deps.MessageProcessor.Emit(message); err != nil {
		deps.Tangle.DeregisterMessageProcessedEvent(message.MessageID())
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
	}

	// wait for at most "messageProcessedTimeout" for the message to be processed
	ctx, cancel := context.WithTimeout(context.Background(), messageProcessedTimeout)
	defer cancel()

	if err := utils.WaitForChannelClosed(ctx, msgProcessedChan); errors.Is(err, context.DeadlineExceeded) {
		deps.Tangle.DeregisterMessageProcessedEvent(message.MessageID())
	}

	return &messageCreatedResponse{
		MessageID: message.MessageID().ToHex(),
	}, nil
}
