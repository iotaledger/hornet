package metrics

import (
	"github.com/iotaledger/hive.go/events"
)

type TPSMetrics struct {
	Incoming uint64 `json:"incoming"`
	New      uint64 `json:"new"`
	Outgoing uint64 `json:"outgoing"`
}

var Events = pluginEvents{
	TPSMetricsUpdated: events.NewEvent(TPSMetricsCaller),
}

type pluginEvents struct {
	TPSMetricsUpdated *events.Event
}

func TPSMetricsCaller(handler interface{}, params ...interface{}) {
	handler.(func(*TPSMetrics))(params[0].(*TPSMetrics))
}
