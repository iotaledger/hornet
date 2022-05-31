package v2

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hornet/core/protocfg"
	"github.com/iotaledger/hornet/pkg/metrics"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	"github.com/iotaledger/hornet/pkg/p2p"
	"github.com/iotaledger/hornet/pkg/pow"
	"github.com/iotaledger/hornet/pkg/protocol/gossip"
	restapipkg "github.com/iotaledger/hornet/pkg/restapi"
	"github.com/iotaledger/hornet/pkg/snapshot"
	"github.com/iotaledger/hornet/pkg/tangle"
	"github.com/iotaledger/hornet/pkg/tipselect"
	"github.com/iotaledger/hornet/plugins/restapi"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// RouteInfo is the route for getting the node info.
	// GET returns the node info.
	RouteInfo = "/info"

	// RouteTips is the route for getting tips.
	// GET returns the tips.
	RouteTips = "/tips"

	// RouteBlock is the route for getting a block by its blockID.
	// GET returns the block based on the given type in the request "Accept" header.
	// MIMEApplicationJSON => json
	// MIMEVendorIOTASerializer => bytes
	RouteBlock = "/blocks/:" + restapipkg.ParameterBlockID

	// RouteBlockMetadata is the route for getting block metadata by its blockID.
	// GET returns block metadata (including info about "promotion/reattachment needed").
	RouteBlockMetadata = "/blocks/:" + restapipkg.ParameterBlockID + "/metadata"

	// RouteBlocks is the route for creating new blocks.
	// POST creates a single new block and returns the new block ID.
	// The block is parsed based on the given type in the request "Content-Type" header.
	// MIMEApplicationJSON => json
	// MIMEVendorIOTASerializer => bytes
	RouteBlocks = "/blocks"

	// RouteTransactionsIncludedBlock is the route for getting the block that was included in the ledger for a given transaction ID.
	// GET returns the block based on the given type in the request "Accept" header.
	// MIMEApplicationJSON => json
	// MIMEVendorIOTASerializer => bytes
	RouteTransactionsIncludedBlock = "/transactions/:" + restapipkg.ParameterTransactionID + "/included-block"

	// RouteMilestoneByID is the route for getting a milestone by its ID.
	// GET returns the milestone.
	// MIMEApplicationJSON => json
	// MIMEVendorIOTASerializer => bytes
	RouteMilestoneByID = "/milestones/:" + restapipkg.ParameterMilestoneID

	// RouteMilestoneByIDUTXOChanges is the route for getting all UTXO changes of a milestone by its ID.
	// GET returns the output IDs of all UTXO changes.
	RouteMilestoneByIDUTXOChanges = "/milestones/:" + restapipkg.ParameterMilestoneID + "/utxo-changes"

	// RouteMilestoneByIndex is the route for getting a milestone by its milestoneIndex.
	// GET returns the milestone.
	// MIMEApplicationJSON => json
	// MIMEVendorIOTASerializer => bytes
	RouteMilestoneByIndex = "/milestones/by-index/:" + restapipkg.ParameterMilestoneIndex

	// RouteMilestoneByIndexUTXOChanges is the route for getting all UTXO changes of a milestone by its milestoneIndex.
	// GET returns the output IDs of all UTXO changes.
	RouteMilestoneByIndexUTXOChanges = "/milestones/by-index/:" + restapipkg.ParameterMilestoneIndex + "/utxo-changes"

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
	Plugin = &app.Plugin{
		Status: app.StatusEnabled,
		Component: &app.Component{
			Name:      "RestAPIV2",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Configure: configure,
		},
	}
}

var (
	Plugin   *app.Plugin
	features = []string{}
	attacher *tangle.BlockAttacher

	deps dependencies
)

type dependencies struct {
	dig.In
	Storage                 *storage.Storage
	SyncManager             *syncmanager.SyncManager
	Tangle                  *tangle.Tangle
	TipScoreCalculator      *tangle.TipScoreCalculator
	PeeringManager          *p2p.Manager
	GossipService           *gossip.Service
	UTXOManager             *utxo.Manager
	PoWHandler              *pow.Handler
	SnapshotManager         *snapshot.SnapshotManager
	AppInfo                 *app.AppInfo
	PeeringConfigManager    *p2p.ConfigManager
	ProtocolParameters      *iotago.ProtocolParameters
	BaseToken               *protocfg.BaseToken
	RestAPILimitsMaxResults int                        `name:"restAPILimitsMaxResults"`
	SnapshotsFullPath       string                     `name:"snapshotsFullPath"`
	SnapshotsDeltaPath      string                     `name:"snapshotsDeltaPath"`
	TipSelector             *tipselect.TipSelector     `optional:"true"`
	Echo                    *echo.Echo                 `optional:"true"`
	RestPluginManager       *restapi.RestPluginManager `optional:"true"`
	RestAPIMetrics          *metrics.RestAPIMetrics
}

