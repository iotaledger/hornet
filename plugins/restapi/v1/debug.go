package v1

import (
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	iotago "github.com/iotaledger/iota.go"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/restapi"
)

func debugOutputsIDs(c echo.Context) (*outputIDsResponse, error) {

	outputIDs := []string{}
	outputConsumerFunc := func(output *utxo.Output) bool {
		outputIDs = append(outputIDs, hex.EncodeToString(output.OutputID()[:]))
		return true
	}

	err := deps.UTXO.ForEachOutput(outputConsumerFunc)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "reading unspent outputs failed, error: %s", err)
	}

	return &outputIDsResponse{
		OutputIDs: outputIDs,
	}, nil
}

func debugUnspentOutputsIDs(c echo.Context) (*outputIDsResponse, error) {

	outputIDs := []string{}
	outputConsumerFunc := func(output *utxo.Output) bool {
		outputIDs = append(outputIDs, hex.EncodeToString(output.OutputID()[:]))
		return true
	}

	err := deps.UTXO.ForEachUnspentOutput(outputConsumerFunc)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "reading unspent outputs failed, error: %s", err)
	}

	return &outputIDsResponse{
		OutputIDs: outputIDs,
	}, nil
}

func debugSpentOutputsIDs(c echo.Context) (*outputIDsResponse, error) {

	outputIDs := []string{}

	spentConsumerFunc := func(spent *utxo.Spent) bool {
		outputIDs = append(outputIDs, hex.EncodeToString(spent.OutputID()[:]))
		return true
	}

	err := deps.UTXO.ForEachSpentOutput(spentConsumerFunc)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "reading spent outputs failed, error: %s", err)
	}

	return &outputIDsResponse{
		OutputIDs: outputIDs,
	}, nil
}

func debugAddresses(c echo.Context) (*addressesResponse, error) {

	addressMap := map[string]*address{}

	outputConsumerFunc := func(output *utxo.Output) bool {
		// ToDo: check for different address types

		if addr, exists := addressMap[output.Address().String()]; exists {
			// add balance to total balance
			addr.Balance += output.Amount()
			return true
		}

		addressMap[output.Address().String()] = &address{
			AddressType: iotago.AddressEd25519,
			Address:     output.Address().String(),
			Balance:     output.Amount(),
		}

		return true
	}

	err := deps.UTXO.ForEachUnspentOutput(outputConsumerFunc)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "reading addresses failed, error: %s", err)
	}

	addresses := []*address{}
	for _, addr := range addressMap {
		addresses = append(addresses, addr)
	}

	return &addressesResponse{
		Addresses: addresses,
	}, nil
}

func debugAddressesEd25519(c echo.Context) (*addressesResponse, error) {

	addressMap := map[string]*address{}

	outputConsumerFunc := func(output *utxo.Output) bool {
		// ToDo: allow ed25519 address type only

		if addr, exists := addressMap[output.Address().String()]; exists {
			// add balance to total balance
			addr.Balance += output.Amount()
			return true
		}

		addressMap[output.Address().String()] = &address{
			AddressType: iotago.AddressEd25519,
			Address:     output.Address().String(),
			Balance:     output.Amount(),
		}

		return true
	}

	err := deps.UTXO.ForEachUnspentOutput(outputConsumerFunc)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "reading addresses failed, error: %s", err)
	}

	addresses := []*address{}
	for _, addr := range addressMap {
		addresses = append(addresses, addr)
	}

	return &addressesResponse{
		Addresses: addresses,
	}, nil
}

func debugMilestoneDiff(c echo.Context) (*milestoneDiffResponse, error) {
	milestoneIndex := strings.ToLower(c.Param(ParameterMilestoneIndex))

	msIndex, err := strconv.ParseUint(milestoneIndex, 10, 64)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid milestone index: %s, error: %s", milestoneIndex, err)
	}

	diffOutputs, diffSpents, err := deps.UTXO.GetMilestoneDiffs(milestone.Index(msIndex))

	outputs := []*outputResponse{}
	spents := []*outputResponse{}

	for _, output := range diffOutputs {
		o, err := newOutputResponse(output, false)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, o)
	}

	for _, spent := range diffSpents {
		o, err := newOutputResponse(spent.Output(), true)
		if err != nil {
			return nil, err
		}
		spents = append(spents, o)
	}

	return &milestoneDiffResponse{
		MilestoneIndex: milestone.Index(msIndex),
		Outputs:        outputs,
		Spents:         spents,
	}, nil
}

func debugRequests(c echo.Context) (*requestsResponse, error) {

	debugReqs := []*request{}

	queued, pending, processing := deps.RequestQueue.Requests()

	for _, req := range queued {
		debugReqs = append(debugReqs, &request{
			MessageID:        req.MessageID.Hex(),
			Type:             "queued",
			MessageExists:    deps.Storage.ContainsMessage(req.MessageID),
			EnqueueTimestamp: req.EnqueueTime.Format(time.RFC3339),
			MilestoneIndex:   req.MilestoneIndex,
		})
	}

	for _, req := range pending {
		debugReqs = append(debugReqs, &request{
			MessageID:        req.MessageID.Hex(),
			Type:             "pending",
			MessageExists:    deps.Storage.ContainsMessage(req.MessageID),
			EnqueueTimestamp: req.EnqueueTime.Format(time.RFC3339),
			MilestoneIndex:   req.MilestoneIndex,
		})
	}

	for _, req := range processing {
		debugReqs = append(debugReqs, &request{
			MessageID:        req.MessageID.Hex(),
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

func debugMessageCone(c echo.Context) (*messageConeResponse, error) {
	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message ID: %s, error: %s", messageIDHex, err)
	}

	cachedStartMsgMeta := deps.Storage.GetCachedMessageMetadataOrNil(messageID) // meta +1
	if cachedStartMsgMeta == nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "message not found: %s", messageIDHex)
	}
	defer cachedStartMsgMeta.Release(true)

	if !cachedStartMsgMeta.GetMetadata().IsSolid() {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "start message is not solid: %s", messageIDHex)
	}

	startMsgReferened, startMsgReferenedAt := cachedStartMsgMeta.GetMetadata().GetReferenced()

	entryPointIndex := deps.Storage.GetSnapshotInfo().EntryPointIndex
	entryPoints := []*entryPoint{}
	tanglePath := []*messageWithParents{}

	if err := dag.TraverseParents(deps.Storage, messageID,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			if referenced, at := cachedMsgMeta.GetMetadata().GetReferenced(); referenced {
				if !startMsgReferened || (at < startMsgReferenedAt) {
					entryPoints = append(entryPoints, &entryPoint{MessageID: cachedMsgMeta.GetMetadata().GetMessageID().Hex(), ReferencedByMilestone: at})
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
						MessageID: metadata.GetMessageID().Hex(),
						Parent1:   metadata.GetParent1MessageID().Hex(),
						Parent2:   metadata.GetParent2MessageID().Hex(),
					},
				)
			})

			return nil
		},
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		func(messageID *hornet.MessageID) {
			entryPoints = append(entryPoints, &entryPoint{MessageID: messageID.Hex(), ReferencedByMilestone: entryPointIndex})
		},
		false, nil); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "traverse parents failed, error: %s", err)
	}

	if len(entryPoints) == 0 {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "no referenced parents found: %s", messageIDHex)
	}

	return &messageConeResponse{
		ConeElementsCount: len(tanglePath),
		EntryPointsCount:  len(entryPoints),
		Cone:              tanglePath,
		EntryPoints:       entryPoints,
	}, nil
}
