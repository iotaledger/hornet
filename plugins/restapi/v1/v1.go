package v1

import (
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/plugins/restapi/common"
	"github.com/gohornet/hornet/plugins/spammer"
	"github.com/gohornet/hornet/plugins/urts"
)

const (
	// ToDo: add checks if node is synced
	waitForNodeSyncedTimeout = 2000 * time.Millisecond
)

const (
	// ParameterMessageID is used to identify a message by it's ID.
	ParameterMessageID = "messageID"

	// ParameterOutputID is used to identify an output by it's ID.
	ParameterOutputID = "outputID"

	// ParameterAddress is used to identify an address.
	ParameterAddress = "address"

	// ParameterMilestoneIndex is used to identify a milestone.
	ParameterMilestoneIndex = "milestoneIndex"

	// ParameterPeerID is used to identify a peer.
	ParameterPeerID = "peerID"
)

const (
	QueryParamControlCmdPruneDatabase      = "prunedatabase"      // targetIndex || depth
	QueryParamControlCmdCreateSnapshotFile = "createsnapshotfile" // targetIndex
	QueryParamControlCmdTiggerSolidifier   = "triggersolidifier"
)

const (
	// RouteInfo is the route for getting the node info.
	// GET returns the node info.
	RouteInfo = "/info"

	// RouteTips is the route for getting two tips.
	// GET returns the tips.
	RouteTips = "/tips"

	// RouteMessageData is the route for getting message data by it's messageID.
	// GET returns message data (json).
	RouteMessageData = "/messages/:" + ParameterMessageID

	// RouteMessageMetadata is the route for getting message metadata by it's messageID.
	// GET returns message metadata (including info about "promotion/reattachment needed").
	RouteMessageMetadata = "/messages/:" + ParameterMessageID + "/metadata"

	// RouteMessageBytes is the route for getting message raw data by it's messageID.
	// GET returns raw message data (bytes).
	RouteMessageBytes = "/messages/:" + ParameterMessageID + "/raw"

	// RouteMessageChildren is the route for getting message IDs of the children of a message, identified by it's messageID.
	// GET returns the message IDs of all children.
	RouteMessageChildren = "/messages/:" + ParameterMessageID + "/children"

	// RouteMessages is the route for getting message IDs or creating new messages.
	// GET with query parameter (mandatory) returns all message IDs that fit these filter criteria (query parameters: "index").
	// POST creates a single new message and returns the new message ID.
	RouteMessages = "/messages"

	// RouteMilestone is the route for getting a milestone by it's milestoneIndex.
	// GET returns the milestone.
	RouteMilestone = "/milestones/:" + ParameterMilestoneIndex

	// RouteOutput is the route for getting outputs by their outputID (transactionHash + outputIndex).
	// GET returns the output.
	RouteOutput = "/outputs/:" + ParameterOutputID

	// RouteAddressBalance is the route for getting the total balance of all unspent outputs of an address.
	// GET returns the balance of all unspent outputs of this address.
	RouteAddressBalance = "/addresses/:" + ParameterAddress

	// RouteAddressOutputs is the route for getting all output IDs for an address.
	// GET returns the outputIDs for all outputs of this address (optional query parameters: "include-spent").
	RouteAddressOutputs = "/addresses/:" + ParameterAddress + "/outputs"

	// RoutePeer is the route for getting peers by their peerID.
	// GET returns the peer
	// DELETE deletes the peer.
	RoutePeer = "/peers/:" + ParameterPeerID

	// RoutePeers is the route for getting all peers of the node.
	// GET returns a list of all peers.
	// POST adds a new peer.
	RoutePeers = "/peers"

	// RouteDebugOutputs is the debug route for getting all output IDs.
	// GET returns the outputIDs for all outputs.
	RouteDebugOutputs = "/debug/outputs"

	// RouteDebugOutputsUnspent is the debug route for getting all unspent output IDs.
	// GET returns the outputIDs for all unspent outputs.
	RouteDebugOutputsUnspent = "/debug/outputs/unspent"

	// RouteDebugOutputsSpent is the debug route for getting all spent output IDs.
	// GET returns the outputIDs for all spent outputs.
	RouteDebugOutputsSpent = "/debug/outputs/spent"

	// RouteDebugMilestoneDiffs is the debug route for getting a milestone diff by it's milestoneIndex.
	// GET returns the utxo diff (new outputs & spents) for the milestone index.
	RouteDebugMilestoneDiffs = "/debug/ms-diff/:" + ParameterMilestoneIndex

	// RouteDebugRequests is the debug route for getting all pending requests.
	// GET returns a list of all pending requests.
	RouteDebugRequests = "/debug/requests"

	// RouteDebugMessageCone is the debug route for traversing a cone of a message.
	// it traverses the parents of a message until they reference an older milestone than the start message.
	// GET returns the path of this traversal and the "entry points".
	RouteDebugMessageCone = "/debug/message-cones/:" + ParameterMessageID
)

