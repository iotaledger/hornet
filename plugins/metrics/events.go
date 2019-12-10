package metrics

import (
	"github.com/iotaledger/hive.go/events"
)

type TPSMetrics struct {
	Incoming uint32 `json:"incoming"`
	New      uint32 `json:"new"`
	Outgoing uint32 `json:"outgoing"`
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
