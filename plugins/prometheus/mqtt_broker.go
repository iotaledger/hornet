package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	mqttBrokerTopicsManagerSize prometheus.Gauge
)

func configureMQTTBroker() {

	mqttBrokerTopicsManagerSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "mqtt_broker",
			Name:      "topics_manager_size",
			Help:      "Number of active topics in the topics manager.",
		})

	registry.MustRegister(mqttBrokerTopicsManagerSize)

	addCollect(collectMQTTBroker)
}

func collectMQTTBroker() {
	mqttBrokerTopicsManagerSize.Set(float64(deps.MQTTBroker.TopicsManagerSize()))
}
