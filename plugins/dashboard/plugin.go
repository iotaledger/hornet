package dashboard

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gohornet/hornet/core/database"
	p2pcore "github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/pkg/p2p"
	gossip2 "github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/libp2p/go-libp2p-core/network"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/websockethub"

	"github.com/gohornet/hornet/core/cli"
	"github.com/gohornet/hornet/core/gossip"
	tanglecore "github.com/gohornet/hornet/core/tangle"
	"github.com/gohornet/hornet/pkg/basicauth"
	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/shutdown"
)

const (
	// MsgTypeSyncStatus is the type of the SyncStatus message.
	MsgTypeSyncStatus byte = iota
	// MsgTypeNodeStatus is the type of the NodeStatus message.
	MsgTypeNodeStatus
	// MsgTypeMPSMetric is the type of the messages per second (MPS) metric message.
	MsgTypeMPSMetric
	// MsgTypeTipSelMetric is the type of the TipSelMetric message.
	MsgTypeTipSelMetric
	// MsgTypeTxZeroValue is the type of the zero value Tx message.
	MsgTypeTxZeroValue
	// MsgTypeTxValue is the type of the value Tx message.
	MsgTypeTxValue
	// MsgTypeMs is the type of the Ms message.
	MsgTypeMs
	// MsgTypePeerMetric is the type of the PeerMetric message.
	MsgTypePeerMetric
	// MsgTypeConfirmedMsMetrics is the type of the ConfirmedMsMetrics message.
	MsgTypeConfirmedMsMetrics
	// MsgTypeVertex is the type of the Vertex message for the visualizer.
	MsgTypeVertex
	// MsgTypeSolidInfo is the type of the SolidInfo message for the visualizer.
	MsgTypeSolidInfo
	// MsgTypeConfirmedInfo is the type of the ConfirmedInfo message for the visualizer.
	MsgTypeConfirmedInfo
	// MsgTypeMilestoneInfo is the type of the MilestoneInfo message for the visualizer.
	MsgTypeMilestoneInfo
	// MsgTypeTipInfo is the type of the TipInfo message for the visualizer.
	MsgTypeTipInfo
	// MsgTypeDatabaseSizeMetric is the type of the database Size message for the metrics.
	MsgTypeDatabaseSizeMetric
	// MsgTypeDatabaseCleanupEvent is the type of the database cleanup message for the metrics.
	MsgTypeDatabaseCleanupEvent
	// MsgTypeSpamMetrics is the type of the SpamMetric message.
	MsgTypeSpamMetrics
	// MsgTypeAvgSpamMetrics is the type of the AvgSpamMetric message.
	MsgTypeAvgSpamMetrics
)

const (
	broadcastQueueSize    = 20000
	clientSendChannelSize = 1000
)

var (
	Plugin *node.Plugin
	log    *logger.Logger

	nodeStartAt = time.Now()

	webSocketWriteTimeout = time.Duration(3) * time.Second

	hub      *websockethub.Hub
	upgrader *websocket.Upgrader

	cachedMilestoneMetrics []*tanglecore.ConfirmedMilestoneMetric
)

func init() {
	Plugin = node.NewPlugin("Dashboard", node.Enabled, configure, run)
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	upgrader = &websocket.Upgrader{
		HandshakeTimeout:  webSocketWriteTimeout,
		CheckOrigin:       func(r *http.Request) bool { return true }, // allow any origin for websocket connections
		EnableCompression: true,
	}

	hub = websockethub.NewHub(log, upgrader, broadcastQueueSize, clientSendChannelSize)
}

