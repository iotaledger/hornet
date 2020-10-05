package v1

import (
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/cli"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/restapi/common"
	"github.com/gohornet/hornet/plugins/spammer"
	tangleplugin "github.com/gohornet/hornet/plugins/tangle"
	"github.com/gohornet/hornet/plugins/urts"
)

const (
	waitForNodeSyncedTimeout = 2000 * time.Millisecond
)

var (
	// ParameterMessageID is used to identify a message by it's ID.
	ParameterMessageID = "messageID"

	// ParameterOutputID is used to identify an output by it's ID.
	ParameterOutputID = "outputID"

	// ParameterAddress is used to identify an address.
	ParameterAddress = "address"

	// ParameterMilestoneIndex is used to identify a milestone.
	ParameterMilestoneIndex = "milestoneIndex"
)

var (
	// RouteInfo is the route for getting the node info.
	// GET returns the node info.
	RouteInfo = "/info"

	// RouteTips is the route for getting two tips.
	// GET returns the tips.
	RouteTips = "/tips"

	// RouteMessage is the route for getting message metadata by it's messageID.
	// GET returns message metadata (including info about "promotion/reattachment needed").
	RouteMessage = "/message/:" + ParameterMessageID

	// RouteMessageData is the route for getting message raw data by it's messageID.
	// GET returns raw message data (json).
	RouteMessageData = "/message/:" + ParameterMessageID + "/data"

	// RouteMessageBytes is the route for getting message raw data by it's messageID.
	// GET returns raw message data (bytes).
	RouteMessageBytes = "/message/:" + ParameterMessageID + "/bytes"

	// RouteMessageChildren is the route for getting message IDs of the children of a message, identified by it's messageID.
	// GET returns the message IDs of all children.
	RouteMessageChildren = "/message/:" + ParameterMessageID + "/children"

	// RouteMessages is the route for getting message IDs or creating new messages.
	// GET with query parameter (mandatory) returns all message IDs that fit these filter criteria (query parameters: "index").
	// POST creates a single new message and returns the new message ID.
	RouteMessages = "/messages"

	// RouteMilestone is the route for getting a milestone by it's milestoneIndex.
	// GET returns the milestone.
	RouteMilestone = "/milestone/:" + ParameterMilestoneIndex

	// RouteOutput is the route for getting outputs by their outputID (transactionHash + outputIndex).
	// GET returns the output.
	RouteOutput = "/output/:" + ParameterOutputID

	// RouteAddressOutputs is the route for getting all output IDs for an address.
	// GET returns the outputIDs for all outputs of this address (optional query parameters: "only-unspent").
	RouteAddressOutputs = "/address/:" + ParameterAddress + "/outputs"

	// RouteAddressBalance is the route for getting the total balance of all unspent outputs of an address.
	// GET returns the balance of all unspent outputs of this address.
	RouteAddressBalance = "/address/:" + ParameterAddress + "/balance"
)



var (
	features = []string{} // Workaround until https://github.com/golang/go/issues/27589 is fixed

	// ErrNodeNotSync is returned when the node was not synced.
	ErrNodeNotSync = errors.New("node not synced")
)

