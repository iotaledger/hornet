package spa

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/parameter"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/cli"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/gossip/server"
	"github.com/gohornet/hornet/plugins/metrics"
	tangle_plugin "github.com/gohornet/hornet/plugins/tangle"
)

var (
	PLUGIN = node.NewPlugin("SPA", node.Enabled, configure, run)
	log    *logger.Logger

	nodeStartAt = time.Now()

	clientsMu    sync.Mutex
	clients             = make(map[uint64]chan interface{}, 0)
	nextClientID uint64 = 0

	wsSendWorkerCount     = 1
	wsSendWorkerQueueSize = 250
	wsSendWorkerPool      *workerpool.WorkerPool
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	wsSendWorkerPool = workerpool.New(func(task workerpool.Task) {
		switch x := task.Param(0).(type) {
		case *metrics.TPSMetrics:
			sendToAllWSClient(&msg{MsgTypeTPSMetric, x})
			sendToAllWSClient(&msg{MsgTypeNodeStatus, currentNodeStatus()})
			sendToAllWSClient(&msg{MsgTypeNeighborMetric, neighborMetrics()})
		case *tangle.Bundle:
			sendToAllWSClient(&msg{MsgTypeNodeStatus, currentNodeStatus()})
		}
		task.Return(nil)
	}, workerpool.WorkerCount(wsSendWorkerCount), workerpool.QueueSize(wsSendWorkerQueueSize))

	configureTipSelMetric()
	configureLiveFeed()
}

func run(plugin *node.Plugin) {

	notifyStatus := events.NewClosure(func(tpsMetrics *metrics.TPSMetrics) {
		wsSendWorkerPool.TrySubmit(tpsMetrics)
	})

	notifyNewMs := events.NewClosure(func(bndl *tangle.Bundle) {
		wsSendWorkerPool.TrySubmit(bndl)
	})

	daemon.BackgroundWorker("SPA[WSSend]", func(shutdownSignal <-chan struct{}) {
		metrics.Events.TPSMetricsUpdated.Attach(notifyStatus)
		tangle_plugin.Events.SolidMilestoneChanged.Attach(notifyNewMs)
		tangle_plugin.Events.LatestMilestoneChanged.Attach(notifyNewMs)
		wsSendWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping SPA[WSSend] ...")
		metrics.Events.TPSMetricsUpdated.Detach(notifyStatus)
		tangle_plugin.Events.SolidMilestoneChanged.Detach(notifyNewMs)
		tangle_plugin.Events.LatestMilestoneChanged.Detach(notifyNewMs)
		wsSendWorkerPool.Stop()
		log.Info("Stopping SPA[WSSend] ... done")
	}, shutdown.ShutdownPrioritySPA)

	runLiveFeed()
	runTipSelMetricWorker()

	// allow any origin for websocket connections
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	if parameter.NodeConfig.GetBool("dashboard.basic_auth.enabled") {
		e.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			if username == parameter.NodeConfig.GetString("dashboard.basic_auth.username") &&
				password == parameter.NodeConfig.GetString("dashboard.basic_auth.password") {
				return true, nil
			}
			return false, nil
		}))
	}

	setupRoutes(e)
	addr := parameter.NodeConfig.GetString("dashboard.bindAddress")
	port := parameter.NodeConfig.GetInt("dashboard.port")

	log.Infof("You can now access the dashboard using: http://%s:%d", addr, port)
	go e.Start(fmt.Sprintf("%s:%d", addr, port))
}

// sends the given message to all connected websocket clients
func sendToAllWSClient(msg interface{}) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for _, channel := range clients {
		select {
		case channel <- msg:
		default:
			// drop if buffer not drained
		}
	}
}

var webSocketWriteTimeout = time.Duration(3) * time.Second

var (
	upgrader = websocket.Upgrader{
		HandshakeTimeout:  webSocketWriteTimeout,
		EnableCompression: true,
	}
)

func getMilestone(index milestone_index.MilestoneIndex) *tangle.CachedTransaction {
	msBndl := tangle.GetMilestone(index)
	if msBndl == nil {
		return nil
	}

	tail := msBndl.GetTail() //+1
	if !tail.Exists() {
		tail.Release() //-1
		return nil
	}

	return tail
}

func preFeed(channel chan interface{}) {
	channel <- &msg{MsgTypeNodeStatus, currentNodeStatus()}
	start := tangle.GetLatestMilestoneIndex()
	for i := start - 10; i <= start; i++ {
		if tailTx := getMilestone(i); tailTx != nil { //+1
			channel <- &msg{MsgTypeMs, &ms{tailTx.GetTransaction().GetHash(), i}}
			tailTx.Release() //-1
		} else {
			break
		}
	}
}

const (
	MsgTypeNodeStatus byte = iota
	MsgTypeTPSMetric
	MsgTypeTipSelMetric
	MsgTypeTx
	MsgTypeMs
	MsgTypeNeighborMetric
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
	Hash  string                         `json:"hash"`
	Index milestone_index.MilestoneIndex `json:"index"`
}

type nodestatus struct {
	LSMI               milestone_index.MilestoneIndex `json:"lsmi"`
	LMI                milestone_index.MilestoneIndex `json:"lmi"`
	SnapshotIndex      milestone_index.MilestoneIndex `json:"snapshot_index"`
	PruningIndex       milestone_index.MilestoneIndex `json:"pruning_index"`
	Version            string                         `json:"version"`
	LatestVersion      string                         `json:"latest_version"`
	Uptime             int64                          `json:"uptime"`
	CurrentRequestedMs milestone_index.MilestoneIndex `json:"current_requested_ms"`
	MsRequestQueueSize int                            `json:"ms_request_queue_size"`
	RequestQueueSize   int                            `json:"request_queue_size"`
	ServerMetrics      *servermetrics                 `json:"server_metrics"`
	Mem                *memmetrics                    `json:"mem"`
	Caches             *cachesmetric                  `json:"caches"`
}

