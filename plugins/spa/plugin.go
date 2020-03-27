package spa

import (
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gohornet/hornet/packages/basicauth"
	"github.com/gohornet/hornet/packages/peering/peer"
	"github.com/gohornet/hornet/packages/protocol/sting"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/peering"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/websockethub"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/metrics"
	"github.com/gohornet/hornet/packages/model/milestone"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/autopeering"
	"github.com/gohornet/hornet/plugins/cli"
	metrics_plugin "github.com/gohornet/hornet/plugins/metrics"
	tangle_plugin "github.com/gohornet/hornet/plugins/tangle"
)

var (
	PLUGIN = node.NewPlugin("SPA", node.Enabled, configure, run)
	log    *logger.Logger

	nodeStartAt = time.Now()

	clientsMu    sync.Mutex
	clients             = make(map[uint64]chan interface{}, 0)
	nextClientID uint64 = 0

	wsSendWorkerPool      *workerpool.WorkerPool
	webSocketWriteTimeout = time.Duration(3) * time.Second

	hub      *websockethub.Hub
	upgrader *websocket.Upgrader

	cachedMilestoneMetrics []*tangle_plugin.ConfirmedMilestoneMetric
)

const (
	BROADCAST_QUEUE_SIZE  = 1000
	wsSendWorkerCount     = 1
	wsSendWorkerQueueSize = 250
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	upgrader = &websocket.Upgrader{
		HandshakeTimeout:  webSocketWriteTimeout,
		EnableCompression: true,
	}

	// allow any origin for websocket connections
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	hub = websockethub.NewHub(log, upgrader, BROADCAST_QUEUE_SIZE)

	wsSendWorkerPool = workerpool.New(func(task workerpool.Task) {
		switch x := task.Param(0).(type) {
		case *metrics_plugin.TPSMetrics:
			hub.BroadcastMsg(&msg{MsgTypeTPSMetric, x})
			hub.BroadcastMsg(&msg{MsgTypeNodeStatus, currentNodeStatus()})
			hub.BroadcastMsg(&msg{MsgTypePeerMetric, peerMetrics()})
		case *tangle.Bundle:
			hub.BroadcastMsg(&msg{MsgTypeNodeStatus, currentNodeStatus()})
		case []*tangle_plugin.ConfirmedMilestoneMetric:
			hub.BroadcastMsg(&msg{MsgTypeConfirmedMsMetrics, x})
		}
		task.Return(nil)
	}, workerpool.WorkerCount(wsSendWorkerCount), workerpool.QueueSize(wsSendWorkerQueueSize))

	configureTipSelMetric()
	configureLiveFeed()
}

