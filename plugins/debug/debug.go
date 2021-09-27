package debug

import (
	"context"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
	v1 "github.com/gohornet/hornet/plugins/restapi/v1"
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go/v2"
)

func computeWhiteFlagMutations(c echo.Context) (*computeWhiteFlagMutationsResponse, error) {

	request := &computeWhiteFlagMutationsRequest{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid request, error: %s", err)
	}

	// check if the requested milestone index would be the next one
	if request.Index > deps.SyncManager.ConfirmedMilestoneIndex()+1 {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, common.ErrNodeNotSynced.Error())
	}

	if len(request.Parents) < 1 {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "no parents given")
	}

	parents, err := hornet.MessageIDsFromHex(request.Parents)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid parents, error: %s", err)
	}

	// register all parents for message solid events
	// this has to be done, even if the parents may be solid already, to prevent race conditions
	msgSolidEventChans := make([]chan struct{}, len(parents))
	for i, parent := range parents {
		msgSolidEventChans[i] = deps.Tangle.RegisterMessageSolidEvent(parent)
	}

	// check all parents for solidity
	for _, parent := range parents {
		cachedMsgMeta := deps.Storage.CachedMessageMetadataOrNil(parent)
		if cachedMsgMeta == nil {
			if deps.Storage.SolidEntryPointsContain(parent) {
				// deregister the event, because the parent is already solid (this also fires the event)
				deps.Tangle.DeregisterMessageSolidEvent(parent)
			}
			continue
		}

		cachedMsgMeta.ConsumeMetadata(func(metadata *storage.MessageMetadata) { // metadata -1
			if !metadata.IsSolid() {
				return
			}

			// deregister the event, because the parent is already solid (this also fires the event)
			deps.Tangle.DeregisterMessageSolidEvent(parent)
		})
	}

	messagesMemcache := storage.NewMessagesMemcache(deps.Storage)
	metadataMemcache := storage.NewMetadataMemcache(deps.Storage)

	defer func() {
		// deregister the events to free the memory
		for _, parent := range parents {
			deps.Tangle.DeregisterMessageSolidEvent(parent)
		}

		// release all messages at the end
		messagesMemcache.Cleanup(true)

		// Release all message metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	// check if all requested parents are solid
	solid, _ := deps.Tangle.SolidQueueCheck(messagesMemcache, metadataMemcache, request.Index, parents, nil)

	if !solid {
		// wait for at most "whiteFlagParentsSolidTimeout" for the parents to become solid
		ctx, cancel := context.WithTimeout(context.Background(), whiteflagParentsSolidTimeout)
		defer cancel()

		for _, msgSolidEventChan := range msgSolidEventChans {
			// wait until the message is solid
			if err := utils.WaitForChannelClosed(ctx, msgSolidEventChan); err != nil {
				return nil, errors.WithMessage(echo.ErrServiceUnavailable, "parents not solid")
			}
		}
	}

	// at this point all parents are solid
	// compute merkle tree root
	mutations, err := whiteflag.ComputeWhiteFlagMutations(deps.Storage, request.Index, metadataMemcache, messagesMemcache, parents)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "failed to compute white flag mutations: %s", err)
	}

	return &computeWhiteFlagMutationsResponse{
		MerkleTreeHash: hex.EncodeToString(mutations.MerkleTreeHash[:]),
	}, nil
}

func typeFilterFromParams(c echo.Context) ([]utxo.UTXOIterateOption, error) {
	var opts []utxo.UTXOIterateOption

	typeParam := strings.ToLower(c.QueryParam("type"))

	if len(typeParam) > 0 {
		outputTypeInt, err := strconv.ParseInt(typeParam, 10, 32)
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid type: %s, error: unknown output type", typeParam)
		}
		outputType := iotago.OutputType(outputTypeInt)
		if outputType != iotago.OutputSigLockedSingleOutput && outputType != iotago.OutputSigLockedDustAllowanceOutput {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid type: %s, error: unknown output type", typeParam)
		}
		return append(opts, utxo.FilterOutputType(outputType)), nil
	}
	return opts, nil
}

func outputsIDs(c echo.Context) (*outputIDsResponse, error) {

	outputIDs := []string{}
	outputConsumerFunc := func(output *utxo.Output) bool {
		outputIDs = append(outputIDs, output.OutputID().ToHex())
		return true
	}

	opts := []utxo.UTXOIterateOption{
		utxo.ReadLockLedger(false),
	}

	filter, err := typeFilterFromParams(c)
	if err != nil {
		return nil, err
	}
	opts = append(opts, filter...)

	err = deps.UTXOManager.ForEachOutput(outputConsumerFunc, opts...)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading unspent outputs failed, error: %s", err)
	}

	return &outputIDsResponse{
		OutputIDs: outputIDs,
	}, nil
}

