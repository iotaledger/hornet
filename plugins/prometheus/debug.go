package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hornet/v2/pkg/pruning"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	"github.com/iotaledger/hornet/v2/pkg/whiteflag"
)

var (
	snapshotBuckets     = []float64{.1, .2, .5, 1, 2, 5, 10, 20, 50, 100, 200, 500}
	pruningBuckets      = []float64{.1, .2, .5, 1, 2, 5, 10, 20, 50, 100, 200, 500}
	confirmationBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 20, 50, 100}

	snapshotTotalDuration              prometheus.Histogram
	snapshotDurations                  *prometheus.GaugeVec
	databasePruningTotalDuration       prometheus.Histogram
	databasePruningDurations           *prometheus.GaugeVec
	milestoneConfirmationTotalDuration prometheus.Histogram
	milestoneConfirmationDurations     *prometheus.GaugeVec

	metricsLock                syncutils.RWMutex
	lastSnapshotMetrics        *snapshot.Metrics
	lastDatabasePruningMetrics *pruning.Metrics
	lastConfirmationMetrics    *whiteflag.ConfirmationMetrics
)

func configureDebug() {

	snapshotTotalDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iota",
			Subsystem: "debug",
			Name:      "snapshot_duration",
			Help:      "Total duration for snapshot creation [s].",
			Buckets:   snapshotBuckets,
		})

	snapshotDurations = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "debug",
			Name:      "snapshot_durations",
			Help:      "Debug durations for snapshot creation [s].",
		},
		[]string{"type"},
	)

	databasePruningTotalDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iota",
			Subsystem: "debug",
			Name:      "pruning_duration",
			Help:      "Total duration for database pruning [s].",
			Buckets:   pruningBuckets,
		})

	databasePruningDurations = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "debug",
			Name:      "pruning_durations",
			Help:      "Debug durations for database pruning [s].",
		},
		[]string{"type"},
	)

	milestoneConfirmationTotalDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iota",
			Subsystem: "debug",
			Name:      "confirmation_duration",
			Help:      "Total duration for milestone confirmation [s].",
			Buckets:   confirmationBuckets,
		})

	milestoneConfirmationDurations = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "debug",
			Name:      "confirmation_durations",
			Help:      "Debug durations for milestone confirmation [s].",
		},
		[]string{"type"},
	)

	deps.SnapshotManager.Events.SnapshotMetricsUpdated.Hook(events.NewClosure(func(metrics *snapshot.Metrics) {
		snapshotTotalDuration.Observe(metrics.DurationTotal.Seconds())
		metricsLock.Lock()
		defer metricsLock.Unlock()
		lastSnapshotMetrics = metrics
	}))

	deps.PruningManager.Events.PruningMetricsUpdated.Hook(events.NewClosure(func(metrics *pruning.Metrics) {
		databasePruningTotalDuration.Observe(metrics.DurationTotal.Seconds())
		metricsLock.Lock()
		defer metricsLock.Unlock()
		lastDatabasePruningMetrics = metrics
	}))

	deps.Tangle.Events.ConfirmationMetricsUpdated.Hook(events.NewClosure(func(metrics *whiteflag.ConfirmationMetrics) {
		milestoneConfirmationTotalDuration.Observe(metrics.DurationTotal.Seconds())
		metricsLock.Lock()
		defer metricsLock.Unlock()
		lastConfirmationMetrics = metrics
	}))

	registry.MustRegister(snapshotTotalDuration)
	registry.MustRegister(snapshotDurations)
	registry.MustRegister(databasePruningTotalDuration)
	registry.MustRegister(databasePruningDurations)
	registry.MustRegister(milestoneConfirmationTotalDuration)
	registry.MustRegister(milestoneConfirmationDurations)

	addCollect(collectDebug)
}