func run(plugin *node.Plugin) {

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	if config.NodeConfig.GetBool(config.CfgDashboardBasicAuthEnabled) {
		// grab auth info
		expectedUsername := config.NodeConfig.GetString(config.CfgDashboardBasicAuthUsername)
		expectedPasswordHash := config.NodeConfig.GetString(config.CfgDashboardBasicAuthPasswordHash)
		passwordSalt := config.NodeConfig.GetString(config.CfgDashboardBasicAuthPasswordSalt)

		if len(expectedUsername) == 0 {
			log.Fatalf("'%s' must not be empty if dashboard basic auth is enabled", config.CfgDashboardBasicAuthUsername)
		}

		if len(expectedPasswordHash) != 64 {
			log.Fatalf("'%s' must be 64 (sha256 hash) in length if dashboard basic auth is enabled", config.CfgDashboardBasicAuthPasswordHash)
		}

		e.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			if username == expectedUsername &&
				basicauth.VerifyPassword(password, passwordSalt, expectedPasswordHash) {
				return true, nil
			}
			return false, nil
		}))
	}

	setupRoutes(e)
	bindAddr := config.NodeConfig.GetString(config.CfgDashboardBindAddress)
	log.Infof("You can now access the dashboard using: http://%s", bindAddr)
	go e.Start(bindAddr)

	notifyStatus := events.NewClosure(func(tpsMetrics *metrics_plugin.TPSMetrics) {
		wsSendWorkerPool.TrySubmit(tpsMetrics)
	})

	notifyNewMs := events.NewClosure(func(cachedBndl *tangle.CachedBundle) {
		wsSendWorkerPool.TrySubmit(cachedBndl.GetBundle())
		cachedBndl.Release(true) // bundle -1
	})

	notifyConfirmedMsMetrics := events.NewClosure(func(metric *tangle_plugin.ConfirmedMilestoneMetric) {
		cachedMilestoneMetrics = append(cachedMilestoneMetrics, metric)
		if len(cachedMilestoneMetrics) > 20 {
			cachedMilestoneMetrics = cachedMilestoneMetrics[len(cachedMilestoneMetrics)-20:]
		}
		wsSendWorkerPool.TrySubmit([]*tangle_plugin.ConfirmedMilestoneMetric{metric})
	})

	daemon.BackgroundWorker("SPA[WSSend]", func(shutdownSignal <-chan struct{}) {
		go hub.Run(shutdownSignal)
		metrics_plugin.Events.TPSMetricsUpdated.Attach(notifyStatus)
		tangle_plugin.Events.SolidMilestoneChanged.Attach(notifyNewMs)
		tangle_plugin.Events.LatestMilestoneChanged.Attach(notifyNewMs)
		tangle_plugin.Events.NewConfirmedMilestoneMetric.Attach(notifyConfirmedMsMetrics)
		wsSendWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping SPA[WSSend] ...")
		metrics_plugin.Events.TPSMetricsUpdated.Detach(notifyStatus)
		tangle_plugin.Events.SolidMilestoneChanged.Detach(notifyNewMs)
		tangle_plugin.Events.LatestMilestoneChanged.Detach(notifyNewMs)
		tangle_plugin.Events.NewConfirmedMilestoneMetric.Detach(notifyConfirmedMsMetrics)
		wsSendWorkerPool.StopAndWait()
		log.Info("Stopping SPA[WSSend] ... done")
	}, shutdown.ShutdownPrioritySPA)

	runLiveFeed()
	runTipSelMetricWorker()
}

// tx +1
func getMilestoneTail(index milestone.Index) *tangle.CachedTransaction {
	cachedMs := tangle.GetMilestoneOrNil(index) // bundle +1
	if cachedMs == nil {
		return nil
	}

	defer cachedMs.Release(true) // bundle -1

	return cachedMs.GetBundle().GetTail() // tx +1
}

const (
	MsgTypeNodeStatus byte = iota
	MsgTypeTPSMetric
	MsgTypeTipSelMetric
	MsgTypeTx
	MsgTypeMs
	MsgTypePeerMetric
	MsgTypeConfirmedMsMetrics
)

type msg struct {
	Type byte        `json:"type"`
	Data interface{} `json:"data"`
}

type tx struct {
	Hash  string `json:"hash"`
	Value int64  `json:"value"`
}

type ms struct {
	Hash  string          `json:"hash"`
	Index milestone.Index `json:"index"`
}

type nodestatus struct {
	LSMI                   milestone.Index `json:"lsmi"`
	LMI                    milestone.Index `json:"lmi"`
	SnapshotIndex          milestone.Index `json:"snapshot_index"`
	PruningIndex           milestone.Index `json:"pruning_index"`
	Version                string          `json:"version"`
	LatestVersion          string          `json:"latest_version"`
	Uptime                 int64           `json:"uptime"`
	AutopeeringID          string          `json:"autopeering_id"`
	NodeAlias              string          `json:"node_alias"`
	ConnectedPeersCount    int             `json:"connected_peers_count"`
	CurrentRequestedMs     milestone.Index `json:"current_requested_ms"`
	RequestQueueQueued     int             `json:"request_queue_queued"`
	RequestQueuePending    int             `json:"request_queue_pending"`
	RequestQueueAvgLatency int64           `json:"request_queue_avg_latency"`
	ServerMetrics          *servermetrics  `json:"server_metrics"`
	Mem                    *memmetrics     `json:"mem"`
	Caches                 *cachesmetric   `json:"caches"`
}

