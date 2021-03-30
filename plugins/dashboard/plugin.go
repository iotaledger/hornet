package dashboard

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/pkg/app"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	restapiv1 "github.com/gohornet/hornet/plugins/restapi/v1"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/websockethub"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.Enabled,
		Pluggable: node.Pluggable{
			Name:      "Dashboard",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Configure: configure,
			Run:       run,
		},
	}
}

const (
	broadcastQueueSize      = 20000
	clientSendChannelSize   = 1000
	maxWebsocketMessageSize = 400
)

var (
	Plugin *node.Plugin
	log    *logger.Logger
	deps   dependencies

	nodeStartAt = time.Now()

	webSocketWriteTimeout = time.Duration(3) * time.Second

	hub      *websockethub.Hub
	upgrader *websocket.Upgrader

	jwtAuth *JWTAuth

	cachedMilestoneMetrics []*tangle.ConfirmedMilestoneMetric
)

type dependencies struct {
	dig.In
	Storage          *storage.Storage
	Tangle           *tangle.Tangle
	ServerMetrics    *metrics.ServerMetrics
	RequestQueue     gossip.RequestQueue
	Manager          *p2p.Manager
	MessageProcessor *gossip.MessageProcessor
	TipSelector      *tipselect.TipSelector       `optional:"true"`
	NodeConfig       *configuration.Configuration `name:"nodeConfig"`
	AppInfo          *app.AppInfo
	Host             host.Host
	NodePrivateKey   crypto.PrivKey
	DatabaseEvents   *database.Events
}

func configure() {
	log = logger.NewLogger(Plugin.Name)

	upgrader = &websocket.Upgrader{
		HandshakeTimeout:  webSocketWriteTimeout,
		CheckOrigin:       func(r *http.Request) bool { return true }, // allow any origin for websocket connections
		EnableCompression: true,
	}

	hub = websockethub.NewHub(log, upgrader, broadcastQueueSize, clientSendChannelSize, maxWebsocketMessageSize)

	jwtAuth = NewJWTAuth(deps.NodeConfig.String(CfgDashboardAuthUsername),
		deps.NodeConfig.String(CfgDashboardAuthPasswordHash),
		deps.NodeConfig.String(CfgDashboardAuthPasswordSalt),
		deps.NodeConfig.Duration(CfgDashboardAuthSessionTimeout),
		deps.Host.ID().String(),
		deps.NodePrivateKey,
	)
}

func run() {

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	setupRoutes(e)
	bindAddr := deps.NodeConfig.String(CfgDashboardBindAddress)
	log.Infof("You can now access the dashboard using: http://%s", bindAddr)
	go e.Start(bindAddr)

	onMPSMetricsUpdated := events.NewClosure(func(mpsMetrics *tangle.MPSMetrics) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeMPSMetric, Data: mpsMetrics})
		hub.BroadcastMsg(&Msg{Type: MsgTypePublicNodeStatus, Data: currentPublicNodeStatus()})
		hub.BroadcastMsg(&Msg{Type: MsgTypeNodeStatus, Data: currentNodeStatus()})
		hub.BroadcastMsg(&Msg{Type: MsgTypePeerMetric, Data: peerMetrics()})
	})

	onConfirmedMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeSyncStatus, Data: currentSyncStatus()})
	})

	onLatestMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeSyncStatus, Data: currentSyncStatus()})
	})

	onNewConfirmedMilestoneMetric := events.NewClosure(func(metric *tangle.ConfirmedMilestoneMetric) {
		cachedMilestoneMetrics = append(cachedMilestoneMetrics, metric)
		if len(cachedMilestoneMetrics) > 20 {
			cachedMilestoneMetrics = cachedMilestoneMetrics[len(cachedMilestoneMetrics)-20:]
		}
		hub.BroadcastMsg(&Msg{Type: MsgTypeConfirmedMsMetrics, Data: []*tangle.ConfirmedMilestoneMetric{metric}})
	})

	Plugin.Daemon().BackgroundWorker("Dashboard[WSSend]", func(shutdownSignal <-chan struct{}) {
		go hub.Run(shutdownSignal)
		deps.Tangle.Events.MPSMetricsUpdated.Attach(onMPSMetricsUpdated)
		deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Attach(onConfirmedMilestoneIndexChanged)
		deps.Tangle.Events.LatestMilestoneIndexChanged.Attach(onLatestMilestoneIndexChanged)
		deps.Tangle.Events.NewConfirmedMilestoneMetric.Attach(onNewConfirmedMilestoneMetric)
		<-shutdownSignal
		log.Info("Stopping Dashboard[WSSend] ...")
		deps.Tangle.Events.MPSMetricsUpdated.Detach(onMPSMetricsUpdated)
		deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Detach(onConfirmedMilestoneIndexChanged)
		deps.Tangle.Events.LatestMilestoneIndexChanged.Detach(onLatestMilestoneIndexChanged)
		deps.Tangle.Events.NewConfirmedMilestoneMetric.Detach(onNewConfirmedMilestoneMetric)

		log.Info("Stopping Dashboard[WSSend] ... done")
	}, shutdown.PriorityDashboard)

	// run the message live feed
	runLiveFeed()
	// run the visualizer message feed
	runVisualizer()
	// run the tipselection feed
	runTipSelMetricWorker()
	// run the database size collector
	runDatabaseSizeCollector()
	// run the spammer feed
	runSpammerMetricWorker()
}