func SetupApiRoutesV1(routeGroup *echo.Group) {

	if !config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
		// Check for features
		if config.NodeConfig.GetBool(config.CfgNodeEnableProofOfWork) {
			features = append(features, "PoW")
		}
	}

	// only handle spammer api calls if the spammer plugin is enabled
	if !node.IsSkipped(spammer.PLUGIN) {
		//setupSpammerRoute(routeGroup)
	}

	routeGroup.GET(RouteInfo, func(c echo.Context) error {
		infoResp, err := info()
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, infoResp)
	})

	// only handle tips api calls if the URTS plugin is enabled
	if !node.IsSkipped(urts.PLUGIN) {
		routeGroup.GET(RouteTips, func(c echo.Context) error {
			tipsResp, err := tips(c)
			if err != nil {
				return err
			}
			return c.JSON(http.StatusOK, tipsResp)
		})
	}

	routeGroup.GET(RouteMessage, func(c echo.Context) error {
		// ToDo: Bech32
		messageMetaResp, err := messageMetaByID(c)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, messageMetaResp)
	})

	routeGroup.GET(RouteMessageData, func(c echo.Context) error {
		// ToDo: Bech32
		messageResp, err := messageDataByID(c)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, messageResp)
	})

	routeGroup.GET(RouteMessageBytes, func(c echo.Context) error {
		// ToDo: Bech32
		messageBytes, err := messageBytesByID(c)
		if err != nil {
			return err
		}

		return c.Blob(http.StatusOK, echo.MIMEOctetStream, messageBytes)
	})

	routeGroup.GET(RouteMessageChildren, func(c echo.Context) error {
		// ToDo: Bech32
		childrenResp, err := childrenByID(c)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, childrenResp)
	})

	routeGroup.GET(RouteMessages, func(c echo.Context) error {

		messageIDsResp, err := messageIDsByIndex(c)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, messageIDsResp)
	})

	routeGroup.POST(RouteMessages, func(c echo.Context) error {

		messageMetaResp, err := sendMessage(c)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusCreated, messageMetaResp)
	})

	routeGroup.GET(RouteMilestone, func(c echo.Context) error {

		milestoneResp, err := milestoneByIndex(c)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, milestoneResp)
	})

	routeGroup.GET(RouteOutput, func(c echo.Context) error {

		outputResp, err := outputByID(c)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, outputResp)
	})

	routeGroup.GET(RouteAddressOutputs, func(c echo.Context) error {

		addressOutputsResp, err := outputsByAddress(c)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, addressOutputsResp)
	})

	routeGroup.GET(RouteAddressBalance, func(c echo.Context) error {

		addressBalanceResp, err := balanceByAddress(c)
		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, addressBalanceResp)
	})

func info() (*infoResponse, error) {

	// latest milestone index
	latestMilestoneMessageID := hornet.GetNullMessageID().Hex()
	latestMilestoneIndex := tangle.GetLatestMilestoneIndex()

	// latest milestone message ID
	cachedLatestMilestoneMsg := tangle.GetMilestoneCachedMessageOrNil(latestMilestoneIndex)
	if cachedLatestMilestoneMsg != nil {
		latestMilestoneMessageID = cachedLatestMilestoneMsg.GetMessage().GetMessageID().Hex()
		cachedLatestMilestoneMsg.Release(true)
	}

	// solid milestone index
	solidMilestoneMessageID := hornet.GetNullMessageID().Hex()
	solidMilestoneIndex := tangle.GetSolidMilestoneIndex()

	// solid milestone message ID
	cachedSolidMilestoneMsg := tangle.GetMilestoneCachedMessageOrNil(solidMilestoneIndex)
	if cachedSolidMilestoneMsg != nil {
		solidMilestoneMessageID = cachedSolidMilestoneMsg.GetMessage().GetMessageID().Hex()
		cachedSolidMilestoneMsg.Release(true)
	}

	// pruning index
	var pruningIndex milestone.Index
	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		pruningIndex = snapshotInfo.PruningIndex
	}

	return &infoResponse{
		Name:                          cli.AppName,
		Version:                       cli.AppVersion,
		IsHealthy:                     tangleplugin.IsNodeHealthy(),
		IsSynced:                      tangle.IsNodeSyncedWithThreshold(),
		CoordinatorPublicKey:          config.NodeConfig.GetString(config.CfgCoordinatorPublicKey),
		LatestMilestoneMessageID:      latestMilestoneMessageID,
		LatestMilestoneIndex:          uint64(latestMilestoneIndex),
		LatestSolidMilestoneMessageID: solidMilestoneMessageID,
		LatestSolidMilestoneIndex:     uint64(solidMilestoneIndex),
		PruningIndex:                  uint64(pruningIndex),
		Features:                      features,
	}, nil
}

