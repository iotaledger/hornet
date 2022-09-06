package coreapi

import (
	"io"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/contextutils"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/restapi"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	"github.com/iotaledger/inx-app/httpserver"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	blockProcessedTimeout = 1 * time.Second
)

func blockMetadataByID(c echo.Context) (*blockMetadataResponse, error) {
	blockID, err := httpserver.ParseBlockIDParam(c, restapi.ParameterBlockID)
	if err != nil {
		return nil, err
	}

	cachedBlockMeta := deps.Storage.CachedBlockMetadataOrNil(blockID)
	if cachedBlockMeta == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "block not found: %s", blockID.ToHex())
	}
	defer cachedBlockMeta.Release(true) // meta -1

	metadata := cachedBlockMeta.Metadata()

	referenced, referencedIndex, wfIndex := metadata.ReferencedWithIndexAndWhiteFlagIndex()

	response := &blockMetadataResponse{
		BlockID:                    blockID.ToHex(),
		Parents:                    metadata.Parents().ToHex(),
		Solid:                      metadata.IsSolid(),
		ReferencedByMilestoneIndex: referencedIndex,
	}

	if metadata.IsMilestone() {
		cachedBlock := deps.Storage.CachedBlockOrNil(blockID)
		if cachedBlock == nil {
			return nil, errors.WithMessagef(echo.ErrNotFound, "block not found: %s", blockID.ToHex())
		}
		defer cachedBlock.Release(true)

		milestone := cachedBlock.Block().Milestone()
		if milestone == nil {
			return nil, errors.WithMessagef(echo.ErrNotFound, "milestone for block not found: %s", blockID.ToHex())
		}
		response.MilestoneIndex = milestone.Index
	}

	if referenced {
		response.WhiteFlagIndex = &wfIndex
		response.LedgerInclusionState = "noTransaction"

		conflict := metadata.Conflict()
		if conflict != storage.ConflictNone {
			response.LedgerInclusionState = "conflicting"
			response.ConflictReason = &conflict
		} else if metadata.IsIncludedTxInLedger() {
			response.LedgerInclusionState = "included"
		}
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
	blockID, err := httpserver.ParseBlockIDParam(c, restapi.ParameterBlockID)
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
	mimeType, err := httpserver.GetRequestContentType(c, httpserver.MIMEApplicationVendorIOTASerializerV1, echo.MIMEApplicationJSON)
	if err != nil {
		return nil, err
	}

	iotaBlock := &iotago.Block{}

	switch mimeType {
	case echo.MIMEApplicationJSON:
		if err := c.Bind(iotaBlock); err != nil {
			return nil, errors.WithMessagef(httpserver.ErrInvalidParameter, "invalid block, error: %s", err)
		}

	case httpserver.MIMEApplicationVendorIOTASerializerV1:
		if c.Request().Body == nil {
			// bad request
			return nil, errors.WithMessage(httpserver.ErrInvalidParameter, "invalid block, error: request body missing")
		}

		bytes, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return nil, errors.WithMessagef(httpserver.ErrInvalidParameter, "invalid block, error: %s", err)
		}

		// Do not validate here, the parents might need to be set
		if _, err := iotaBlock.Deserialize(bytes, serializer.DeSeriModeNoValidation, deps.ProtocolManager.Current()); err != nil {
			return nil, errors.WithMessagef(httpserver.ErrInvalidParameter, "invalid block, error: %s", err)
		}

	default:
		return nil, echo.ErrUnsupportedMediaType
	}

	mergedCtx, mergedCtxCancel := contextutils.MergeContexts(c.Request().Context(), Plugin.Daemon().ContextStopped())
	defer mergedCtxCancel()

	blockID, err := attacher.AttachBlock(mergedCtx, iotaBlock)
	if err != nil {
		switch {
		case errors.Is(err, tangle.ErrBlockAttacherInvalidBlock):
			return nil, errors.WithMessagef(httpserver.ErrInvalidParameter, "failed to attach block: %s", err.Error())

		case errors.Is(err, tangle.ErrBlockAttacherAttachingNotPossible):
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "failed to attach block: %s", err.Error())

		case errors.Is(err, tangle.ErrBlockAttacherPoWNotAvailable):
			return nil, errors.WithMessagef(echo.ErrServiceUnavailable, "failed to attach block: %s", err.Error())

		default:
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "failed to attach block: %s", err.Error())
		}
	}

	return &blockCreatedResponse{
		BlockID: blockID.ToHex(),
	}, nil
}
