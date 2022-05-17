package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	cacheSizes *prometheus.GaugeVec
)

func configureCaches() {
	cacheSizes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "caches",
			Name:      "size",
			Help:      "Size of the cache.",
		},
		[]string{"type"},
	)

	registry.MustRegister(cacheSizes)

	addCollect(collectCaches)
}

func collectCaches() {
	cacheSizes.WithLabelValues("children").Set(float64(deps.Storage.ChildrenStorageSize()))
	cacheSizes.WithLabelValues("messages").Set(float64(deps.Storage.BlockStorageSize()))
	cacheSizes.WithLabelValues("messages_metadata").Set(float64(deps.Storage.BlockMetadataStorageSize()))
	cacheSizes.WithLabelValues("milestones").Set(float64(deps.Storage.MilestoneStorageSize()))
	cacheSizes.WithLabelValues("unreferenced_messages").Set(float64(deps.Storage.UnreferencedMessageStorageSize()))
	cacheSizes.WithLabelValues("message_processor_work_units").Set(float64(deps.MessageProcessor.WorkUnitsSize()))
}
