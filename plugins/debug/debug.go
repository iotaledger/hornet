package debug

import (
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/restapi"
	"github.com/iotaledger/hornet/v2/plugins/coreapi"
	"github.com/iotaledger/inx-app/httpserver"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (

	// QueryParameterOutputType is used to filter for a certain output type.
	QueryParameterOutputType = "type"
)

func parseOutputTypeQueryParam(c echo.Context) (*iotago.OutputType, error) {
	typeParam := strings.ToLower(c.QueryParam(QueryParameterOutputType))
	var filteredType *iotago.OutputType

	if len(typeParam) > 0 {
		outputTypeInt, err := strconv.ParseInt(typeParam, 10, 32)
		if err != nil {
			return nil, errors.WithMessagef(httpserver.ErrInvalidParameter, "invalid type: %s, error: unknown output type", typeParam)
		}
		outputType := iotago.OutputType(outputTypeInt)
		switch outputType {
		case iotago.OutputBasic, iotago.OutputAlias, iotago.OutputNFT, iotago.OutputFoundry:
		default:
			return nil, errors.WithMessagef(httpserver.ErrInvalidParameter, "invalid type: %s, error: unknown output type", typeParam)
		}
		filteredType = &outputType
	}

	return filteredType, nil
}

func outputsIDs(c echo.Context) (*outputIDsResponse, error) {
	filterType, err := parseOutputTypeQueryParam(c)
	if err != nil {
		return nil, err
	}

	outputIDs := []string{}
	appendConsumerFunc := func(output *utxo.Output) bool {
		outputIDs = append(outputIDs, output.OutputID().ToHex())

		return true
	}

	outputConsumerFunc := appendConsumerFunc

	if filterType != nil {
		outputConsumerFunc = func(output *utxo.Output) bool {
			if output.OutputType() == *filterType {
				return appendConsumerFunc(output)
			}

			return true
		}
	}

	err = deps.UTXOManager.ForEachOutput(outputConsumerFunc, utxo.ReadLockLedger(false))
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading unspent outputs failed, error: %s", err)
	}

	return &outputIDsResponse{
		OutputIDs: outputIDs,
	}, nil
}

func unspentOutputsIDs(c echo.Context) (*outputIDsResponse, error) {
	filterType, err := parseOutputTypeQueryParam(c)
	if err != nil {
		return nil, err
	}

	outputIDs := []string{}
	appendConsumerFunc := func(output *utxo.Output) bool {
		outputIDs = append(outputIDs, output.OutputID().ToHex())

		return true
	}

	outputConsumerFunc := appendConsumerFunc

	if filterType != nil {
		outputConsumerFunc = func(output *utxo.Output) bool {
			if output.OutputType() == *filterType {
				return appendConsumerFunc(output)
			}

			return true
		}
	}

	err = deps.UTXOManager.ForEachUnspentOutput(outputConsumerFunc, utxo.ReadLockLedger(false))
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading unspent outputs failed, error: %s", err)
	}

	return &outputIDsResponse{
		OutputIDs: outputIDs,
	}, nil
}

func spentOutputsIDs(c echo.Context) (*outputIDsResponse, error) {
	filterType, err := parseOutputTypeQueryParam(c)
	if err != nil {
		return nil, err
	}

	outputIDs := []string{}
	appendConsumerFunc := func(spent *utxo.Spent) bool {
		outputIDs = append(outputIDs, spent.OutputID().ToHex())

		return true
	}

	spentConsumerFunc := appendConsumerFunc

	if filterType != nil {
		spentConsumerFunc = func(spent *utxo.Spent) bool {
			if spent.OutputType() == *filterType {
				return appendConsumerFunc(spent)
			}

			return true
		}
	}

	err = deps.UTXOManager.ForEachSpentOutput(spentConsumerFunc, utxo.ReadLockLedger(false))
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading spent outputs failed, error: %s", err)
	}

	return &outputIDsResponse{
		OutputIDs: outputIDs,
	}, nil
}

func milestoneDiff(c echo.Context) (*milestoneDiffResponse, error) {

	msIndex, err := httpserver.ParseMilestoneIndexParam(c, restapi.ParameterMilestoneIndex)
	if err != nil {
		return nil, err
	}

	diff, err := deps.UTXOManager.MilestoneDiffWithoutLocking(msIndex)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "can't load milestone diff for index: %d, error: %s", msIndex, err)
		}

		return nil, errors.WithMessagef(echo.ErrInternalServerError, "can't load milestone diff for index: %d, error: %s", msIndex, err)
	}

	outputs := make([]*coreapi.OutputResponse, len(diff.Outputs))
	spents := make([]*coreapi.OutputResponse, len(diff.Spents))

	for i, output := range diff.Outputs {
		o, err := coreapi.NewOutputResponse(output, diff.Index)
		if err != nil {
			return nil, err
		}
		outputs[i] = o
	}

	for i, spent := range diff.Spents {
		o, err := coreapi.NewSpentResponse(spent, diff.Index)
		if err != nil {
			return nil, err
		}
		spents[i] = o
	}

	return &milestoneDiffResponse{
		MilestoneIndex: msIndex,
		Outputs:        outputs,
		Spents:         spents,
	}, nil
}