type servermetrics struct {
	NumberOfAllTransactions        uint64 `json:"all_txs"`
	NumberOfNewTransactions        uint64 `json:"new_txs"`
	NumberOfKnownTransactions      uint64 `json:"known_txs"`
	NumberOfInvalidTransactions    uint64 `json:"invalid_txs"`
	NumberOfInvalidRequests        uint64 `json:"invalid_req"`
	NumberOfStaleTransactions      uint64 `json:"stale_txs"`
	NumberOfReceivedTransactionReq uint64 `json:"rec_tx_req"`
	NumberOfReceivedMilestoneReq   uint64 `json:"rec_ms_req"`
	NumberOfReceivedHeartbeats     uint64 `json:"rec_heartbeat"`
	NumberOfSentTransactions       uint64 `json:"sent_txs"`
	NumberOfSentTransactionsReq    uint64 `json:"sent_tx_req"`
	NumberOfSentMilestoneReq       uint64 `json:"sent_ms_req"`
	NumberOfSentHeartbeats         uint64 `json:"sent_heartbeat"`
	NumberOfDroppedSentPackets     uint64 `json:"dropped_sent_packets"`
	NumberOfSentSpamTxsCount       uint64 `json:"sent_spam_txs"`
	NumberOfValidatedBundles       uint64 `json:"validated_bundles"`
	NumberOfSeenSpentAddr          uint64 `json:"spent_addr"`
}

