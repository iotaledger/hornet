package v2

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/app"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	restapipkg "github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/restapi"
	"github.com/iotaledger/hive.go/configuration"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// RouteInfo is the route for getting the node info.
	// GET returns the node info.
	RouteInfo = "/info"

	// RouteTips is the route for getting tips.
	// GET returns the tips.
	RouteTips = "/tips"

	// RouteMessage is the route for getting a message by its messageID.
	// GET returns the message based on the given type in the request "Accept" header.
	// MIMEApplicationJSON => json
	// MIMEVendorIOTASerializer => bytes
	RouteMessage = "/messages/:" + restapipkg.ParameterMessageID

	// RouteMessageMetadata is the route for getting message metadata by its messageID.
	// GET returns message metadata (including info about "promotion/reattachment needed").
	RouteMessageMetadata = "/messages/:" + restapipkg.ParameterMessageID + "/metadata"

	// RouteMessageChildren is the route for getting message IDs of the children of a message, identified by its messageID.
	// GET returns the message IDs of all children.
	RouteMessageChildren = "/messages/:" + restapipkg.ParameterMessageID + "/children"

	// RouteMessages is the route for creating new messages.
	// POST creates a single new message and returns the new message ID.
	// The message is parsed based on the given type in the request "Content-Type" header.
	// MIMEApplicationJSON => json
	// MIMEVendorIOTASerializer => bytes
	RouteMessages = "/messages"

	// RouteTransactionsIncludedMessage is the route for getting the message that was included in the ledger for a given transaction ID.
	// GET returns the message based on the given type in the request "Accept" header.
	// MIMEApplicationJSON => json
	// MIMEVendorIOTASerializer => bytes
	RouteTransactionsIncludedMessage = "/transactions/:" + restapipkg.ParameterTransactionID + "/included-message"

	// RouteMilestone is the route for getting a milestone by its milestoneIndex.
	// GET returns the milestone.
	RouteMilestone = "/milestones/:" + restapipkg.ParameterMilestoneIndex

	// RouteMilestoneUTXOChanges is the route for getting all UTXO changes of a milestone by its milestoneIndex.
	// GET returns the output IDs of all UTXO changes.
	RouteMilestoneUTXOChanges = "/milestones/:" + restapipkg.ParameterMilestoneIndex + "/utxo-changes"

	// RouteOutput is the route for getting an output by its outputID (transactionHash + outputIndex).
	// GET returns the output based on the given type in the request "Accept" header.
	// MIMEApplicationJSON => json
	// MIMEVendorIOTASerializer => bytes
	RouteOutput = "/outputs/:" + restapipkg.ParameterOutputID

	// RouteOutputMetadata is the route for getting output metadata by its outputID (transactionHash + outputIndex) without getting the data again.
	// GET returns the output metadata.
	RouteOutputMetadata = "/outputs/:" + restapipkg.ParameterOutputID + "/metadata"

	// RouteTreasury is the route for getting the current treasury output.
	// GET returns the treasury.
	RouteTreasury = "/treasury"

	// RouteReceipts is the route for getting all persisted receipts on a node.
	// GET returns the receipts.
	RouteReceipts = "/receipts"

	// RouteReceiptsMigratedAtIndex is the route for getting all persisted receipts for a given migrated at index on a node.
	// GET returns the receipts for the given migrated at index.
	RouteReceiptsMigratedAtIndex = "/receipts/:" + restapipkg.ParameterMilestoneIndex

	// RouteComputeWhiteFlagMutations is the route to compute the white flag mutations for the cone of the given parents.
	// POST computes the white flag mutations.
	RouteComputeWhiteFlagMutations = "/whiteflag"

	// RoutePeer is the route for getting peers by their peerID.
	// GET returns the peer
	// DELETE deletes the peer.
	RoutePeer = "/peers/:" + restapipkg.ParameterPeerID

	// RoutePeers is the route for getting all peers of the node.
	// GET returns a list of all peers.
	// POST adds a new peer.
	RoutePeers = "/peers"

	// RouteControlDatabasePrune is the control route to manually prune the database.
	// POST prunes the database.
	RouteControlDatabasePrune = "/control/database/prune"

	// RouteControlSnapshotsCreate is the control route to manually create a snapshot files.
	// POST creates a snapshot (full, delta or both).
	RouteControlSnapshotsCreate = "/control/snapshots/create"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusEnabled,
		Pluggable: node.Pluggable{
			Name:      "RestAPIV2",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Configure: configure,
		},
	}
}

