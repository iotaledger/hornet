package metrics

import (
	"github.com/iotaledger/hive.go/events"
)

type MPSMetrics struct {
	Incoming uint32 `json:"incoming"`
	New      uint32 `json:"new"`
	Outgoing uint32 `json:"outgoing"`
}

var Events = pluginEvents{
	MPSMetricsUpdated: events.NewEvent(MPSMetricsCaller),
}

type pluginEvents struct {
	MPSMetricsUpdated *events.Event
}

func MPSMetricsCaller(handler interface{}, params ...interface{}) {
	handler.(func(*MPSMetrics))(params[0].(*MPSMetrics))
}