func unspentOutputsIDs(c echo.Context) (*outputIDsResponse, error) {

	outputIDs := []string{}
	outputConsumerFunc := func(output *utxo.Output) bool {
		outputIDs = append(outputIDs, output.OutputID().ToHex())
		return true
	}

	opts := []utxo.UTXOIterateOption{
		utxo.ReadLockLedger(false),
	}

	filter, err := typeFilterFromParams(c)
	if err != nil {
		return nil, err
	}
	opts = append(opts, filter...)

	err = deps.UTXOManager.ForEachUnspentOutput(outputConsumerFunc, opts...)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading unspent outputs failed, error: %s", err)
	}

	return &outputIDsResponse{
		OutputIDs: outputIDs,
	}, nil
}

func spentOutputsIDs(c echo.Context) (*outputIDsResponse, error) {

	outputIDs := []string{}

	spentConsumerFunc := func(spent *utxo.Spent) bool {
		outputIDs = append(outputIDs, spent.OutputID().ToHex())
		return true
	}

	opts := []utxo.UTXOIterateOption{
		utxo.ReadLockLedger(false),
	}

	filter, err := typeFilterFromParams(c)
	if err != nil {
		return nil, err
	}
	opts = append(opts, filter...)

	err = deps.UTXOManager.ForEachSpentOutput(spentConsumerFunc, opts...)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading spent outputs failed, error: %s", err)
	}

	return &outputIDsResponse{
		OutputIDs: outputIDs,
	}, nil
}

func addresses(_ echo.Context) (*addressesResponse, error) {

	addressMap := map[string]*address{}

	outputConsumerFunc := func(output *utxo.Output) bool {
		if addr, exists := addressMap[output.Address().String()]; exists {
			// add balance to total balance
			addr.Balance += output.Amount()
			return true
		}

		addressMap[output.Address().String()] = &address{
			AddressType: output.Address().Type(),
			Address:     output.Address().String(),
			Balance:     output.Amount(),
		}

		return true
	}

	err := deps.UTXOManager.ForEachUnspentOutput(outputConsumerFunc, utxo.ReadLockLedger(false))
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading addresses failed, error: %s", err)
	}

	addresses := make([]*address, 0, len(addressMap))
	for _, addr := range addressMap {
		addresses = append(addresses, addr)
	}

	return &addressesResponse{
		Addresses: addresses,
	}, nil
}

func addressesEd25519(_ echo.Context) (*addressesResponse, error) {

	addressMap := map[string]*address{}

	outputConsumerFunc := func(output *utxo.Output) bool {

		if addr, exists := addressMap[output.Address().String()]; exists {
			// add balance to total balance
			addr.Balance += output.Amount()
			return true
		}

		addressMap[output.Address().String()] = &address{
			AddressType: output.Address().Type(),
			Address:     output.Address().String(),
			Balance:     output.Amount(),
		}

		return true
	}

	err := deps.UTXOManager.ForEachUnspentOutput(outputConsumerFunc, utxo.ReadLockLedger(false))
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "reading addresses failed, error: %s", err)
	}

	addresses := make([]*address, 0, len(addressMap))
	for _, addr := range addressMap {
		addresses = append(addresses, addr)
	}

	return &addressesResponse{
		Addresses: addresses,
	}, nil
}

func milestoneDiff(c echo.Context) (*milestoneDiffResponse, error) {

	msIndex, err := v1.ParseMilestoneIndexParam(c)
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

	outputs := make([]*v1.OutputResponse, len(diff.Outputs))
	spents := make([]*v1.OutputResponse, len(diff.Spents))

	for i, output := range diff.Outputs {
		o, err := v1.NewOutputResponse(output, false, diff.Index)
		if err != nil {
			return nil, err
		}
		outputs[i] = o
	}

	for i, spent := range diff.Spents {
		o, err := v1.NewOutputResponse(spent.Output(), true, diff.Index)
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
	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message ID: %s, error: %s", messageIDHex, err)
	}

	cachedStartMsgMeta := deps.Storage.CachedMessageMetadataOrNil(messageID) // meta +1
	if cachedStartMsgMeta == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "message not found: %s", messageIDHex)
	}
	defer cachedStartMsgMeta.Release(true)

	if !cachedStartMsgMeta.Metadata().IsSolid() {
		return nil, errors.WithMessagef(echo.ErrServiceUnavailable, "start message is not solid: %s", messageIDHex)
	}

	startMsgReferened, startMsgReferenedAt := cachedStartMsgMeta.Metadata().ReferencedWithIndex()

	entryPointIndex := deps.Storage.SnapshotInfo().EntryPointIndex
	entryPoints := []*entryPoint{}
	tanglePath := []*messageWithParents{}

	if err := dag.TraverseParentsOfMessage(deps.Storage, messageID,
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
		func(messageID hornet.MessageID) {
			entryPoints = append(entryPoints, &entryPoint{MessageID: messageID.ToHex(), ReferencedByMilestone: entryPointIndex})
		},
		false, nil); err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "traverse parents failed, error: %s", err)
	}

	if len(entryPoints) == 0 {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "no referenced parents found: %s", messageIDHex)
	}

	return &messageConeResponse{
		ConeElementsCount: len(tanglePath),
		EntryPointsCount:  len(entryPoints),
		Cone:              tanglePath,
		EntryPoints:       entryPoints,
	}, nil
}
