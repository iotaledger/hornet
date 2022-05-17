package dashboard

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/pkg/errors"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/daemon"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/jwt"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	restapipkg "github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/restapi"
	restapiv2 "github.com/gohornet/hornet/plugins/restapi/v2"
	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/basicauth"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/websockethub"
)

func init() {
	Plugin = &app.Plugin{
		Status: app.StatusEnabled,
		Component: &app.Component{
			Name:           "Dashboard",
			DepsFunc:       func(cDeps dependencies) { deps = cDeps },
			Params:         params,
			InitConfigPars: initConfigPars,
			Configure:      configure,
			Run:            run,
		},
	}
}

const (
	broadcastQueueSize    = 20000
	clientSendChannelSize = 1000
)

var (
	maxWebsocketMessageSize int64 = 400 + maxDashboardAuthUsernameSize + 10 // 10 buffer due to variable JWT lengths

	Plugin *app.Plugin
	deps   dependencies

	nodeStartAt = time.Now()

	webSocketWriteTimeout = time.Duration(3) * time.Second

	hub      *websockethub.Hub
	upgrader *websocket.Upgrader

	basicAuth *basicauth.BasicAuth
	jwtAuth   *jwt.JWTAuth

	cachedMilestoneMetrics []*tangle.ConfirmedMilestoneMetric
)

type dependencies struct {
	dig.In
	TangleDatabase           *database.Database `name:"tangleDatabase"`
	UTXODatabase             *database.Database `name:"utxoDatabase"`
	Storage                  *storage.Storage
	SyncManager              *syncmanager.SyncManager
	Tangle                   *tangle.Tangle
	ServerMetrics            *metrics.ServerMetrics
	RequestQueue             gossip.RequestQueue
	PeeringManager           *p2p.Manager
	MessageProcessor         *gossip.MessageProcessor
	TipSelector              *tipselect.TipSelector `optional:"true"`
	RestAPIBindAddress       string                 `name:"restAPIBindAddress"`
	AppInfo                  *app.AppInfo
	Host                     host.Host
	NodePrivateKey           crypto.PrivKey          `name:"nodePrivateKey"`
	DashboardAllowedAPIRoute restapipkg.AllowedRoute `name:"dashboardAllowedAPIRoute" optional:"true"`
}

func initConfigPars(c *dig.Container) error {

	type cfgResult struct {
		dig.Out
		DashboardAuthUsername string `name:"dashboardAuthUsername"`
	}

	if err := c.Provide(func() cfgResult {

		username := ParamsDashboard.Auth.Username
		if len(username) == 0 {
			Plugin.LogPanicf("%s cannot be empty", Plugin.App.Config().GetParameterPath(&(ParamsDashboard.Auth.Username)))
		}
		if len(username) > maxDashboardAuthUsernameSize {
			Plugin.LogPanicf("%s has a max length of %d", Plugin.App.Config().GetParameterPath(&(ParamsDashboard.Auth.Username)), maxDashboardAuthUsernameSize)
		}

		return cfgResult{
			DashboardAuthUsername: username,
		}
	}); err != nil {
		Plugin.LogPanic(err)
	}

	return nil
}

func configure() error {

	// check if RestAPI plugin is disabled
	if Plugin.App.IsPluginSkipped(restapi.Plugin) {
		Plugin.LogPanic("RestAPI plugin needs to be enabled to use the Dashboard plugin")
	}

	// check if RestAPIV2 plugin is disabled
	if Plugin.App.IsPluginSkipped(restapiv2.Plugin) {
		Plugin.LogPanic("RestAPIV2 plugin needs to be enabled to use the Dashboard plugin")
	}

	upgrader = &websocket.Upgrader{
		HandshakeTimeout: webSocketWriteTimeout,
		CheckOrigin:      func(r *http.Request) bool { return true }, // allow any origin for websocket connections
		// Disable compression due to incompatibilities with latest Safari browsers:
		// https://github.com/tilt-dev/tilt/issues/4746
		// https://github.com/gorilla/websocket/issues/731
		EnableCompression: false,
	}

	hub = websockethub.NewHub(Plugin.Logger(), upgrader, broadcastQueueSize, clientSendChannelSize, maxWebsocketMessageSize)

	var err error
	basicAuth, err = basicauth.NewBasicAuth(ParamsDashboard.Auth.Username,
		ParamsDashboard.Auth.PasswordHash,
		ParamsDashboard.Auth.PasswordSalt)
	if err != nil {
		Plugin.LogPanicf("basic auth initialization failed: %w", err)
	}

	jwtAuth, err = jwt.NewJWTAuth(
		ParamsDashboard.Auth.Username,
		ParamsDashboard.Auth.SessionTimeout,
		deps.Host.ID().String(),
		deps.NodePrivateKey,
	)
	if err != nil {
		Plugin.LogPanicf("JWT auth initialization failed: %w", err)
	}

	return nil
}