func collectDebug() {
	metricsLock.RLock()
	defer metricsLock.RUnlock()

	if lastSnapshotMetrics != nil {
		snapshotDurations.WithLabelValues("read_lock_ledger").Set(lastSnapshotMetrics.DurationReadLockLedger.Seconds())
		snapshotDurations.WithLabelValues("init").Set(lastSnapshotMetrics.DurationInit.Seconds())
		snapshotDurations.WithLabelValues("set_snapshot_info").Set(lastSnapshotMetrics.DurationSetSnapshotInfo.Seconds())
		snapshotDurations.WithLabelValues("snapshot_milestone_index_changed").Set(lastSnapshotMetrics.DurationSnapshotMilestoneIndexChanged.Seconds())
		snapshotDurations.WithLabelValues("header").Set(lastSnapshotMetrics.DurationHeader.Seconds())
		snapshotDurations.WithLabelValues("solid_entry_points").Set(lastSnapshotMetrics.DurationSolidEntryPoints.Seconds())
		snapshotDurations.WithLabelValues("outputs").Set(lastSnapshotMetrics.DurationOutputs.Seconds())
		snapshotDurations.WithLabelValues("milestone_diffs").Set(lastSnapshotMetrics.DurationMilestoneDiffs.Seconds())
		snapshotDurations.WithLabelValues("total").Set(lastSnapshotMetrics.DurationTotal.Seconds())
	}

	if lastDatabasePruningMetrics != nil {
		databasePruningDurations.WithLabelValues("prune_unreferenced_blocks").Set(lastDatabasePruningMetrics.DurationPruneUnreferencedBlocks.Seconds())
		databasePruningDurations.WithLabelValues("traverse_milestone_cone").Set(lastDatabasePruningMetrics.DurationTraverseMilestoneCone.Seconds())
		databasePruningDurations.WithLabelValues("prune_milestone").Set(lastDatabasePruningMetrics.DurationPruneMilestone.Seconds())
		databasePruningDurations.WithLabelValues("prune_blocks").Set(lastDatabasePruningMetrics.DurationPruneBlocks.Seconds())
		databasePruningDurations.WithLabelValues("set_snapshot_info").Set(lastDatabasePruningMetrics.DurationSetSnapshotInfo.Seconds())
		databasePruningDurations.WithLabelValues("pruning_milestone_index_changed").Set(lastDatabasePruningMetrics.DurationPruningMilestoneIndexChanged.Seconds())
		databasePruningDurations.WithLabelValues("total").Set(lastDatabasePruningMetrics.DurationTotal.Seconds())
	}

	if lastConfirmationMetrics != nil {
		milestoneConfirmationDurations.WithLabelValues("whiteflag").Set(lastConfirmationMetrics.DurationWhiteflag.Seconds())
		milestoneConfirmationDurations.WithLabelValues("receipts").Set(lastConfirmationMetrics.DurationReceipts.Seconds())
		milestoneConfirmationDurations.WithLabelValues("confirmation").Set(lastConfirmationMetrics.DurationConfirmation.Seconds())
		milestoneConfirmationDurations.WithLabelValues("apply_confirmation").Set(lastConfirmationMetrics.DurationApplyConfirmation.Seconds())
		milestoneConfirmationDurations.WithLabelValues("on_ledger_updated").Set(lastConfirmationMetrics.DurationLedgerUpdated.Seconds())
		milestoneConfirmationDurations.WithLabelValues("on_treasury_updated").Set(lastConfirmationMetrics.DurationTreasuryMutated.Seconds())
		milestoneConfirmationDurations.WithLabelValues("on_milestone_confirmed").Set(lastConfirmationMetrics.DurationOnMilestoneConfirmed.Seconds())
		milestoneConfirmationDurations.WithLabelValues("set_confirmed_milestone_index").Set(lastConfirmationMetrics.DurationSetConfirmedMilestoneIndex.Seconds())
		milestoneConfirmationDurations.WithLabelValues("update_cone_root_indexes").Set(lastConfirmationMetrics.DurationUpdateConeRootIndexes.Seconds())
		milestoneConfirmationDurations.WithLabelValues("confirmed_milestone_changed").Set(lastConfirmationMetrics.DurationConfirmedMilestoneChanged.Seconds())
		milestoneConfirmationDurations.WithLabelValues("confirmed_milestone_index_changed").Set(lastConfirmationMetrics.DurationConfirmedMilestoneIndexChanged.Seconds())
		milestoneConfirmationDurations.WithLabelValues("total").Set(lastConfirmationMetrics.DurationTotal.Seconds())
	}
}
