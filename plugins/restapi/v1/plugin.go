package v1

import (
	"net/http"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/hive.go/configuration"

	powcore "github.com/gohornet/hornet/core/pow"
	"github.com/gohornet/hornet/pkg/app"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/urts"
)

const (
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

	// RouteAddressBech32Balance is the route for getting the total balance of all unspent outputs of an address.
	// The address must be encoded in bech32.
	// GET returns the balance of all unspent outputs of this address.
	RouteAddressBech32Balance = "/addresses/:" + ParameterAddress

	// RouteAddressEd25519Balance is the route for getting the total balance of all unspent outputs of an ed25519 address.
	// The ed25519 address must be encoded in hex.
	// GET returns the balance of all unspent outputs of this address.
	RouteAddressEd25519Balance = "/addresses/ed25519/:" + ParameterAddress

	// RouteAddressBech32Outputs is the route for getting all output IDs for an address.
	// The address must be encoded in bech32.
	// GET returns the outputIDs for all outputs of this address (optional query parameters: "include-spent").
	RouteAddressBech32Outputs = "/addresses/:" + ParameterAddress + "/outputs"

	// RouteAddressEd25519Outputs is the route for getting all output IDs for an ed25519 address.
	// The ed25519 address must be encoded in hex.
	// GET returns the outputIDs for all outputs of this address (optional query parameters: "include-spent").
	RouteAddressEd25519Outputs = "/addresses/ed25519/:" + ParameterAddress + "/outputs"

	// RoutePeer is the route for getting peers by their peerID.
	// GET returns the peer
	// DELETE deletes the peer.
	RoutePeer = "/peers/:" + ParameterPeerID

	// RoutePeers is the route for getting all peers of the node.
	// GET returns a list of all peers.
	// POST adds a new peer.
	RoutePeers = "/peers"

	// RouteControlDatabasePrune is the control route to manually prune the database.
	// GET prunes the database. (query parameters: "index" || "depth")
	RouteControlDatabasePrune = "/control/database/prune"

	// RouteControlSnapshotCreate is the control route to manually create a snapshot file.
	// GET creates a snapshot. (query parameters: "index")
	RouteControlSnapshotCreate = "/control/snapshots/create"

	// RouteDebugSolidifer is the debug route to manually trigger the solidifier.
	// GET triggers the solidifier.
	RouteDebugSolidifer = "/debug/solidifer"

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

func init() {
	Plugin = &node.Plugin{
		Status: node.Enabled,
		Pluggable: node.Pluggable{
			Name:      "RestAPIV1",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Configure: configure,
		},
	}
}

var (
	Plugin             *node.Plugin
	proofOfWorkEnabled bool
	features           []string

	// ErrNodeNotSync is returned when the node was not synced.
	ErrNodeNotSync = errors.New("node not synced")

	deps dependencies
)

type dependencies struct {
	dig.In
	Storage          *storage.Storage
	Tangle           *tangle.Tangle
	Manager          *p2p.Manager
	RequestQueue     gossip.RequestQueue
	UTXO             *utxo.Manager
	PoWHandler       *pow.Handler
	MessageProcessor *gossip.MessageProcessor
	Snapshot         *snapshot.Snapshot
	AppInfo          *app.AppInfo
	NodeConfig       *configuration.Configuration `name:"nodeConfig"`
	NetworkID        uint64                       `name:"networkId"`
	TipSelector      *tipselect.TipSelector
	Echo             *echo.Echo
}

func configure() {
	routeGroup := deps.Echo.Group("/api/v1")

	proofOfWorkEnabled = deps.NodeConfig.Bool(powcore.CfgNodeEnableProofOfWork)

	// Check for features
	features = []string{}
	if proofOfWorkEnabled {
		features = append(features, "PoW")
	}

	routeGroup.GET(RouteInfo, func(c echo.Context) error {
		resp, err := info()
		if err != nil {
			return err
		}
		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	// only handle tips api calls if the URTS plugin is enabled
	if !Plugin.Node.IsSkipped(urts.Plugin) {
		routeGroup.GET(RouteTips, func(c echo.Context) error {
			resp, err := tips(c)
			if err != nil {
				return err
			}
			return restapi.JSONResponse(c, http.StatusOK, resp)
		})
	}

	routeGroup.GET(RouteMessageMetadata, func(c echo.Context) error {
		resp, err := messageMetadataByID(c)
		if err != nil {
			return err
		}
		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteMessageData, func(c echo.Context) error {
		resp, err := messageByID(c)
		if err != nil {
			return err
		}
		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteMessageBytes, func(c echo.Context) error {
		resp, err := messageBytesByID(c)
		if err != nil {
			return err
		}

		return c.Blob(http.StatusOK, echo.MIMEOctetStream, resp)
	})

	routeGroup.GET(RouteMessageChildren, func(c echo.Context) error {
		resp, err := childrenIDsByID(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteMessages, func(c echo.Context) error {
		resp, err := messageIDsByIndex(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.POST(RouteMessages, func(c echo.Context) error {
		resp, err := sendMessage(c)
		if err != nil {
			return err
		}
		c.Response().Header().Set(echo.HeaderLocation, resp.MessageID)
		return restapi.JSONResponse(c, http.StatusCreated, resp)
	})

	routeGroup.GET(RouteMilestone, func(c echo.Context) error {
		resp, err := milestoneByIndex(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteOutput, func(c echo.Context) error {
		resp, err := outputByID(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAddressBech32Balance, func(c echo.Context) error {
		resp, err := balanceByBech32Address(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAddressEd25519Balance, func(c echo.Context) error {
		resp, err := balanceByEd25519Address(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAddressBech32Outputs, func(c echo.Context) error {
		resp, err := outputsIDsByBech32Address(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAddressEd25519Outputs, func(c echo.Context) error {
		resp, err := outputsIDsByEd25519Address(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RoutePeer, func(c echo.Context) error {
		resp, err := getPeer(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.DELETE(RoutePeer, func(c echo.Context) error {
		if err := removePeer(c); err != nil {
			return err
		}

		return c.NoContent(http.StatusOK)
	})

	routeGroup.GET(RoutePeers, func(c echo.Context) error {
		resp, err := listPeers(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.POST(RoutePeers, func(c echo.Context) error {
		resp, err := addPeer(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteControlDatabasePrune, func(c echo.Context) error {
		resp, err := pruneDatabase(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteControlSnapshotCreate, func(c echo.Context) error {
		resp, err := createSnapshot(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteDebugSolidifer, func(c echo.Context) error {
		deps.Tangle.TriggerSolidifier()

		return restapi.JSONResponse(c, http.StatusOK, "solidifier triggered")
	})

	routeGroup.GET(RouteDebugOutputs, func(c echo.Context) error {
		resp, err := debugOutputsIDs(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteDebugOutputsUnspent, func(c echo.Context) error {
		resp, err := debugUnspentOutputsIDs(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteDebugOutputsSpent, func(c echo.Context) error {
		resp, err := debugSpentOutputsIDs(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteDebugMilestoneDiffs, func(c echo.Context) error {
		resp, err := debugMilestoneDiff(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteDebugRequests, func(c echo.Context) error {
		resp, err := debugRequests(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteDebugMessageCone, func(c echo.Context) error {
		resp, err := debugMessageCone(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})
}