func run(_ *node.Plugin) {

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	if config.NodeConfig.Bool(config.CfgDashboardBasicAuthEnabled) {
		// grab auth info
		expectedUsername := config.NodeConfig.String(config.CfgDashboardBasicAuthUsername)
		expectedPasswordHash := config.NodeConfig.String(config.CfgDashboardBasicAuthPasswordHash)
		passwordSalt := config.NodeConfig.String(config.CfgDashboardBasicAuthPasswordSalt)

		if len(expectedUsername) == 0 {
			log.Fatalf("'%s' must not be empty if dashboard basic auth is enabled", config.CfgDashboardBasicAuthUsername)
		}

		if len(expectedPasswordHash) != 64 {
			log.Fatalf("'%s' must be 64 (sha256 hash) in length if dashboard basic auth is enabled", config.CfgDashboardBasicAuthPasswordHash)
		}

		e.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			if username != expectedUsername {
				return false, nil
			}

			if valid, _ := basicauth.VerifyPassword([]byte(password), []byte(passwordSalt), []byte(expectedPasswordHash)); !valid {
				return false, nil
			}

			return true, nil
		}))
	}

	setupRoutes(e)
	bindAddr := config.NodeConfig.String(config.CfgDashboardBindAddress)
	log.Infof("You can now access the dashboard using: http://%s", bindAddr)
	go e.Start(bindAddr)

	onMPSMetricsUpdated := events.NewClosure(func(mpsMetrics *tanglecore.MPSMetrics) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeMPSMetric, Data: mpsMetrics})
		hub.BroadcastMsg(&Msg{Type: MsgTypeNodeStatus, Data: currentNodeStatus()})
		hub.BroadcastMsg(&Msg{Type: MsgTypePeerMetric, Data: peerMetrics()})
	})

	onSolidMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeSyncStatus, Data: currentSyncStatus()})
	})

	onLatestMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		hub.BroadcastMsg(&Msg{Type: MsgTypeSyncStatus, Data: currentSyncStatus()})
	})

	onNewConfirmedMilestoneMetric := events.NewClosure(func(metric *tanglecore.ConfirmedMilestoneMetric) {
		cachedMilestoneMetrics = append(cachedMilestoneMetrics, metric)
		if len(cachedMilestoneMetrics) > 20 {
			cachedMilestoneMetrics = cachedMilestoneMetrics[len(cachedMilestoneMetrics)-20:]
		}
		hub.BroadcastMsg(&Msg{Type: MsgTypeConfirmedMsMetrics, Data: []*tanglecore.ConfirmedMilestoneMetric{metric}})
	})

	Plugin.Daemon().BackgroundWorker("Dashboard[WSSend]", func(shutdownSignal <-chan struct{}) {
		go hub.Run(shutdownSignal)
		tanglecore.Events.MPSMetricsUpdated.Attach(onMPSMetricsUpdated)
		tanglecore.Events.SolidMilestoneIndexChanged.Attach(onSolidMilestoneIndexChanged)
		tanglecore.Events.LatestMilestoneIndexChanged.Attach(onLatestMilestoneIndexChanged)
		tanglecore.Events.NewConfirmedMilestoneMetric.Attach(onNewConfirmedMilestoneMetric)
		<-shutdownSignal
		log.Info("Stopping Dashboard[WSSend] ...")
		tanglecore.Events.MPSMetricsUpdated.Detach(onMPSMetricsUpdated)
		tanglecore.Events.SolidMilestoneIndexChanged.Detach(onSolidMilestoneIndexChanged)
		tanglecore.Events.LatestMilestoneIndexChanged.Detach(onLatestMilestoneIndexChanged)
		tanglecore.Events.NewConfirmedMilestoneMetric.Detach(onNewConfirmedMilestoneMetric)

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

func getMilestoneMessageID(index milestone.Index) *hornet.MessageID {
	cachedMs := database.Tangle().GetMilestoneCachedMessageOrNil(index) // message +1
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

// LivefeedMessage represents a message for the livefeed.
type LivefeedMessage struct {
	MessageID string `json:"messageID"`
	Value     int64  `json:"value"`
}

// LivefeedMilestone represents a milestone for the livefeed.
type LivefeedMilestone struct {
	MessageID string          `json:"messageID"`
	Index     milestone.Index `json:"index"`
}

// SyncStatus represents the node sync status.
type SyncStatus struct {
	LSMI milestone.Index `json:"lsmi"`
	LMI  milestone.Index `json:"lmi"`
}

// NodeStatus represents the node status.
type NodeStatus struct {
	SnapshotIndex          milestone.Index `json:"snapshot_index"`
	PruningIndex           milestone.Index `json:"pruning_index"`
	IsHealthy              bool            `json:"is_healthy"`
	Version                string          `json:"version"`
	LatestVersion          string          `json:"latest_version"`
	Uptime                 int64           `json:"uptime"`
	AutopeeringID          string          `json:"autopeering_id"`
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

// PeerMetric represents metrics of a peer.
type PeerMetric struct {
	Identity         string                `json:"identity"`
	Alias            string                `json:"alias,omitempty"`
	OriginAddr       string                `json:"origin_addr"`
	ConnectionOrigin network.Direction     `json:"connection_origin"`
	ProtocolVersion  uint16                `json:"protocol_version"`
	BytesRead        uint64                `json:"bytes_read"`
	BytesWritten     uint64                `json:"bytes_written"`
	Heartbeat        *gossip2.Heartbeat    `json:"heartbeat"`
	Info             *p2p.PeerInfoSnapshot `json:"info"`
	Connected        bool                  `json:"connected"`
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

func peerMetrics() []*PeerMetric {
	/*
		infos := peering.Manager().PeerInfoSnapshots()
		var stats []*PeerMetric
		for _, info := range infos {
			m := &PeerMetric{
				OriginAddr: info.DomainWithPort,
				Info:       info,
			}
			if info.Peer != nil && info.Peer.Protocol != nil {
				m.Identity = info.Peer.ID
				m.Alias = info.Alias
				m.ConnectionOrigin = info.Peer.ConnectionOrigin
				m.ProtocolVersion = info.Peer.Protocol.Version
				m.BytesRead = info.Peer.Conn.BytesRead()
				m.BytesWritten = info.Peer.Conn.BytesWritten()
				m.Heartbeat = info.Peer.LatestHeartbeat
				m.Connected = info.Connected
			} else {
				m.Identity = info.ID
			}
			stats = append(stats, m)
		}
	*/
	return nil
}

func currentSyncStatus() *SyncStatus {
	return &SyncStatus{LSMI: database.Tangle().GetSolidMilestoneIndex(), LMI: database.Tangle().GetLatestMilestoneIndex()}
}

func currentNodeStatus() *NodeStatus {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	status := &NodeStatus{}

	// node status
	var requestedMilestone milestone.Index
	peekedRequest := gossip.RequestQueue().Peek()
	queued, pending, processing := gossip.RequestQueue().Size()
	if peekedRequest != nil {
		requestedMilestone = peekedRequest.MilestoneIndex
	}
	status.Version = cli.AppVersion
	status.LatestVersion = cli.LatestGithubVersion
	status.Uptime = time.Since(nodeStartAt).Milliseconds()
	status.IsHealthy = tanglecore.IsNodeHealthy()
	status.NodeAlias = config.NodeConfig.String(config.CfgNodeAlias)

	status.ConnectedPeersCount = p2pcore.Manager().ConnectedCount()

	snapshotInfo := database.Tangle().GetSnapshotInfo()
	if snapshotInfo != nil {
		status.SnapshotIndex = snapshotInfo.SnapshotIndex
		status.PruningIndex = snapshotInfo.PruningIndex
	}
	status.CurrentRequestedMs = requestedMilestone
	status.RequestQueueQueued = queued
	status.RequestQueuePending = pending
	status.RequestQueueProcessing = processing
	status.RequestQueueAvgLatency = gossip.RequestQueue().AvgLatency()

	// cache metrics
	status.Caches = &CachesMetric{
		Children: Cache{
			Size: database.Tangle().GetChildrenStorageSize(),
		},
		RequestQueue: Cache{
			Size: queued + pending,
		},
		Milestones: Cache{
			Size: database.Tangle().GetMilestoneStorageSize(),
		},
		Messages: Cache{
			Size: database.Tangle().GetMessageStorageSize(),
		},
		IncomingMessageWorkUnits: Cache{
			Size: gossip.MessageProcessor().WorkUnitsSize(),
		},
	}

	// server metrics
	status.ServerMetrics = &ServerMetrics{
		AllMessages:          metrics.SharedServerMetrics.Messages.Load(),
		NewMessages:          metrics.SharedServerMetrics.NewMessages.Load(),
		KnownMessages:        metrics.SharedServerMetrics.KnownMessages.Load(),
		InvalidMessages:      metrics.SharedServerMetrics.InvalidMessages.Load(),
		InvalidRequests:      metrics.SharedServerMetrics.InvalidRequests.Load(),
		ReceivedMessageReq:   metrics.SharedServerMetrics.ReceivedMessageRequests.Load(),
		ReceivedMilestoneReq: metrics.SharedServerMetrics.ReceivedMilestoneRequests.Load(),
		ReceivedHeartbeats:   metrics.SharedServerMetrics.ReceivedHeartbeats.Load(),
		SentMessages:         metrics.SharedServerMetrics.SentMessages.Load(),
		SentMessageReq:       metrics.SharedServerMetrics.SentMessageRequests.Load(),
		SentMilestoneReq:     metrics.SharedServerMetrics.SentMilestoneRequests.Load(),
		SentHeartbeats:       metrics.SharedServerMetrics.SentHeartbeats.Load(),
		DroppedSentPackets:   metrics.SharedServerMetrics.DroppedMessages.Load(),
		SentSpamMsgsCount:    metrics.SharedServerMetrics.SentSpamMessages.Load(),
		ValidatedMessages:    metrics.SharedServerMetrics.ValidatedMessages.Load(),
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