func run() error {

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	setupRoutes(e)
	bindAddr := ParamsDashboard.BindAddress

	go func() {
		Plugin.LogInfof("You can now access the dashboard using: http://%s", bindAddr)

		if err := e.Start(bindAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			Plugin.LogWarnf("Stopped dashboard server due to an error (%s)", err)
		}
	}()

	onBPSMetricsUpdated := events.NewClosure(func(bpsMetrics *tangle.BPSMetrics) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeBPSMetric, Data: bpsMetrics})
		hub.BroadcastMsg(&Msg{Type: MsgTypePublicNodeStatus, Data: currentPublicNodeStatus()})
		hub.BroadcastMsg(&Msg{Type: MsgTypeNodeStatus, Data: currentNodeStatus()})
		hub.BroadcastMsg(&Msg{Type: MsgTypePeerMetric, Data: peerMetrics()})
	})

	onConfirmedMilestoneIndexChanged := events.NewClosure(func(_ milestone.Index) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeSyncStatus, Data: currentSyncStatus()})
	})

	onLatestMilestoneIndexChanged := events.NewClosure(func(_ milestone.Index) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeSyncStatus, Data: currentSyncStatus()})
	})

	onNewConfirmedMilestoneMetric := events.NewClosure(func(metric *tangle.ConfirmedMilestoneMetric) {
		cachedMilestoneMetrics = append(cachedMilestoneMetrics, metric)
		if len(cachedMilestoneMetrics) > 20 {
			cachedMilestoneMetrics = cachedMilestoneMetrics[len(cachedMilestoneMetrics)-20:]
		}
		hub.BroadcastMsg(&Msg{Type: MsgTypeConfirmedMsMetrics, Data: []*tangle.ConfirmedMilestoneMetric{metric}})
	})

	if err := Plugin.Daemon().BackgroundWorker("Dashboard[WSSend]", func(ctx context.Context) {
		go hub.Run(ctx)
		deps.Tangle.Events.BPSMetricsUpdated.Attach(onBPSMetricsUpdated)
		deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Attach(onConfirmedMilestoneIndexChanged)
		deps.Tangle.Events.LatestMilestoneIndexChanged.Attach(onLatestMilestoneIndexChanged)
		deps.Tangle.Events.NewConfirmedMilestoneMetric.Attach(onNewConfirmedMilestoneMetric)
		<-ctx.Done()
		Plugin.LogInfo("Stopping Dashboard[WSSend] ...")
		deps.Tangle.Events.BPSMetricsUpdated.Detach(onBPSMetricsUpdated)
		deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Detach(onConfirmedMilestoneIndexChanged)
		deps.Tangle.Events.LatestMilestoneIndexChanged.Detach(onLatestMilestoneIndexChanged)
		deps.Tangle.Events.NewConfirmedMilestoneMetric.Detach(onNewConfirmedMilestoneMetric)

		Plugin.LogInfo("Stopping Dashboard[WSSend] ... done")
	}, daemon.PriorityDashboard); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}

	// run the message live feed
	runLiveFeed()
	// run the visualizer message feed
	runVisualizer()

	if deps.TipSelector != nil {
		// run the tipselection feed
		runTipSelMetricWorker()
	}

	// run the database size collector
	runDatabaseSizeCollector()
	// run the spammer feed
	runSpammerMetricWorker()

	return nil
}

func getMilestoneIDHex(index milestone.Index) (string, error) {
	cachedMilestone := deps.Storage.CachedMilestoneByIndexOrNil(index) // milestone +1
	if cachedMilestone == nil {
		return "", storage.ErrMilestoneNotFound
	}
	defer cachedMilestone.Release(true) // milestone -1

	return cachedMilestone.Milestone().MilestoneIDHex(), nil
}

// Msg represents a websocket message.
type Msg struct {
	Type byte        `json:"type"`
	Data interface{} `json:"data"`
}