var (
	Plugin   *node.Plugin
	features = []string{}
	attacher *tangle.MessageAttacher

	// ErrNodeNotSync is returned when the node was not synced.
	ErrNodeNotSync = errors.New("node not synced")

	deps dependencies
)

type dependencies struct {
	dig.In
	Storage                               *storage.Storage
	SyncManager                           *syncmanager.SyncManager
	Tangle                                *tangle.Tangle
	TipScoreCalculator                    *tangle.TipScoreCalculator
	PeeringManager                        *p2p.Manager
	GossipService                         *gossip.Service
	UTXOManager                           *utxo.Manager
	PoWHandler                            *pow.Handler
	SnapshotManager                       *snapshot.SnapshotManager
	AppInfo                               *app.AppInfo
	NodeConfig                            *configuration.Configuration `name:"nodeConfig"`
	PeeringConfigManager                  *p2p.ConfigManager
	NetworkID                             uint64 `name:"networkId"`
	NetworkIDName                         string `name:"networkIdName"`
	DeserializationParameters             *iotago.DeSerializationParameters
	MaxDeltaMsgYoungestConeRootIndexToCMI int                        `name:"maxDeltaMsgYoungestConeRootIndexToCMI"`
	MaxDeltaMsgOldestConeRootIndexToCMI   int                        `name:"maxDeltaMsgOldestConeRootIndexToCMI"`
	BelowMaxDepth                         int                        `name:"belowMaxDepth"`
	WhiteFlagParentsSolidTimeout          time.Duration              `name:"whiteFlagParentsSolidTimeout"`
	MinPoWScore                           float64                    `name:"minPoWScore"`
	Bech32HRP                             iotago.NetworkPrefix       `name:"bech32HRP"`
	RestAPILimitsMaxResults               int                        `name:"restAPILimitsMaxResults"`
	SnapshotsFullPath                     string                     `name:"snapshotsFullPath"`
	SnapshotsDeltaPath                    string                     `name:"snapshotsDeltaPath"`
	TipSelector                           *tipselect.TipSelector     `optional:"true"`
	Echo                                  *echo.Echo                 `optional:"true"`
	RestPluginManager                     *restapi.RestPluginManager `optional:"true"`
}