var (
	features = []string{} // Workaround until https://github.com/golang/go/issues/27589 is fixed

	// ErrNodeNotSync is returned when the node was not synced.
	ErrNodeNotSync = errors.New("node not synced")
)

// jsonResponse wraps the result into a "data" field and sends the JSON response with status code.
func jsonResponse(c echo.Context, statusCode int, result interface{}) error {
	return c.JSON(statusCode, &common.HTTPOkResponseEnvelope{Data: result})
}

func SetupApiRoutesV1(routeGroup *echo.Group) {

	// Check for features
	if config.NodeConfig.GetBool(config.CfgNodeEnableProofOfWork) {
		features = append(features, "PoW")
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
		return jsonResponse(c, http.StatusOK, infoResp)
	})

	// only handle tips api calls if the URTS plugin is enabled
	if !node.IsSkipped(urts.PLUGIN) {
		routeGroup.GET(RouteTips, func(c echo.Context) error {

			tipsResp, err := tips(c)
			if err != nil {
				return err
			}
			return jsonResponse(c, http.StatusOK, tipsResp)
		})
	}

	routeGroup.GET(RouteMessageMetadata, func(c echo.Context) error {

		messageMetaResp, err := messageMetadataByID(c)
		if err != nil {
			return err
		}
		return jsonResponse(c, http.StatusOK, messageMetaResp)
	})

	routeGroup.GET(RouteMessageData, func(c echo.Context) error {

		messageResp, err := messageByID(c)
		if err != nil {
			return err
		}
		return jsonResponse(c, http.StatusOK, messageResp)
	})

	routeGroup.GET(RouteMessageBytes, func(c echo.Context) error {

		messageBytes, err := messageBytesByID(c)
		if err != nil {
			return err
		}

		return c.Blob(http.StatusOK, echo.MIMEOctetStream, messageBytes)
	})

	routeGroup.GET(RouteMessageChildren, func(c echo.Context) error {

		childrenResp, err := childrenIDsByID(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, childrenResp)
	})

	routeGroup.GET(RouteMessages, func(c echo.Context) error {

		messageIDsResp, err := messageIDsByIndex(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, messageIDsResp)
	})

	routeGroup.POST(RouteMessages, func(c echo.Context) error {

		messageMetaResp, err := sendMessage(c)
		if err != nil {
			return err
		}
		c.Response().Header().Set(echo.HeaderLocation, messageMetaResp.MessageID)
		return jsonResponse(c, http.StatusCreated, messageMetaResp)
	})

	routeGroup.GET(RouteMilestone, func(c echo.Context) error {

		milestoneResp, err := milestoneByIndex(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, milestoneResp)
	})

	routeGroup.GET(RouteOutput, func(c echo.Context) error {

		outputResp, err := outputByID(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, outputResp)
	})

	routeGroup.GET(RouteAddressBalance, func(c echo.Context) error {

		addressBalanceResp, err := balanceByAddress(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, addressBalanceResp)
	})

	routeGroup.GET(RouteAddressOutputs, func(c echo.Context) error {

		addressOutputsResp, err := outputsIDsByAddress(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, addressOutputsResp)
	})

	routeGroup.GET(RoutePeer, func(c echo.Context) error {

		peerResp, err := getPeer(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, peerResp)
	})

	routeGroup.DELETE(RoutePeer, func(c echo.Context) error {

		err := removePeer(c)
		if err != nil {
			return err
		}

		return c.NoContent(http.StatusOK)
	})

	routeGroup.GET(RoutePeers, func(c echo.Context) error {

		peersResp, err := listPeers(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, peersResp)
	})

	routeGroup.POST(RoutePeers, func(c echo.Context) error {

		peerResp, err := addPeer(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, peerResp)
	})

	routeGroup.GET(RouteDebugOutputs, func(c echo.Context) error {

		outputIdsResp, err := debugOutputsIDs(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, outputIdsResp)
	})

	routeGroup.GET(RouteDebugOutputsUnspent, func(c echo.Context) error {

		outputIdsResp, err := debugUnspentOutputsIDs(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, outputIdsResp)
	})

	routeGroup.GET(RouteDebugOutputsSpent, func(c echo.Context) error {

		outputIdsResp, err := debugSpentOutputsIDs(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, outputIdsResp)
	})

	routeGroup.GET(RouteDebugMilestoneDiffs, func(c echo.Context) error {

		milestoneDiffResp, err := debugMilestoneDiff(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, milestoneDiffResp)
	})

	routeGroup.GET(RouteDebugRequests, func(c echo.Context) error {

		requestsResp, err := debugRequests(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, requestsResp)
	})

	routeGroup.GET(RouteDebugMessageCone, func(c echo.Context) error {

		messsageConeResp, err := debugMessageCone(c)
		if err != nil {
			return err
		}

		return jsonResponse(c, http.StatusOK, messsageConeResp)
	})
}