func tips(c echo.Context) (*tipsResponse, error) {
	spammerTips := false
	for query := range c.QueryParams() {
		if strings.ToLower(query) == "spammertips" {
			spammerTips = true
			break
		}
	}

	var tips hornet.MessageIDs
	var err error

	if !spammerTips {
		tips, err = urts.TipSelector.SelectNonLazyTips()
	} else {
		_, tips, err = urts.TipSelector.SelectSpammerTips()
	}

	if err != nil {
		if err == tangle.ErrNodeNotSynced || err == tipselect.ErrNoTipsAvailable {
			return nil, errors.WithMessage(common.ErrServiceUnavailable, err.Error())
		}
		return nil, errors.WithMessage(common.ErrInternalError, err.Error())
	}

	return &tipsResponse{Tip1: tips[0].Hex(), Tip2: tips[1].Hex()}, nil
}

func messageMetaByID(c echo.Context) (*messageMetadataResponse, error) {
	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message ID: %s, error: %w", messageIDHex, err)
	}

	return messageMetaByMessageID(messageID)
}

func messageMetaByMessageID(messageID *hornet.MessageID) (*messageMetadataResponse, error) {
	cachedMsgMeta := tangle.GetCachedMessageMetadataOrNil(messageID)
	if cachedMsgMeta == nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "message not found: %s", messageID.Hex())
	}
	defer cachedMsgMeta.Release(true)

	metadata := cachedMsgMeta.GetMetadata()

	var referencedByMilestone *uint64 = nil
	referenced, referencedIndex := metadata.GetReferenced()
	if referenced {
		refIndex := uint64(referencedIndex)
		referencedByMilestone = &refIndex
	}

	messageMetadataResponse := &messageMetadataResponse{
		MessageID:             metadata.GetMessageID().Hex(),
		Parent1:               metadata.GetParent1MessageID().Hex(),
		Parent2:               metadata.GetParent2MessageID().Hex(),
		Solid:                 metadata.IsSolid(),
		ReferencedByMilestone: referencedByMilestone,
	}

	if referenced {
		inclusionState := "noTransaction"

		if metadata.IsConflictingTx() {
			inclusionState = "conflicting"
		} else if metadata.IsIncludedTxInLedger() {
			inclusionState = "included"
		}

		messageMetadataResponse.LedgerInclusionState = &inclusionState
	} else if metadata.IsSolid() {
		// determine info about the quality of the tip if not referenced
		lsmi := tangle.GetSolidMilestoneIndex()
		ycri, ocri := dag.GetConeRootIndexes(cachedMsgMeta.Retain(), lsmi)

		// if none of the following checks is true, the tip is non-lazy, so there is no need to promote or reattach
		shouldPromote := false
		shouldReattach := false

		if (lsmi - ocri) > milestone.Index(config.NodeConfig.GetInt(config.CfgTipSelBelowMaxDepth)) {
			// if the OCRI to LSMI delta is over BelowMaxDepth/below-max-depth, then the tip is lazy and should be reattached
			shouldPromote = false
			shouldReattach = true
		} else if (lsmi - ycri) > milestone.Index(config.NodeConfig.GetInt(config.CfgTipSelMaxDeltaMsgYoungestConeRootIndexToLSMI)) {
			// if the LSMI to YCRI delta is over CfgTipSelMaxDeltaMsgYoungestConeRootIndexToLSMI, then the tip is lazy and should be promoted
			shouldPromote = true
			shouldReattach = false
		} else if (lsmi - ocri) > milestone.Index(config.NodeConfig.GetInt(config.CfgTipSelMaxDeltaMsgOldestConeRootIndexToLSMI)) {
			// if the OCRI to LSMI delta is over CfgTipSelMaxDeltaMsgOldestConeRootIndexToLSMI, the tip is semi-lazy and should be promoted
			shouldPromote = true
			shouldReattach = false
		}

		messageMetadataResponse.ShouldPromote = &shouldPromote
		messageMetadataResponse.ShouldReattach = &shouldReattach
	}

	return messageMetadataResponse, nil
}

func messageDataByID(c echo.Context) (*iotago.Message, error) {
	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message ID: %s, error: %w", messageIDHex, err)
	}

	// ToDo: Load data without de-/serialization
	cachedMsg := tangle.GetCachedMessageOrNil(messageID)
	if cachedMsg == nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "message not found: %s", messageIDHex)
	}
	defer cachedMsg.Release(true)

	return cachedMsg.GetMessage().GetMessage(), nil
}

