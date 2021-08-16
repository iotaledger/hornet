package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/syncutils"
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
	lastSnapshotMetrics        *snapshot.SnapshotMetrics
	lastDatabasePruningMetrics *snapshot.PruningMetrics
	lastConfirmationMetrics    *whiteflag.ConfirmationMetrics
)

func configureDebug() {

	snapshotTotalDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iota",
			Subsystem: "debug",
			Name:      "snapshot_duration_total",
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
			Name:      "pruning_duration_total",
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
			Name:      "confirmation_duration_total",
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

	deps.Snapshot.Events.SnapshotMetricsUpdated.Attach(events.NewClosure(func(metrics *snapshot.SnapshotMetrics) {
		snapshotTotalDuration.Observe(metrics.DurationTotal.Seconds())
		metricsLock.Lock()
		defer metricsLock.Unlock()
		lastSnapshotMetrics = metrics
	}))

	deps.Snapshot.Events.PruningMetricsUpdated.Attach(events.NewClosure(func(metrics *snapshot.PruningMetrics) {
		databasePruningTotalDuration.Observe(metrics.DurationTotal.Seconds())
		metricsLock.Lock()
		defer metricsLock.Unlock()
		lastDatabasePruningMetrics = metrics
	}))

	deps.Tangle.Events.ConfirmationMetricsUpdated.Attach(events.NewClosure(func(metrics *whiteflag.ConfirmationMetrics) {
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
		databasePruningDurations.WithLabelValues("prune_unreferenced_messages").Set(lastDatabasePruningMetrics.DurationPruneUnreferencedMessages.Seconds())
		databasePruningDurations.WithLabelValues("traverse_milestone_cone").Set(lastDatabasePruningMetrics.DurationTraverseMilestoneCone.Seconds())
		databasePruningDurations.WithLabelValues("prune_milestone").Set(lastDatabasePruningMetrics.DurationPruneMilestone.Seconds())
		databasePruningDurations.WithLabelValues("prune_messages").Set(lastDatabasePruningMetrics.DurationPruneMessages.Seconds())
		databasePruningDurations.WithLabelValues("set_snapshot_info").Set(lastDatabasePruningMetrics.DurationSetSnapshotInfo.Seconds())
		databasePruningDurations.WithLabelValues("pruning_milestone_index_changed").Set(lastDatabasePruningMetrics.DurationPruningMilestoneIndexChanged.Seconds())
		databasePruningDurations.WithLabelValues("total").Set(lastDatabasePruningMetrics.DurationTotal.Seconds())
	}

	if lastConfirmationMetrics != nil {
		milestoneConfirmationDurations.WithLabelValues("whiteflag").Set(lastConfirmationMetrics.DurationWhiteflag.Seconds())
		milestoneConfirmationDurations.WithLabelValues("receipts").Set(lastConfirmationMetrics.DurationReceipts.Seconds())
		milestoneConfirmationDurations.WithLabelValues("confirmation").Set(lastConfirmationMetrics.DurationConfirmation.Seconds())
		milestoneConfirmationDurations.WithLabelValues("apply_included_with_transactions").Set(lastConfirmationMetrics.DurationApplyIncludedWithTransactions.Seconds())
		milestoneConfirmationDurations.WithLabelValues("apply_excluded_without_transactions").Set(lastConfirmationMetrics.DurationApplyExcludedWithoutTransactions.Seconds())
		milestoneConfirmationDurations.WithLabelValues("apply_milestone").Set(lastConfirmationMetrics.DurationApplyMilestone.Seconds())
		milestoneConfirmationDurations.WithLabelValues("apply_excluded_with_conflicting_transactions").Set(lastConfirmationMetrics.DurationApplyExcludedWithConflictingTransactions.Seconds())
		milestoneConfirmationDurations.WithLabelValues("on_milestone_confirmed").Set(lastConfirmationMetrics.DurationOnMilestoneConfirmed.Seconds())
		milestoneConfirmationDurations.WithLabelValues("for_each_new_output").Set(lastConfirmationMetrics.DurationForEachNewOutput.Seconds())
		milestoneConfirmationDurations.WithLabelValues("for_each_new_spent").Set(lastConfirmationMetrics.DurationForEachNewSpent.Seconds())
		milestoneConfirmationDurations.WithLabelValues("set_confirmed_milestone_index").Set(lastConfirmationMetrics.DurationSetConfirmedMilestoneIndex.Seconds())
		milestoneConfirmationDurations.WithLabelValues("update_cone_root_indexes").Set(lastConfirmationMetrics.DurationUpdateConeRootIndexes.Seconds())
		milestoneConfirmationDurations.WithLabelValues("confirmed_milestone_changed").Set(lastConfirmationMetrics.DurationConfirmedMilestoneChanged.Seconds())
		milestoneConfirmationDurations.WithLabelValues("confirmed_milestone_index_changed").Set(lastConfirmationMetrics.DurationConfirmedMilestoneIndexChanged.Seconds())
		milestoneConfirmationDurations.WithLabelValues("milestone_confirmed_sync_event").Set(lastConfirmationMetrics.DurationMilestoneConfirmedSyncEvent.Seconds())
		milestoneConfirmationDurations.WithLabelValues("milestone_confirmed").Set(lastConfirmationMetrics.DurationMilestoneConfirmed.Seconds())
		milestoneConfirmationDurations.WithLabelValues("total").Set(lastConfirmationMetrics.DurationTotal.Seconds())
	}
}