func configure() {
	// check if RestAPI plugin is disabled
	if Plugin.Node.IsSkipped(restapi.Plugin) {
		Plugin.LogPanic("RestAPI plugin needs to be enabled to use the RestAPIV2 plugin")
	}

	routeGroup := deps.Echo.Group("/api/v2")

	attacherOpts := []tangle.MessageAttacherOption{
		tangle.WithTimeout(messageProcessedTimeout),
		tangle.WithDeserializationParameters(deps.DeserializationParameters),
		tangle.WithMinPoWScore(deps.MinPoWScore),
	}
	if deps.TipSelector != nil {
		attacherOpts = append(attacherOpts, tangle.WithTipSel(deps.TipSelector.SelectNonLazyTips))
	}

	// Check for features
	if deps.NodeConfig.Bool(restapi.CfgRestAPIPoWEnabled) {
		AddFeature("PoW")
		attacherOpts = append(attacherOpts, tangle.WithPoW(deps.PoWHandler, deps.NodeConfig.Int(restapi.CfgRestAPIPoWWorkerCount)))
	}

	attacher = deps.Tangle.MessageAttacher(attacherOpts...)

	routeGroup.GET(RouteInfo, func(c echo.Context) error {
		resp, err := info()
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	// only handle tips api calls if the URTS plugin is enabled
	if deps.TipSelector != nil {
		routeGroup.GET(RouteTips, func(c echo.Context) error {
			resp, err := tips(c)
			if err != nil {
				return err
			}
			return restapipkg.JSONResponse(c, http.StatusOK, resp)
		})
	}

	routeGroup.GET(RouteMessageMetadata, func(c echo.Context) error {
		resp, err := messageMetadataByID(c)
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteMessage, func(c echo.Context) error {
		mimeType, err := restapipkg.GetAcceptHeaderContentType(c, restapipkg.MIMEApplicationVendorIOTASerializerV1, echo.MIMEApplicationJSON)
		if err != nil && err != restapipkg.ErrNotAcceptable {
			return err
		}

		switch mimeType {
		case restapipkg.MIMEApplicationVendorIOTASerializerV1:
			resp, err := messageBytesByID(c)
			if err != nil {
				return err
			}
			return c.Blob(http.StatusOK, restapipkg.MIMEApplicationVendorIOTASerializerV1, resp)

		default:
			// default to echo.MIMEApplicationJSON
			resp, err := messageByID(c)
			if err != nil {
				return err
			}
			return restapipkg.JSONResponse(c, http.StatusOK, resp)
		}
	})

	routeGroup.GET(RouteMessageChildren, func(c echo.Context) error {
		resp, err := childrenIDsByID(c)
		if err != nil {
			return err
		}

		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.POST(RouteMessages, func(c echo.Context) error {
		resp, err := sendMessage(c)
		if err != nil {
			return err
		}
		c.Response().Header().Set(echo.HeaderLocation, resp.MessageID)
		return restapipkg.JSONResponse(c, http.StatusCreated, resp)
	})

	routeGroup.GET(RouteTransactionsIncludedMessage, func(c echo.Context) error {
		mimeType, err := restapipkg.GetAcceptHeaderContentType(c, restapipkg.MIMEApplicationVendorIOTASerializerV1, echo.MIMEApplicationJSON)
		if err != nil && err != restapipkg.ErrNotAcceptable {
			return err
		}

		switch mimeType {
		case restapipkg.MIMEApplicationVendorIOTASerializerV1:
			resp, err := messageBytesByTransactionID(c)
			if err != nil {
				return err
			}
			return c.Blob(http.StatusOK, restapipkg.MIMEApplicationVendorIOTASerializerV1, resp)

		default:
			// default to echo.MIMEApplicationJSON
			resp, err := messageByTransactionID(c)
			if err != nil {
				return err
			}
			return restapipkg.JSONResponse(c, http.StatusOK, resp)
		}
	})

	routeGroup.GET(RouteMilestone, func(c echo.Context) error {
		resp, err := milestoneByIndex(c)
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteMilestoneUTXOChanges, func(c echo.Context) error {
		resp, err := milestoneUTXOChangesByIndex(c)
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteOutput, func(c echo.Context) error {
		mimeType, err := restapipkg.GetAcceptHeaderContentType(c, restapipkg.MIMEApplicationVendorIOTASerializerV1, echo.MIMEApplicationJSON)
		if err != nil && err != restapipkg.ErrNotAcceptable {
			return err
		}

		switch mimeType {
		case restapipkg.MIMEApplicationVendorIOTASerializerV1:
			resp, err := rawOutputByID(c)
			if err != nil {
				return err
			}
			return c.Blob(http.StatusOK, restapipkg.MIMEApplicationVendorIOTASerializerV1, resp)

		default:
			// default to echo.MIMEApplicationJSON
			resp, err := outputByID(c, false)
			if err != nil {
				return err
			}
			return restapipkg.JSONResponse(c, http.StatusOK, resp)
		}
	})

	routeGroup.GET(RouteOutputMetadata, func(c echo.Context) error {
		resp, err := outputByID(c, true)
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteTreasury, func(c echo.Context) error {
		resp, err := treasury(c)
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteReceipts, func(c echo.Context) error {
		resp, err := receipts(c)
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteReceiptsMigratedAtIndex, func(c echo.Context) error {
		resp, err := receiptsByMigratedAtIndex(c)
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RoutePeer, func(c echo.Context) error {
		resp, err := getPeer(c)
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.DELETE(RoutePeer, func(c echo.Context) error {
		if err := removePeer(c); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})

	routeGroup.GET(RoutePeers, func(c echo.Context) error {
		resp, err := listPeers(c)
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.POST(RouteComputeWhiteFlagMutations, func(c echo.Context) error {
		resp, err := computeWhiteFlagMutations(c)
		if err != nil {
			return err
		}

		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.POST(RoutePeers, func(c echo.Context) error {
		resp, err := addPeer(c)
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.POST(RouteControlDatabasePrune, func(c echo.Context) error {
		resp, err := pruneDatabase(c)
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.POST(RouteControlSnapshotsCreate, func(c echo.Context) error {
		resp, err := createSnapshots(c)
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})
}

// AddFeature adds a feature to the RouteInfo endpoint.
func AddFeature(feature string) {
	features = append(features, feature)
}