// LivefeedMilestone represents a milestone for the livefeed.
type LivefeedMilestone struct {
	MilestoneID string          `json:"milestoneId"`
	Index       milestone.Index `json:"index"`
}

// SyncStatus represents the node sync status.
type SyncStatus struct {
	CMI milestone.Index `json:"cmi"`
	LMI milestone.Index `json:"lmi"`
}

// PublicNodeStatus represents the public node status.
type PublicNodeStatus struct {
	SnapshotIndex milestone.Index `json:"snapshot_index"`
	PruningIndex  milestone.Index `json:"pruning_index"`
	IsHealthy     bool            `json:"is_healthy"`
	IsSynced      bool            `json:"is_synced"`
}

// NodeStatus represents the node status.
type NodeStatus struct {
	Version                string          `json:"version"`
	LatestVersion          string          `json:"latest_version"`
	Uptime                 int64           `json:"uptime"`
	NodeID                 string          `json:"node_id"`
	NodeAlias              string          `json:"node_alias"`
	ConnectedPeersCount    int             `json:"connected_peers_count"`
	CurrentRequestedMs     milestone.Index `json:"current_requested_ms"`
	RequestQueueQueued     int             `json:"request_queue_queued"`
	RequestQueuePending    int             `json:"request_queue_pending"`
	RequestQueueProcessing int             `json:"request_queue_processing"`
	RequestQueueAvgLatency int64           `json:"request_queue_avg_latency"`
	ServerMetrics          *ServerMetrics  `json:"server_metrics"`
	Mem                    *MemMetrics     `json:"mem"`
	Caches                 *CachesMetric   `json:"caches"`
}

// ServerMetrics are global metrics of the server.
type ServerMetrics struct {
	AllBlocks                 uint32 `json:"all_blocks"`
	NewBlocks                 uint32 `json:"new_blocks"`
	KnownBlocks               uint32 `json:"known_blocks"`
	InvalidBlocks             uint32 `json:"invalid_blocks"`
	InvalidRequests           uint32 `json:"invalid_req"`
	ReceivedBlockRequests     uint32 `json:"rec_block_req"`
	ReceivedMilestoneRequests uint32 `json:"rec_ms_req"`
	ReceivedHeartbeats        uint32 `json:"rec_heartbeat"`
	SentBlocks                uint32 `json:"sent_blocks"`
	SentBlockRequests         uint32 `json:"sent_block_req"`
	SentMilestoneRequests     uint32 `json:"sent_ms_req"`
	SentHeartbeats            uint32 `json:"sent_heartbeat"`
	DroppedSentPackets        uint32 `json:"dropped_sent_packets"`
	SentSpamBlocksCount       uint32 `json:"sent_spam_blocks"`
	ValidatedBlocks           uint32 `json:"validated_blocks"`
}

// MemMetrics represents memory metrics.
type MemMetrics struct {
	Sys          uint64 `json:"sys"`
	HeapSys      uint64 `json:"heap_sys"`
	HeapInuse    uint64 `json:"heap_inuse"`
	HeapIdle     uint64 `json:"heap_idle"`
	HeapReleased uint64 `json:"heap_released"`
	HeapObjects  uint64 `json:"heap_objects"`
	MSpanInuse   uint64 `json:"m_span_inuse"`
	MCacheInuse  uint64 `json:"m_cache_inuse"`
	StackSys     uint64 `json:"stack_sys"`
	NumGC        uint32 `json:"num_gc"`
	LastPauseGC  uint64 `json:"last_pause_gc"`
}

// CachesMetric represents cache metrics.
type CachesMetric struct {
	RequestQueue            Cache `json:"request_queue"`
	Children                Cache `json:"children"`
	Milestones              Cache `json:"milestones"`
	Blocks                  Cache `json:"blocks"`
	IncomingBlocksWorkUnits Cache `json:"incoming_block_work_units"`
}

// Cache represents metrics about a cache.
type Cache struct {
	Size int `json:"size"`
}

func peerMetrics() []*restapiv2.PeerResponse {
	peerInfos := deps.PeeringManager.PeerInfoSnapshots()
	results := make([]*restapiv2.PeerResponse, len(peerInfos))
	for i, info := range peerInfos {
		results[i] = restapiv2.WrapInfoSnapshot(info)
	}
	return results
}