func configure() error {
	// check if RestAPI plugin is disabled
	if Plugin.App.IsPluginSkipped(restapi.Plugin) {
		Plugin.LogPanic("RestAPI plugin needs to be enabled to use the RestAPIV2 plugin")
	}

	routeGroup := deps.Echo.Group("/api/v2")

	attacherOpts := []tangle.BlockAttacherOption{
		tangle.WithTimeout(blockProcessedTimeout),
		tangle.WithPoWMetrics(deps.RestAPIMetrics),
	}
	if deps.TipSelector != nil {
		attacherOpts = append(attacherOpts, tangle.WithTipSel(deps.TipSelector.SelectNonLazyTips))
	}

	// Check for features
	if restapi.ParamsRestAPI.PoW.Enabled {
		AddFeature("PoW")
		attacherOpts = append(attacherOpts, tangle.WithPoW(deps.PoWHandler, restapi.ParamsRestAPI.PoW.WorkerCount))
	}

	attacher = deps.Tangle.BlockAttacher(attacherOpts...)

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

	routeGroup.GET(RouteBlockMetadata, func(c echo.Context) error {
		resp, err := blockMetadataByID(c)
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteBlock, func(c echo.Context) error {
		mimeType, err := restapipkg.GetAcceptHeaderContentType(c, restapipkg.MIMEApplicationVendorIOTASerializerV1, echo.MIMEApplicationJSON)
		if err != nil && err != restapipkg.ErrNotAcceptable {
			return err
		}

		switch mimeType {
		case restapipkg.MIMEApplicationVendorIOTASerializerV1:
			resp, err := blockBytesByID(c)
			if err != nil {
				return err
			}
			return c.Blob(http.StatusOK, restapipkg.MIMEApplicationVendorIOTASerializerV1, resp)

		default:
			// default to echo.MIMEApplicationJSON
			resp, err := blockByID(c)
			if err != nil {
				return err
			}
			return restapipkg.JSONResponse(c, http.StatusOK, resp)
		}
	})

	routeGroup.POST(RouteBlocks, func(c echo.Context) error {
		resp, err := sendBlock(c)
		if err != nil {
			return err
		}
		c.Response().Header().Set(echo.HeaderLocation, resp.BlockID)
		return restapipkg.JSONResponse(c, http.StatusCreated, resp)
	})

	routeGroup.GET(RouteTransactionsIncludedBlock, func(c echo.Context) error {
		mimeType, err := restapipkg.GetAcceptHeaderContentType(c, restapipkg.MIMEApplicationVendorIOTASerializerV1, echo.MIMEApplicationJSON)
		if err != nil && err != restapipkg.ErrNotAcceptable {
			return err
		}

		switch mimeType {
		case restapipkg.MIMEApplicationVendorIOTASerializerV1:
			resp, err := blockBytesByTransactionID(c)
			if err != nil {
				return err
			}
			return c.Blob(http.StatusOK, restapipkg.MIMEApplicationVendorIOTASerializerV1, resp)

		default:
			// default to echo.MIMEApplicationJSON
			resp, err := blockByTransactionID(c)
			if err != nil {
				return err
			}
			return restapipkg.JSONResponse(c, http.StatusOK, resp)
		}
	})

	routeGroup.GET(RouteMilestoneByID, func(c echo.Context) error {
		mimeType, err := restapipkg.GetAcceptHeaderContentType(c, restapipkg.MIMEApplicationVendorIOTASerializerV1, echo.MIMEApplicationJSON)
		if err != nil && err != restapipkg.ErrNotAcceptable {
			return err
		}

		switch mimeType {
		case restapipkg.MIMEApplicationVendorIOTASerializerV1:
			resp, err := milestoneBytesByID(c)
			if err != nil {
				return err
			}
			return c.Blob(http.StatusOK, restapipkg.MIMEApplicationVendorIOTASerializerV1, resp)

		default:
			// default to echo.MIMEApplicationJSON
			resp, err := milestoneByID(c)
			if err != nil {
				return err
			}
			return restapipkg.JSONResponse(c, http.StatusOK, resp)
		}
	})

	routeGroup.GET(RouteMilestoneByIDUTXOChanges, func(c echo.Context) error {
		resp, err := milestoneUTXOChangesByID(c)
		if err != nil {
			return err
		}
		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteMilestoneByIndex, func(c echo.Context) error {
		mimeType, err := restapipkg.GetAcceptHeaderContentType(c, restapipkg.MIMEApplicationVendorIOTASerializerV1, echo.MIMEApplicationJSON)
		if err != nil && err != restapipkg.ErrNotAcceptable {
			return err
		}

		switch mimeType {
		case restapipkg.MIMEApplicationVendorIOTASerializerV1:
			resp, err := milestoneBytesByIndex(c)
			if err != nil {
				return err
			}
			return c.Blob(http.StatusOK, restapipkg.MIMEApplicationVendorIOTASerializerV1, resp)

		default:
			// default to echo.MIMEApplicationJSON
			resp, err := milestoneByIndex(c)
			if err != nil {
				return err
			}
			return restapipkg.JSONResponse(c, http.StatusOK, resp)
		}
	})

	routeGroup.GET(RouteMilestoneByIndexUTXOChanges, func(c echo.Context) error {
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
			resp, err := outputByID(c)
			if err != nil {
				return err
			}
			return restapipkg.JSONResponse(c, http.StatusOK, resp)
		}
	})

	routeGroup.GET(RouteOutputMetadata, func(c echo.Context) error {
		resp, err := outputMetadataByID(c)
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

	return nil
}

// AddFeature adds a feature to the RouteInfo endpoint.
func AddFeature(feature string) {
	features = append(features, feature)
}
