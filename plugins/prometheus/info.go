package prometheus

import (
	"strconv"

	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/plugins/cli"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/iotaledger/iota.go/consts"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	infoApp                   *prometheus.GaugeVec
	infoMilestone             *prometheus.GaugeVec
	infoMilestoneIndex        prometheus.Gauge
	infoSolidMilestone        *prometheus.GaugeVec
	infoSolidMilestoneIndex   prometheus.Gauge
	infoSnapshotIndex         prometheus.Gauge
	infoPruningIndex          prometheus.Gauge
	infoTips                  prometheus.Gauge
	infoTransactionsToRequest prometheus.Gauge
)

func init() {
	infoApp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_info_app",
			Help: "Node software name and version.",
		},
		[]string{"name", "version"},
	)
	infoMilestone = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_info_latest_milestone",
			Help: "Latest milestone.",
		},
		[]string{"hash", "index"},
	)
	infoMilestoneIndex = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_info_latest_milestone_index",
		Help: "Latest milestone index.",
	})
	infoSolidMilestone = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_info_latest_solid_milestone",
			Help: "Latest solid milestone.",
		},
		[]string{"hash", "index"},
	)
	infoSolidMilestoneIndex = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_info_latest_solid_milestone_index",
		Help: "Latest solid milestone index.",
	})
	infoSnapshotIndex = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_info_snapshot_index",
		Help: "Snapshot index.",
	})
	infoPruningIndex = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_info_pruning_index",
		Help: "Pruning index.",
	})
	infoTips = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_info_tips",
		Help: "Number of tips.",
	})
	infoTransactionsToRequest = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_info_transactions_to_request",
		Help: "Number of transactions to request.",
	})

	infoApp.WithLabelValues(cli.AppName, cli.AppVersion).Set(1)

	registry.MustRegister(infoApp)
	registry.MustRegister(infoMilestone)
	registry.MustRegister(infoMilestoneIndex)
	registry.MustRegister(infoSolidMilestone)
	registry.MustRegister(infoSolidMilestoneIndex)
	registry.MustRegister(infoSnapshotIndex)
	registry.MustRegister(infoPruningIndex)
	registry.MustRegister(infoTips)
	registry.MustRegister(infoTransactionsToRequest)

	addCollect(collectInfo)
}

func collectInfo() {
	// Latest milestone index
	lmi := tangle.GetLatestMilestoneIndex()
	infoMilestoneIndex.Set(float64(lmi))
	infoMilestone.Reset()
	infoMilestone.WithLabelValues(consts.NullHashTrytes, strconv.Itoa(int(lmi))).Set(1)

	// Latest milestone hash
	cachedLatestMs := tangle.GetMilestoneOrNil(lmi)
	if cachedLatestMs != nil {
		cachedMsTailTx := cachedLatestMs.GetBundle().GetTail()
		infoMilestone.Reset()
		infoMilestone.WithLabelValues(cachedMsTailTx.GetTransaction().Tx.Hash, strconv.Itoa(int(lmi))).Set(1)
		cachedMsTailTx.Release()
		cachedLatestMs.Release()
	}

	// Solid milestone index
	smi := tangle.GetSolidMilestoneIndex()
	infoSolidMilestoneIndex.Set(float64(smi))
	infoSolidMilestone.Reset()
	infoSolidMilestone.WithLabelValues(consts.NullHashTrytes, strconv.Itoa(int(smi))).Set(1)

	// Solid milestone hash
	cachedSolidMs := tangle.GetMilestoneOrNil(smi)
	if cachedSolidMs != nil {
		cachedMsTailTx := cachedSolidMs.GetBundle().GetTail()
		infoSolidMilestone.Reset()
		infoSolidMilestone.WithLabelValues(cachedMsTailTx.GetTransaction().Tx.Hash, strconv.Itoa(int(smi))).Set(1)
		cachedMsTailTx.Release()
		cachedSolidMs.Release()
	}

	// Snapshot index and Pruning index
	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		infoSnapshotIndex.Set(float64(snapshotInfo.SnapshotIndex))
		infoPruningIndex.Set(float64(snapshotInfo.PruningIndex))
	}

	// Tips
	infoTips.Set(0)

	// Transactions to request
	queued, pending, _ := gossip.RequestQueue().Size()
	infoTransactionsToRequest.Set(float64(queued + pending))
}