func messageBytesByID(c echo.Context) ([]byte, error) {
	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message ID: %s, error: %w", messageIDHex, err)
	}

	// ToDo: Load data without de-/serialization
	cachedMsg := tangle.GetCachedMessageOrNil(messageID)
	if cachedMsg == nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "message not found: %s", messageIDHex)
	}
	defer cachedMsg.Release(true)

	data, err := cachedMsg.GetMessage().GetMessage().Serialize(iotago.DeSeriModeNoValidation)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInternalError, "message serialization failed: %s, error: %w", messageIDHex, err)
	}

	return data, nil
}

func childrenByID(c echo.Context) (*childrenResponse, error) {
	messageIDHex := strings.ToLower(c.Param(ParameterMessageID))

	messageID, err := hornet.MessageIDFromHex(messageIDHex)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message ID: %s, error: %w", messageIDHex, err)
	}

	maxResults := config.NodeConfig.GetInt(config.CfgRestAPILimitsMaxResults)

	var childrenMessageIDsHex []string
	for _, childrenMessageID := range tangle.GetChildrenMessageIDs(messageID, maxResults) {
		childrenMessageIDsHex = append(childrenMessageIDsHex, childrenMessageID.Hex())
	}

	return &childrenResponse{
		MessageID:  messageID.Hex(),
		MaxResults: uint32(maxResults),
		Count:      uint32(len(childrenMessageIDsHex)),
		Children:   childrenMessageIDsHex,
	}, nil
}

func messageIDsByIndex(c echo.Context) (*messageIDsResponse, error) {
	index := c.QueryParam("index")

	if index == "" {
		return nil, errors.WithMessage(common.ErrInvalidParameter, "query parameter index empty")
	}

	maxResults := config.NodeConfig.GetInt(config.CfgRestAPILimitsMaxResults)

	var messageIDsHex []string
	for _, messageID := range tangle.GetIndexMessageIDs(index, maxResults) {
		messageIDsHex = append(messageIDsHex, messageID.Hex())
	}

	return &messageIDsResponse{
		Index:      index,
		MaxResults: uint32(maxResults),
		Count:      uint32(len(messageIDsHex)),
		MessageIDs: messageIDsHex,
	}, nil
}

func sendMessage(c echo.Context) (*messageMetadataResponse, error) {

	msg := &iotago.Message{}

	if err := c.Bind(msg); err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message, error: %w", err)
	}

	message, err := tangle.NewMessage(msg)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message, error: %w", err)
	}

	messageIDLock := syncutils.Mutex{}

	// search the ID of the sent message
	messageID := message.GetMessageID()

	// wgMessageProcessed waits until the message got processed
	wgMessageProcessed := sync.WaitGroup{}
	wgMessageProcessed.Add(1)

	onMessageProcessed := events.NewClosure(func(msgID *hornet.MessageID) {
		messageIDLock.Lock()
		defer messageIDLock.Unlock()

		if messageID == nil {
			return
		}

		if *messageID != *msgID {
			return
		}

		// message was processed
		wgMessageProcessed.Done()

		// we have to set the messageID to nil, because the event may be fired several times
		messageID = nil
	})

	tangleplugin.Events.ProcessedMessage.Attach(onMessageProcessed)
	defer tangleplugin.Events.ProcessedMessage.Detach(onMessageProcessed)

	if err := gossip.Processor().SerializeAndEmit(message, iotago.DeSeriModePerformValidation); err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid message, error: %w", err)
	}

	// wait until the message got processed
	wgMessageProcessed.Wait()

	return messageMetaByMessageID(message.GetMessageID())
}

func milestoneByIndex(c echo.Context) (*milestoneResponse, error) {
	milestoneIndex := strings.ToLower(c.Param(ParameterMilestoneIndex))

	msIndex, err := strconv.ParseUint(milestoneIndex, 10, 64)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid milestone index: %s, error: %w", milestoneIndex, err)
	}

	cachedMilestone := tangle.GetCachedMilestoneOrNil(milestone.Index(msIndex)) // milestone +1
	if cachedMilestone == nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "milestone not found: %d", msIndex)
	}
	defer cachedMilestone.Release(true)

	return &milestoneResponse{
		Index:     uint32(cachedMilestone.GetMilestone().Index),
		MessageID: cachedMilestone.GetMilestone().MessageID.Hex(),
		Time:      cachedMilestone.GetMilestone().Timestamp.Format(time.RFC3339),
	}, nil

}