//nolint:unparam // even if the error is never used, the structure of all routes should be the same
func requests(_ echo.Context) (*requestsResponse, error) {

	queued, pending, processing := deps.RequestQueue.Requests()
	debugReqs := make([]*request, 0, len(queued)+len(pending)+len(processing))

	for _, req := range queued {
		debugReqs = append(debugReqs, &request{
			BlockID:          req.BlockID.ToHex(),
			Type:             "queued",
			BlockExists:      deps.Storage.ContainsBlock(req.BlockID),
			EnqueueTimestamp: req.EnqueueTime.Format(time.RFC3339),
			MilestoneIndex:   req.MilestoneIndex,
		})
	}

	for _, req := range pending {
		debugReqs = append(debugReqs, &request{
			BlockID:          req.BlockID.ToHex(),
			Type:             "pending",
			BlockExists:      deps.Storage.ContainsBlock(req.BlockID),
			EnqueueTimestamp: req.EnqueueTime.Format(time.RFC3339),
			MilestoneIndex:   req.MilestoneIndex,
		})
	}

	for _, req := range processing {
		debugReqs = append(debugReqs, &request{
			BlockID:          req.BlockID.ToHex(),
			Type:             "processing",
			BlockExists:      deps.Storage.ContainsBlock(req.BlockID),
			EnqueueTimestamp: req.EnqueueTime.Format(time.RFC3339),
			MilestoneIndex:   req.MilestoneIndex,
		})
	}

	return &requestsResponse{
		Requests: debugReqs,
	}, nil
}

func blockCone(c echo.Context) (*blockConeResponse, error) {

	blockID, err := httpserver.ParseBlockIDParam(c, restapi.ParameterBlockID)
	if err != nil {
		return nil, err
	}

	cachedBlockMetaStart := deps.Storage.CachedBlockMetadataOrNil(blockID) // meta +1
	if cachedBlockMetaStart == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "block not found: %s", blockID.ToHex())
	}
	defer cachedBlockMetaStart.Release(true) // meta -1

	if !cachedBlockMetaStart.Metadata().IsSolid() {
		return nil, errors.WithMessagef(echo.ErrServiceUnavailable, "start block is not solid: %s", blockID.ToHex())
	}

	startBlockReferenced, startBlockReferencedAt := cachedBlockMetaStart.Metadata().ReferencedWithIndex()

	snapshotInfo := deps.Storage.SnapshotInfo()
	if snapshotInfo == nil {
		return nil, common.ErrSnapshotInfoNotFound
	}

	entryPointIndex := snapshotInfo.EntryPointIndex()
	entryPoints := []*entryPoint{}
	tanglePath := []*blockWithParents{}

	if err := dag.TraverseParentsOfBlock(
		Plugin.Daemon().ContextStopped(),
		deps.Storage,
		blockID,
		// traversal stops if no more blocks pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedBlockMeta.Release(true) // meta -1

			if referenced, at := cachedBlockMeta.Metadata().ReferencedWithIndex(); referenced {
				if !startBlockReferenced || (at < startBlockReferencedAt) {
					entryPoints = append(entryPoints, &entryPoint{BlockID: cachedBlockMeta.Metadata().BlockID().ToHex(), ReferencedByMilestone: at})

					return false, nil
				}
			}

			return true, nil
		},
		// consumer
		func(cachedBlockMeta *storage.CachedMetadata) error { // meta +1
			cachedBlockMeta.ConsumeMetadata(func(metadata *storage.BlockMetadata) { // meta -1
				id := metadata.BlockID()
				tanglePath = append(tanglePath,
					&blockWithParents{
						BlockID: id.ToHex(),
						Parents: metadata.Parents().ToHex(),
					},
				)
			})

			return nil
		},
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		func(blockID iotago.BlockID) error {
			entryPoints = append(entryPoints, &entryPoint{BlockID: blockID.ToHex(), ReferencedByMilestone: entryPointIndex})

			return nil
		},
		false); err != nil {
		if errors.Is(err, common.ErrOperationAborted) {
			return nil, errors.WithMessagef(echo.ErrServiceUnavailable, "traverse parents failed, error: %s", err)
		}

		return nil, errors.WithMessagef(echo.ErrInternalServerError, "traverse parents failed, error: %s", err)
	}

	if len(entryPoints) == 0 {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "no referenced parents found: %s", blockID.ToHex())
	}

	return &blockConeResponse{
		ConeElementsCount: len(tanglePath),
		EntryPointsCount:  len(entryPoints),
		Cone:              tanglePath,
		EntryPoints:       entryPoints,
	}, nil
}