type servermetrics struct {
	AllTxs             uint32 `json:"all_txs"`
	InvalidTxs         uint32 `json:"invalid_txs"`
	StaleTxs           uint32 `json:"stale_txs"`
	RandomTxs          uint32 `json:"random_txs"`
	SentTxs            uint32 `json:"sent_txs"`
	RecMsReq           uint32 `json:"rec_ms_req"`
	SentMsReq          uint32 `json:"sent_ms_req"`
	NewTxs             uint32 `json:"new_txs"`
	DroppedSentPackets uint32 `json:"dropped_sent_packets"`
	RecTxReq           uint32 `json:"rec_tx_req"`
	SentTxReq          uint32 `json:"sent_tx_req"`
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

type neighbormetric struct {
	Identity         string                  `json:"identity"`
	Alias            string                  `json:"alias" omitempty`
	OriginAdrr       string                  `json:"origin_addr"`
	ConnectionOrigin gossip.ConnectionOrigin `json:"connection_origin"`
	ProtocolVersion  byte                    `json:"protocol_version"`
	BytesRead        int                     `json:"bytes_read"`
	BytesWritten     int                     `json:"bytes_written"`
	Heartbeat        *gossip.Heartbeat       `json:"heartbeat"`
	Info             gossip.NeighborInfo     `json:"info"`
	Connected        bool                    `json:"connected"`
}

type cachesmetric struct {
	RequestQueue              cache `json:"request_queue"`
	Approvers                 cache `json:"approvers"`
	Bundles                   cache `json:"bundles"`
	Milestones                cache `json:"milestones"`
	SpentAddresses            cache `json:"spent_addresses"`
	Transactions              cache `json:"transactions"`
	IncomingTransactionFilter cache `json:"incoming_transaction_filter"`
	RefsInvalidBundle         cache `json:"refs_invalid_bundle"`
}

type cache struct {
	Size     int `json:"size"`
	Capacity int `json:"capacity"`
}

func neighborMetrics() []*neighbormetric {
	infos := gossip.GetNeighbors()
	stats := []*neighbormetric{}
	for _, info := range infos {
		m := &neighbormetric{
			OriginAdrr: info.DomainWithPort,
			Info:       info,
		}
		if info.Neighbor != nil {
			m.Identity = info.Neighbor.Identity
			m.Alias = info.Alias
			m.ConnectionOrigin = info.Neighbor.ConnectionOrigin
			m.ProtocolVersion = info.Neighbor.Protocol.Version
			m.BytesRead = info.Neighbor.Protocol.Conn.BytesRead
			m.BytesWritten = info.Neighbor.Protocol.Conn.BytesWritten
			m.Heartbeat = info.Neighbor.LatestHeartbeat
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
	requestedMilestone, requestCount := gossip.RequestQueue.CurrentMilestoneIndexAndSize()
	status.Version = cli.AppVersion
	status.LatestVersion = cli.LatestGithubVersion
	status.Uptime = time.Since(nodeStartAt).Milliseconds()
	status.LSMI = tangle.GetSolidMilestoneIndex()
	status.LMI = tangle.GetLatestMilestoneIndex()

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		status.SnapshotIndex = snapshotInfo.SnapshotIndex
		status.PruningIndex = snapshotInfo.PruningIndex
	}
	status.MsRequestQueueSize = requestCount
	status.CurrentRequestedMs = requestedMilestone
	status.RequestQueueSize = requestCount

	// cache metrics
	status.Caches = &cachesmetric{
		Approvers: cache{
			Size:     tangle.GetApproversStorageSize(),
			Capacity: 0,
		},
		RequestQueue: cache{
			Size:     gossip.RequestQueue.GetStorageSize(),
			Capacity: 0,
		},
		Bundles: cache{
			Size:     tangle.BundleBucketCache.GetSize(),
			Capacity: tangle.BundleBucketCache.GetCapacity(),
		},
		Milestones: cache{
			Size:     tangle.GetMilestoneStorageSize(),
			Capacity: 0,
		},
		Transactions: cache{
			Size:     tangle.GetTransactionStorageSize(),
			Capacity: 0,
		},
		IncomingTransactionFilter: cache{
			Size:     gossip.GetIncomingStorageSize(),
			Capacity: 0,
		},
		RefsInvalidBundle: cache{
			Size:     tangle_plugin.GetRefsAnInvalidBundleStorageSize(),
			Capacity: 0,
		},
	}

	// server metrics
	status.ServerMetrics = &servermetrics{
		AllTxs:             server.SharedServerMetrics.GetAllTransactionsCount(),
		InvalidTxs:         server.SharedServerMetrics.GetInvalidTransactionsCount(),
		StaleTxs:           server.SharedServerMetrics.GetStaleTransactionsCount(),
		RandomTxs:          server.SharedServerMetrics.GetRandomTransactionRequestsCount(),
		SentTxs:            server.SharedServerMetrics.GetSentTransactionsCount(),
		NewTxs:             server.SharedServerMetrics.GetNewTransactionsCount(),
		DroppedSentPackets: server.SharedServerMetrics.GetDroppedSendPacketsCount(),
		RecMsReq:           server.SharedServerMetrics.GetReceivedMilestoneRequestsCount(),
		SentMsReq:          server.SharedServerMetrics.GetSentMilestoneRequestsCount(),
		RecTxReq:           server.SharedServerMetrics.GetReceivedTransactionRequestCount(),
		SentTxReq:          server.SharedServerMetrics.GetSentTransactionRequestCount(),
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