type memmetrics struct {
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

type peermetric struct {
	Identity         string                `json:"identity"`
	Alias            string                `json:"alias,omitempty"`
	OriginAdrr       string                `json:"origin_addr"`
	ConnectionOrigin peer.ConnectionOrigin `json:"connection_origin"`
	ProtocolVersion  byte                  `json:"protocol_version"`
	BytesRead        int                   `json:"bytes_read"`
	BytesWritten     int                   `json:"bytes_written"`
	Heartbeat        *sting.Heartbeat      `json:"heartbeat"`
	Info             *peer.Info            `json:"info"`
	Connected        bool                  `json:"connected"`
}

type cachesmetric struct {
	RequestQueue                 cache `json:"request_queue"`
	Approvers                    cache `json:"approvers"`
	Bundles                      cache `json:"bundles"`
	Milestones                   cache `json:"milestones"`
	SpentAddresses               cache `json:"spent_addresses"`
	Transactions                 cache `json:"transactions"`
	IncomingTransactionWorkUnits cache `json:"incoming_transaction_work_units"`
	RefsInvalidBundle            cache `json:"refs_invalid_bundle"`
}

type cache struct {
	Size int `json:"size"`
}

func peerMetrics() []*peermetric {
	infos := peering.Manager().PeerInfos()
	var stats []*peermetric
	for _, info := range infos {
		m := &peermetric{
			OriginAdrr: info.DomainWithPort,
			Info:       info,
		}
		if info.Peer != nil && info.Peer.Protocol != nil {
			m.Identity = info.Peer.ID
			m.Alias = info.Alias
			m.ConnectionOrigin = info.Peer.ConnectionOrigin
			m.ProtocolVersion = info.Peer.Protocol.FeatureSet
			m.BytesRead = info.Peer.Conn.BytesRead
			m.BytesWritten = info.Peer.Conn.BytesWritten
			m.Heartbeat = info.Peer.LatestHeartbeat
			m.Connected = info.Connected
		} else {
			m.Identity = info.Address
		}
		stats = append(stats, m)
	}
	return stats
}

func currentNodeStatus() *nodestatus {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	status := &nodestatus{}

	// node status
	var requestedMilestone milestone.Index
	peekedRequest := gossip.RequestQueue().Peek()
	queued, pending := gossip.RequestQueue().Size()
	if peekedRequest != nil {
		requestedMilestone = peekedRequest.MilestoneIndex
	}
	status.Version = cli.AppVersion
	status.LatestVersion = cli.LatestGithubVersion
	status.Uptime = time.Since(nodeStartAt).Milliseconds()
	if !node.IsSkipped(autopeering.PLUGIN) {
		status.AutopeeringID = autopeering.ID
	}
	status.NodeAlias = config.NodeConfig.GetString(config.CfgNodeAlias)
	status.LSMI = tangle.GetSolidMilestoneIndex()
	status.LMI = tangle.GetLatestMilestoneIndex()

	status.ConnectedPeersCount = peering.Manager().ConnectedPeerCount()

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		status.SnapshotIndex = snapshotInfo.SnapshotIndex
		status.PruningIndex = snapshotInfo.PruningIndex
	}
	status.CurrentRequestedMs = requestedMilestone
	status.RequestQueuePending = pending
	status.RequestQueueQueued = queued
	status.RequestQueueAvgLatency = gossip.RequestQueue().AvgLatency()

	// cache metrics
	status.Caches = &cachesmetric{
		Approvers: cache{
			Size: tangle.GetApproversStorageSize(),
		},
		RequestQueue: cache{
			Size: queued + pending,
		},
		Bundles: cache{
			Size: tangle.GetBundleStorageSize(),
		},
		Milestones: cache{
			Size: tangle.GetMilestoneStorageSize(),
		},
		Transactions: cache{
			Size: tangle.GetTransactionStorageSize(),
		},
		IncomingTransactionWorkUnits: cache{
			Size: gossip.Processor().WorkUnitsSize(),
		},
		RefsInvalidBundle: cache{
			Size: tangle_plugin.GetRefsAnInvalidBundleStorageSize(),
		},
	}

	// server metrics
	status.ServerMetrics = &servermetrics{
		NumberOfAllTransactions:        metrics.SharedServerMetrics.Transactions.Load(),
		NumberOfNewTransactions:        metrics.SharedServerMetrics.NewTransactions.Load(),
		NumberOfKnownTransactions:      metrics.SharedServerMetrics.KnownTransactions.Load(),
		NumberOfInvalidTransactions:    metrics.SharedServerMetrics.InvalidTransactions.Load(),
		NumberOfInvalidRequests:        metrics.SharedServerMetrics.InvalidRequests.Load(),
		NumberOfStaleTransactions:      metrics.SharedServerMetrics.StaleTransactions.Load(),
		NumberOfReceivedTransactionReq: metrics.SharedServerMetrics.ReceivedTransactionRequests.Load(),
		NumberOfReceivedMilestoneReq:   metrics.SharedServerMetrics.ReceivedMilestoneRequests.Load(),
		NumberOfReceivedHeartbeats:     metrics.SharedServerMetrics.ReceivedHeartbeats.Load(),
		NumberOfSentTransactions:       metrics.SharedServerMetrics.SentTransactions.Load(),
		NumberOfSentTransactionsReq:    metrics.SharedServerMetrics.SentTransactionRequests.Load(),
		NumberOfSentMilestoneReq:       metrics.SharedServerMetrics.SentMilestoneRequests.Load(),
		NumberOfSentHeartbeats:         metrics.SharedServerMetrics.SentHeartbeats.Load(),
		NumberOfDroppedSentPackets:     metrics.SharedServerMetrics.DroppedMessages.Load(),
		NumberOfSentSpamTxsCount:       metrics.SharedServerMetrics.SentSpamTransactions.Load(),
		NumberOfValidatedBundles:       metrics.SharedServerMetrics.ValidatedBundles.Load(),
		NumberOfSeenSpentAddr:          metrics.SharedServerMetrics.SeenSpentAddresses.Load(),
	}

	// memory metrics
	status.Mem = &memmetrics{
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
