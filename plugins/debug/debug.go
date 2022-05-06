package debug

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/restapi"
	restapiv2 "github.com/gohornet/hornet/plugins/restapi/v2"
	"github.com/iotaledger/hive.go/kvstore"
)

func outputsIDs(c echo.Context) (*outputIDsResponse, error) {
	filterType, err := restapi.ParseOutputTypeQueryParam(c)
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
	filterType, err := restapi.ParseOutputTypeQueryParam(c)
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
	filterType, err := restapi.ParseOutputTypeQueryParam(c)
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

	msIndex, err := restapi.ParseMilestoneIndexParam(c, restapi.ParameterMilestoneIndex)
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

	outputs := make([]*restapiv2.OutputResponse, len(diff.Outputs))
	spents := make([]*restapiv2.OutputResponse, len(diff.Spents))

	for i, output := range diff.Outputs {
		o, err := restapiv2.NewOutputResponse(output, diff.Index)
		if err != nil {
			return nil, err
		}
		outputs[i] = o
	}

	for i, spent := range diff.Spents {
		o, err := restapiv2.NewSpentResponse(spent, diff.Index)
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
			MessageID:        req.MessageID.ToHex(),
			Type:             "queued",
			MessageExists:    deps.Storage.ContainsMessage(req.MessageID),
			EnqueueTimestamp: req.EnqueueTime.Format(time.RFC3339),
			MilestoneIndex:   req.MilestoneIndex,
		})
	}

	for _, req := range pending {
		debugReqs = append(debugReqs, &request{
			MessageID:        req.MessageID.ToHex(),
			Type:             "pending",
			MessageExists:    deps.Storage.ContainsMessage(req.MessageID),
			EnqueueTimestamp: req.EnqueueTime.Format(time.RFC3339),
			MilestoneIndex:   req.MilestoneIndex,
		})
	}

	for _, req := range processing {
		debugReqs = append(debugReqs, &request{
			MessageID:        req.MessageID.ToHex(),
			Type:             "processing",
			MessageExists:    deps.Storage.ContainsMessage(req.MessageID),
			EnqueueTimestamp: req.EnqueueTime.Format(time.RFC3339),
			MilestoneIndex:   req.MilestoneIndex,
		})
	}

	return &requestsResponse{
		Requests: debugReqs,
	}, nil
}

func messageCone(c echo.Context) (*messageConeResponse, error) {

	messageID, err := restapi.ParseMessageIDParam(c)
	if err != nil {
		return nil, err
	}

	cachedMsgMetaStart := deps.Storage.CachedMessageMetadataOrNil(messageID) // meta +1
	if cachedMsgMetaStart == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "message not found: %s", messageID.ToHex())
	}
	defer cachedMsgMetaStart.Release(true) // meta -1

	if !cachedMsgMetaStart.Metadata().IsSolid() {
		return nil, errors.WithMessagef(echo.ErrServiceUnavailable, "start message is not solid: %s", messageID.ToHex())
	}

	startMsgReferened, startMsgReferenedAt := cachedMsgMetaStart.Metadata().ReferencedWithIndex()

	entryPointIndex := deps.Storage.SnapshotInfo().EntryPointIndex
	entryPoints := []*entryPoint{}
	tanglePath := []*messageWithParents{}

	if err := dag.TraverseParentsOfMessage(
		Plugin.Daemon().ContextStopped(),
		deps.Storage,
		messageID,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			if referenced, at := cachedMsgMeta.Metadata().ReferencedWithIndex(); referenced {
				if !startMsgReferened || (at < startMsgReferenedAt) {
					entryPoints = append(entryPoints, &entryPoint{MessageID: cachedMsgMeta.Metadata().MessageID().ToHex(), ReferencedByMilestone: at})
					return false, nil
				}
			}

			return true, nil
		},
		// consumer
		func(cachedMsgMeta *storage.CachedMetadata) error { // meta +1
			cachedMsgMeta.ConsumeMetadata(func(metadata *storage.MessageMetadata) { // meta -1
				tanglePath = append(tanglePath,
					&messageWithParents{
						MessageID: metadata.MessageID().ToHex(),
						Parents:   metadata.Parents().ToHex(),
					},
				)
			})

			return nil
		},
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		func(messageID hornet.MessageID) error {
			entryPoints = append(entryPoints, &entryPoint{MessageID: messageID.ToHex(), ReferencedByMilestone: entryPointIndex})
			return nil
		},
		false); err != nil {
		if errors.Is(err, common.ErrOperationAborted) {
			return nil, errors.WithMessagef(echo.ErrServiceUnavailable, "traverse parents failed, error: %s", err)
		}
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "traverse parents failed, error: %s", err)
	}

	if len(entryPoints) == 0 {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "no referenced parents found: %s", messageID.ToHex())
	}

	return &messageConeResponse{
		ConeElementsCount: len(tanglePath),
		EntryPointsCount:  len(entryPoints),
		Cone:              tanglePath,
		EntryPoints:       entryPoints,
	}, nil
}
