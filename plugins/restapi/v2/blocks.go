package v2

import (
	"io/ioutil"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/contextutils"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/pkg/common"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/restapi"
	"github.com/iotaledger/hornet/pkg/tangle"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	blockProcessedTimeout = 1 * time.Second
)

func blockMetadataByID(c echo.Context) (*blockMetadataResponse, error) {
	blockID, err := restapi.ParseBlockIDParam(c)
	if err != nil {
		return nil, err
	}

	cachedBlockMeta := deps.Storage.CachedBlockMetadataOrNil(blockID)
	if cachedBlockMeta == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "block not found: %s", blockID.ToHex())
	}
	defer cachedBlockMeta.Release(true) // meta -1

	metadata := cachedBlockMeta.Metadata()

	var referencedByMilestone *milestone.Index = nil
	referenced, referencedIndex := metadata.ReferencedWithIndex()
	if referenced {
		referencedByMilestone = &referencedIndex
	}

	response := &blockMetadataResponse{
		BlockID:                    blockID.ToHex(),
		Parents:                    metadata.Parents().ToHex(),
		Solid:                      metadata.IsSolid(),
		ReferencedByMilestoneIndex: referencedByMilestone,
	}

	if metadata.IsMilestone() {
		response.MilestoneIndex = referencedByMilestone
	}

	if referenced {
		inclusionState := "noTransaction"

		conflict := metadata.Conflict()

		if conflict != storage.ConflictNone {
			inclusionState = "conflicting"
			response.ConflictReason = &conflict
		} else if metadata.IsIncludedTxInLedger() {
			inclusionState = "included"
		}

		response.LedgerInclusionState = &inclusionState
	} else if metadata.IsSolid() {
		// determine info about the quality of the tip if not referenced
		cmi := deps.SyncManager.ConfirmedMilestoneIndex()

		tipScore, err := deps.TipScoreCalculator.TipScore(Plugin.Daemon().ContextStopped(), cachedBlockMeta.Metadata().BlockID(), cmi)
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

		response.ShouldPromote = &shouldPromote
		response.ShouldReattach = &shouldReattach
	}

	return response, nil
}

func storageBlockByID(c echo.Context) (*storage.Block, error) {
	blockID, err := restapi.ParseBlockIDParam(c)
	if err != nil {
		return nil, err
	}

	cachedBlock := deps.Storage.CachedBlockOrNil(blockID) // block +1
	if cachedBlock == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "block not found: %s", blockID.ToHex())
	}
	defer cachedBlock.Release(true) // block -1

	return cachedBlock.Block(), nil
}

func blockByID(c echo.Context) (*iotago.Block, error) {
	block, err := storageBlockByID(c)
	if err != nil {
		return nil, err
	}
	return block.Block(), nil
}

func blockBytesByID(c echo.Context) ([]byte, error) {
	block, err := storageBlockByID(c)
	if err != nil {
		return nil, err
	}
	return block.Data(), nil
}

func sendBlock(c echo.Context) (*blockCreatedResponse, error) {
	mimeType, err := restapi.GetRequestContentType(c, restapi.MIMEApplicationVendorIOTASerializerV1, echo.MIMEApplicationJSON)
	if err != nil {
		return nil, err
	}

	iotaBlock := &iotago.Block{}

	switch mimeType {
	case echo.MIMEApplicationJSON:
		if err := c.Bind(iotaBlock); err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid block, error: %s", err)
		}

	case restapi.MIMEApplicationVendorIOTASerializerV1:
		if c.Request().Body == nil {
			return nil, errors.WithMessage(restapi.ErrInvalidParameter, "invalid block, error: request body missing")
			// bad request
		}

		bytes, err := ioutil.ReadAll(c.Request().Body)
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid block, error: %s", err)
		}

		// Do not validate here, the parents might need to be set
		if _, err := iotaBlock.Deserialize(bytes, serializer.DeSeriModeNoValidation, deps.ProtocolManager.Current()); err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid block, error: %s", err)
		}

	default:
		return nil, echo.ErrUnsupportedMediaType
	}

	if iotaBlock.ProtocolVersion != deps.ProtocolManager.Current().Version {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "invalid block, error: protocolVersion invalid")
	}

	switch payload := iotaBlock.Payload.(type) {
	case *iotago.Transaction:
		if payload.Essence.NetworkID != deps.ProtocolManager.Current().NetworkID() {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid payload, error: wrong networkID: %d", payload.Essence.NetworkID)
		}
	default:
	}

	mergedCtx, mergedCtxCancel := contextutils.MergeContexts(c.Request().Context(), Plugin.Daemon().ContextStopped())
	defer mergedCtxCancel()

	blockID, err := attacher.AttachBlock(mergedCtx, iotaBlock)
	if err != nil {
		if errors.Is(err, tangle.ErrBlockAttacherAttachingNotPossible) {
			return nil, errors.WithMessage(echo.ErrServiceUnavailable, err.Error())
		}
		if errors.Is(err, tangle.ErrBlockAttacherInvalidBlock) {
			return nil, errors.WithMessage(restapi.ErrInvalidParameter, err.Error())
		}
		return nil, err
	}

	return &blockCreatedResponse{
		BlockID: blockID.ToHex(),
	}, nil
}
