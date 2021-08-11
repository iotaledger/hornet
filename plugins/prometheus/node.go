package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	appInfo                     *prometheus.GaugeVec
	health                      prometheus.Gauge
	messagesPerSecond           prometheus.Gauge
	referencedMessagesPerSecond prometheus.Gauge
	referencedRate              prometheus.Gauge
	milestones                  *prometheus.GaugeVec
	tips                        *prometheus.GaugeVec
	requests                    *prometheus.GaugeVec
)

func configureNode() {

	appInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "node",
			Name:      "app_info",
			Help:      "Node software name and version.",
		},
		[]string{"name", "version"},
	)

	health = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "node",
			Name:      "health",
			Help:      "Health of the node.",
		})

	messagesPerSecond = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "node",
			Name:      "messages_per_second",
			Help:      "Current rate of new messages per second.",
		})

	referencedMessagesPerSecond = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "node",
			Name:      "referenced_messages_per_second",
			Help:      "Current rate of referenced messages per second.",
		})

	referencedRate = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "node",
			Name:      "referenced_rate",
			Help:      "Ratio of referenced messages in relation to new messages of the last confirmed milestone.",
		})

	milestones = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "node",
			Name:      "milestones",
			Help:      "Infos about milestone indexes.",
		},
		[]string{"type"},
	)

	if deps.TipSelector != nil {
		tips = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "iota",
				Subsystem: "node",
				Name:      "tip_count",
				Help:      "Number of tips.",
			}, []string{"type"},
		)
	}

	requests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "node",
			Name:      "request_count",
			Help:      "Number of messages to request.",
		}, []string{"type"},
	)

	appInfo.WithLabelValues(deps.AppInfo.Name, deps.AppInfo.Version).Set(1)

	registry.MustRegister(appInfo)
	registry.MustRegister(health)
	registry.MustRegister(messagesPerSecond)
	registry.MustRegister(referencedMessagesPerSecond)
	registry.MustRegister(referencedRate)
	registry.MustRegister(milestones)

	if deps.TipSelector != nil {
		registry.MustRegister(tips)
	}

	registry.MustRegister(requests)

	addCollect(collectInfo)
}

func collectInfo() {
	health.Set(0)
	if deps.Tangle.IsNodeHealthy() {
		health.Set(1)
	}

	messagesPerSecond.Set(0)
	referencedMessagesPerSecond.Set(0)
	referencedRate.Set(0)

	lastConfirmedMilestoneMetric := deps.Tangle.LastConfirmedMilestoneMetric()
	if lastConfirmedMilestoneMetric != nil {
		messagesPerSecond.Set(lastConfirmedMilestoneMetric.MPS)
		referencedMessagesPerSecond.Set(lastConfirmedMilestoneMetric.RMPS)
		referencedRate.Set(lastConfirmedMilestoneMetric.ReferencedRate)
	}

	milestones.WithLabelValues("latest").Set(float64(deps.Storage.LatestMilestoneIndex()))
	milestones.WithLabelValues("confirmed").Set(float64(deps.Storage.ConfirmedMilestoneIndex()))

	snapshotInfo := deps.Storage.SnapshotInfo()
	milestones.WithLabelValues("snapshot").Set(0)
	milestones.WithLabelValues("pruning").Set(0)
	if snapshotInfo != nil {
		milestones.WithLabelValues("snapshot").Set(float64(snapshotInfo.SnapshotIndex))
		milestones.WithLabelValues("pruning").Set(float64(snapshotInfo.PruningIndex))
	}

	if deps.TipSelector != nil {
		nonLazyTipCount, semiLazyTipCount := deps.TipSelector.TipCount()
		tips.WithLabelValues("nonlazy").Set(float64(nonLazyTipCount))
		tips.WithLabelValues("semilazy").Set(float64(semiLazyTipCount))
	}

	queued, pending, processing := deps.RequestQueue.Size()
	requests.WithLabelValues("queued").Set(float64(queued))
	requests.WithLabelValues("pending").Set(float64(pending))
	requests.WithLabelValues("processing").Set(float64(processing))
}
