package prometheus

import (
	"strconv"

	"github.com/gohornet/hornet/core/app"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	infoApp                 *prometheus.GaugeVec
	infoMilestone           *prometheus.GaugeVec
	infoMilestoneIndex      prometheus.Gauge
	infoSolidMilestone      *prometheus.GaugeVec
	infoSolidMilestoneIndex prometheus.Gauge
	infoSnapshotIndex       prometheus.Gauge
	infoPruningIndex        prometheus.Gauge
	infoTips                prometheus.Gauge
	infoMessagesToRequest   prometheus.Gauge
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
		[]string{"messageID", "index"},
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
		[]string{"messageID", "index"},
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
	infoMessagesToRequest = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_info_messages_to_request",
		Help: "Number of messages to request.",
	})

	infoApp.WithLabelValues(app.Name, app.Version).Set(1)

	registry.MustRegister(infoApp)
	registry.MustRegister(infoMilestone)
	registry.MustRegister(infoMilestoneIndex)
	registry.MustRegister(infoSolidMilestone)
	registry.MustRegister(infoSolidMilestoneIndex)
	registry.MustRegister(infoSnapshotIndex)
	registry.MustRegister(infoPruningIndex)
	registry.MustRegister(infoTips)
	registry.MustRegister(infoMessagesToRequest)

	addCollect(collectInfo)
}

func collectInfo() {
	// Latest milestone index
	lmi := deps.Tangle.GetLatestMilestoneIndex()
	infoMilestoneIndex.Set(float64(lmi))
	infoMilestone.Reset()
	infoMilestone.WithLabelValues(hornet.GetNullMessageID().Hex(), strconv.Itoa(int(lmi))).Set(1)

	// Latest milestone message ID
	cachedLatestMilestone := deps.Tangle.GetCachedMilestoneOrNil(lmi)
	if cachedLatestMilestone != nil {
		infoMilestone.Reset()
		infoMilestone.WithLabelValues(cachedLatestMilestone.GetMilestone().MessageID.Hex(), strconv.Itoa(int(lmi))).Set(1)
		cachedLatestMilestone.Release(true)
	}

	// Solid milestone index
	smi := deps.Tangle.GetSolidMilestoneIndex()
	infoSolidMilestoneIndex.Set(float64(smi))
	infoSolidMilestone.Reset()
	infoSolidMilestone.WithLabelValues(hornet.GetNullMessageID().Hex(), strconv.Itoa(int(smi))).Set(1)

	// Solid milestone message ID
	cachedSolidMilestone := deps.Tangle.GetCachedMilestoneOrNil(smi)
	if cachedSolidMilestone != nil {
		infoSolidMilestone.Reset()
		infoSolidMilestone.WithLabelValues(cachedSolidMilestone.GetMilestone().MessageID.Hex(), strconv.Itoa(int(smi))).Set(1)
		cachedSolidMilestone.Release(true)
	}

	// Snapshot index and Pruning index
	snapshotInfo := deps.Tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		infoSnapshotIndex.Set(float64(snapshotInfo.SnapshotIndex))
		infoPruningIndex.Set(float64(snapshotInfo.PruningIndex))
	}

	// Tips
	infoTips.Set(0)

	// Messages to request
	queued, pending, _ := deps.RequestQueue.Size()
	infoMessagesToRequest.Set(float64(queued + pending))
}