func currentSyncStatus() *SyncStatus {
	return &SyncStatus{CMI: deps.SyncManager.ConfirmedMilestoneIndex(), LMI: deps.SyncManager.LatestMilestoneIndex()}
}

func currentPublicNodeStatus() *PublicNodeStatus {
	status := &PublicNodeStatus{}

	status.IsHealthy = deps.Tangle.IsNodeHealthy()
	status.IsSynced = deps.SyncManager.IsNodeAlmostSynced()

	snapshotInfo := deps.Storage.SnapshotInfo()
	if snapshotInfo != nil {
		status.SnapshotIndex = snapshotInfo.SnapshotIndex
		status.PruningIndex = snapshotInfo.PruningIndex
	}

	return status
}

func currentNodeStatus() *NodeStatus {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	status := &NodeStatus{}

	// node status
	var requestedMilestone milestone.Index
	peekedRequest := deps.RequestQueue.Peek()
	queued, pending, processing := deps.RequestQueue.Size()
	if peekedRequest != nil {
		requestedMilestone = peekedRequest.MilestoneIndex
	}

	status.Version = deps.AppInfo.Version
	status.LatestVersion = deps.AppInfo.LatestGitHubVersion
	status.Uptime = time.Since(nodeStartAt).Milliseconds()
	status.NodeAlias = ParamsNode.Alias
	status.NodeID = deps.Host.ID().String()

	status.ConnectedPeersCount = deps.PeeringManager.ConnectedCount()
	status.CurrentRequestedMs = requestedMilestone
	status.RequestQueueQueued = queued
	status.RequestQueuePending = pending
	status.RequestQueueProcessing = processing
	status.RequestQueueAvgLatency = deps.RequestQueue.AvgLatency()

	// cache metrics
	status.Caches = &CachesMetric{
		Children: Cache{
			Size: deps.Storage.ChildrenStorageSize(),
		},
		RequestQueue: Cache{
			Size: queued + pending,
		},
		Milestones: Cache{
			Size: deps.Storage.MilestoneStorageSize(),
		},
		Blocks: Cache{
			Size: deps.Storage.BlockStorageSize(),
		},
		IncomingBlocksWorkUnits: Cache{
			Size: deps.MessageProcessor.WorkUnitsSize(),
		},
	}

	// server metrics
	status.ServerMetrics = &ServerMetrics{
		AllBlocks:                 deps.ServerMetrics.Blocks.Load(),
		NewBlocks:                 deps.ServerMetrics.NewBlocks.Load(),
		KnownBlocks:               deps.ServerMetrics.KnownBlocks.Load(),
		InvalidBlocks:             deps.ServerMetrics.InvalidBlocks.Load(),
		InvalidRequests:           deps.ServerMetrics.InvalidRequests.Load(),
		ReceivedBlockRequests:     deps.ServerMetrics.ReceivedBlockRequests.Load(),
		ReceivedMilestoneRequests: deps.ServerMetrics.ReceivedMilestoneRequests.Load(),
		ReceivedHeartbeats:        deps.ServerMetrics.ReceivedHeartbeats.Load(),
		SentBlocks:                deps.ServerMetrics.SentBlocks.Load(),
		SentBlockRequests:         deps.ServerMetrics.SentBlockRequests.Load(),
		SentMilestoneRequests:     deps.ServerMetrics.SentMilestoneRequests.Load(),
		SentHeartbeats:            deps.ServerMetrics.SentHeartbeats.Load(),
		DroppedSentPackets:        deps.ServerMetrics.DroppedPackets.Load(),
		SentSpamBlocksCount:       deps.ServerMetrics.SentSpamBlocks.Load(),
		ValidatedBlocks:           deps.ServerMetrics.ValidatedBlocks.Load(),
	}

	// memory metrics
	status.Mem = &MemMetrics{
		Sys:          m.Sys,
		HeapSys:      m.HeapSys,
		HeapInuse:    m.HeapInuse,
		HeapIdle:     m.HeapIdle,
		HeapReleased: m.HeapReleased,
		HeapObjects:  m.HeapObjects,
		MSpanInuse:   m.MSpanInuse,
		MCacheInuse:  m.MCacheInuse,
		StackSys:     m.StackSys,
		NumGC:        m.NumGC,
		LastPauseGC:  m.PauseNs[(m.NumGC+255)%256],
	}
	return status
}