func outputByID(c echo.Context) (*outputResponse, error) {
	outputIDParam := strings.ToLower(c.Param(ParameterOutputID))

	outputIDBytes, err := hex.DecodeString(outputIDParam)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid output ID: %s, error: %w", outputIDParam, err)
	}

	if len(outputIDBytes) != (iotago.TransactionIDLength + iotago.UInt16ByteSize) {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid output ID: %s, error: %w", outputIDParam, err)
	}

	var outputID iotago.UTXOInputID
	copy(outputID[:], outputIDBytes)

	output, err := utxo.ReadOutputByOutputID(&outputID)
	if err != nil {
		if err == kvstore.ErrKeyNotFound {
			return nil, errors.WithMessagef(common.ErrInvalidParameter, "output not found: %s", outputIDParam)
		}

		return nil, errors.WithMessagef(common.ErrInternalError, "reading output failed: %s, error: %w", outputIDParam, err)
	}

	return &outputResponse{
		OutputID:   hex.EncodeToString(output.OutputID()[:]),
		MessageID:  output.MessageID().Hex(),
		OutputType: byte(output.OutputType()),
		Address:    output.Address().String(),
		Amount:     output.Amount(),
	}, nil
}

func outputsByAddress(c echo.Context) (*addressOutputsResponse, error) {
	addressParam := strings.ToLower(c.Param(ParameterAddress))

	addressBytes, err := hex.DecodeString(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid address: %s, error: %w", addressParam, err)
	}

	if len(addressBytes) != (iotago.Ed25519AddressBytesLength) {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid address length: %s", addressParam)
	}

	var address iotago.Ed25519Address
	copy(address[:], addressBytes)

	maxResults := config.NodeConfig.GetInt(config.CfgRestAPILimitsMaxResults)

	unspentOutputs, err := utxo.UnspentOutputsForAddress(&address, maxResults)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInternalError, "reading unspent outputs failed: %s, error: %w", address, err)
	}

	var outputIDs []string
	for _, unspentOutput := range unspentOutputs {
		outputIDs = append(outputIDs, hex.EncodeToString(unspentOutput.OutputID()[:]))
	}

	includeSpent := false
	for query := range c.QueryParams() {
		if strings.ToLower(query) == "include-spent" {
			includeSpent = true
			break
		}
	}

	if includeSpent && maxResults-len(outputIDs) > 0 {

		spents, err := utxo.SpentOutputsForAddress(&address, maxResults-len(outputIDs))
		if err != nil {
			return nil, errors.WithMessagef(common.ErrInternalError, "reading spent outputs failed: %s, error: %w", address, err)
		}

		for _, spent := range spents {
			outputIDs = append(outputIDs, hex.EncodeToString(spent.OutputID()[:]))
		}
	}

	return &addressOutputsResponse{
		Address:    addressParam,
		MaxResults: uint32(maxResults),
		Count:      uint32(len(outputIDs)),
		OutputIDs:  outputIDs,
	}, nil
}

func balanceByAddress(c echo.Context) (*addressBalanceResponse, error) {
	addressParam := strings.ToLower(c.Param(ParameterAddress))

	addressBytes, err := hex.DecodeString(addressParam)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid address: %s, error: %w", addressParam, err)
	}

	if len(addressBytes) != (iotago.Ed25519AddressBytesLength) {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid address length: %s", addressParam)
	}

	var address iotago.Ed25519Address
	copy(address[:], addressBytes)

	maxResults := config.NodeConfig.GetInt(config.CfgRestAPILimitsMaxResults)

	balance, count, err := utxo.AddressBalance(&address, maxResults)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInternalError, "reading address balance failed: %s, error: %w", address, err)
	}

	return &addressBalanceResponse{
		Address:    addressParam,
		MaxResults: uint32(maxResults),
		Count:      uint32(count),
		Balance:    balance,
	}, nil
}