func getMilestoneMessageID(index milestone.Index) hornet.MessageID {
	cachedMs := deps.Storage.GetMilestoneCachedMessageOrNil(index) // message +1
	if cachedMs == nil {
		return nil
	}
	defer cachedMs.Release(true) // message -1

	return cachedMs.GetMessage().GetMessageID()
}

// Msg represents a websocket message.
type Msg struct {
	Type byte        `json:"type"`
	Data interface{} `json:"data"`
}

// LivefeedMilestone represents a milestone for the livefeed.
type LivefeedMilestone struct {
	MessageID string          `json:"messageID"`
	Index     milestone.Index `json:"index"`
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
	AllMessages          uint32 `json:"all_msgs"`
	NewMessages          uint32 `json:"new_msgs"`
	KnownMessages        uint32 `json:"known_msgs"`
	InvalidMessages      uint32 `json:"invalid_msgs"`
	InvalidRequests      uint32 `json:"invalid_req"`
	ReceivedMessageReq   uint32 `json:"rec_msg_req"`
	ReceivedMilestoneReq uint32 `json:"rec_ms_req"`
	ReceivedHeartbeats   uint32 `json:"rec_heartbeat"`
	SentMessages         uint32 `json:"sent_msgs"`
	SentMessageReq       uint32 `json:"sent_msg_req"`
	SentMilestoneReq     uint32 `json:"sent_ms_req"`
	SentHeartbeats       uint32 `json:"sent_heartbeat"`
	DroppedSentPackets   uint32 `json:"dropped_sent_packets"`
	SentSpamMsgsCount    uint32 `json:"sent_spam_messages"`
	ValidatedMessages    uint32 `json:"validated_messages"`
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
	RequestQueue             Cache `json:"request_queue"`
	Children                 Cache `json:"children"`
	Milestones               Cache `json:"milestones"`
	Messages                 Cache `json:"messages"`
	IncomingMessageWorkUnits Cache `json:"incoming_message_work_units"`
}

// Cache represents metrics about a cache.
type Cache struct {
	Size int `json:"size"`
}

func peerMetrics() []*restapiv1.PeerResponse {
	peerInfos := deps.Manager.PeerInfoSnapshots()
	results := make([]*restapiv1.PeerResponse, len(peerInfos))
	for i, info := range peerInfos {
		results[i] = restapiv1.WrapInfoSnapshot(info)
	}
	return results
}

func currentSyncStatus() *SyncStatus {
	return &SyncStatus{CMI: deps.Storage.GetConfirmedMilestoneIndex(), LMI: deps.Storage.GetLatestMilestoneIndex()}
}

func currentPublicNodeStatus() *PublicNodeStatus {
	status := &PublicNodeStatus{}

	status.IsHealthy = deps.Tangle.IsNodeHealthy()
	status.IsSynced = deps.Storage.IsNodeAlmostSynced()

	snapshotInfo := deps.Storage.GetSnapshotInfo()
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
	status.NodeAlias = deps.NodeConfig.String(CfgNodeAlias)
	status.NodeID = deps.Host.ID().String()

	status.ConnectedPeersCount = deps.Manager.ConnectedCount()
	status.CurrentRequestedMs = requestedMilestone
	status.RequestQueueQueued = queued
	status.RequestQueuePending = pending
	status.RequestQueueProcessing = processing
	status.RequestQueueAvgLatency = deps.RequestQueue.AvgLatency()

	// cache metrics
	status.Caches = &CachesMetric{
		Children: Cache{
			Size: deps.Storage.GetChildrenStorageSize(),
		},
		RequestQueue: Cache{
			Size: queued + pending,
		},
		Milestones: Cache{
			Size: deps.Storage.GetMilestoneStorageSize(),
		},
		Messages: Cache{
			Size: deps.Storage.GetMessageStorageSize(),
		},
		IncomingMessageWorkUnits: Cache{
			Size: deps.MessageProcessor.WorkUnitsSize(),
		},
	}

	// server metrics
	status.ServerMetrics = &ServerMetrics{
		AllMessages:          deps.ServerMetrics.Messages.Load(),
		NewMessages:          deps.ServerMetrics.NewMessages.Load(),
		KnownMessages:        deps.ServerMetrics.KnownMessages.Load(),
		InvalidMessages:      deps.ServerMetrics.InvalidMessages.Load(),
		InvalidRequests:      deps.ServerMetrics.InvalidRequests.Load(),
		ReceivedMessageReq:   deps.ServerMetrics.ReceivedMessageRequests.Load(),
		ReceivedMilestoneReq: deps.ServerMetrics.ReceivedMilestoneRequests.Load(),
		ReceivedHeartbeats:   deps.ServerMetrics.ReceivedHeartbeats.Load(),
		SentMessages:         deps.ServerMetrics.SentMessages.Load(),
		SentMessageReq:       deps.ServerMetrics.SentMessageRequests.Load(),
		SentMilestoneReq:     deps.ServerMetrics.SentMilestoneRequests.Load(),
		SentHeartbeats:       deps.ServerMetrics.SentHeartbeats.Load(),
		DroppedSentPackets:   deps.ServerMetrics.DroppedMessages.Load(),
		SentSpamMsgsCount:    deps.ServerMetrics.SentSpamMessages.Load(),
		ValidatedMessages:    deps.ServerMetrics.ValidatedMessages.Load(),
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
